package controller

import (
	"context"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	iamv1beta1 "github.com/thalassa-cloud/thalassa-workload-identity-controller/api/v1beta1"
	"github.com/thalassa-cloud/thalassa-workload-identity-controller/internal/config"
	"github.com/thalassa-cloud/thalassa-workload-identity-controller/internal/wif"
)

// WorkloadIdentityBindingReconciler reconciles namespaced WorkloadIdentityBinding CRs.
type WorkloadIdentityBindingReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	IAM    wif.IAMClient
	Config config.Config
	Now    func() time.Time
}

// +kubebuilder:rbac:groups=iam.thalassa.cloud,resources=workloadidentitybindings,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=iam.thalassa.cloud,resources=workloadidentitybindings/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=iam.thalassa.cloud,resources=workloadidentitybindings/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch

func (r *WorkloadIdentityBindingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var binding iamv1beta1.WorkloadIdentityBinding
	if err := r.Get(ctx, req.NamespacedName, &binding); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if len(r.Config.WatchNamespaces) > 0 && !contains(r.Config.WatchNamespaces, binding.Namespace) {
		return ctrl.Result{}, nil
	}

	br := &bindingReconciler{Client: r.Client, IAM: r.IAM, Config: r.Config, Now: r.Now}
	desired := bindingDesired{
		Namespace:       binding.Namespace,
		ServiceAccount:  binding.Spec.ServiceAccountName,
		PolicyRef:       binding.Spec.PolicyRef,
		PolicyID:        binding.Status.PolicyID,
		RoleRef:         binding.Spec.RoleRef,
		Scopes:          binding.Spec.Scopes,
		NameOverride:    binding.Spec.NameOverride,
		DeleteResources: binding.Spec.DeleteResources,
	}
	hasFinalizer := controllerutil.ContainsFinalizer(&binding, wif.BindingFinalizerName)
	result, obs, err := br.reconcileBinding(ctx, &binding, desired, hasFinalizer)
	if obs != nil {
		if statusErr := r.patchStatus(ctx, &binding, obs); statusErr != nil && err == nil {
			return ctrl.Result{}, statusErr
		}
	}
	return result, err
}

func (r *WorkloadIdentityBindingReconciler) patchStatus(ctx context.Context, binding *iamv1beta1.WorkloadIdentityBinding, obs *bindingObserved) error {
	var latest iamv1beta1.WorkloadIdentityBinding
	if err := r.Get(ctx, client.ObjectKeyFromObject(binding), &latest); err != nil {
		return err
	}
	now := metav1.NewTime(r.now().UTC())
	latest.Status.Phase = obs.Phase
	latest.Status.ObservedGeneration = latest.Generation
	latest.Status.LastReconcileTime = &now
	if obs.ServiceAccountID != "" {
		latest.Status.ServiceAccountID = obs.ServiceAccountID
		latest.Status.FederatedIdentityID = obs.FederatedIdentityID
		latest.Status.ProviderID = obs.ProviderID
		// Always record the resolved policy identity (empty when role-only).
		latest.Status.PolicyID = obs.PolicyID
	}
	latest.Status.Conditions = setReadyCondition(latest.Status.Conditions, obs, latest.Generation, now)
	return r.Status().Update(ctx, &latest)
}

func (r *WorkloadIdentityBindingReconciler) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

// SetupWithManager registers the reconciler.
func (r *WorkloadIdentityBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	pred := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		b, ok := obj.(*iamv1beta1.WorkloadIdentityBinding)
		if !ok {
			return false
		}
		if len(r.Config.WatchNamespaces) > 0 && !contains(r.Config.WatchNamespaces, b.Namespace) {
			return false
		}
		return true
	})
	return ctrl.NewControllerManagedBy(mgr).
		For(&iamv1beta1.WorkloadIdentityBinding{}).
		WithEventFilter(pred).
		Complete(r)
}
