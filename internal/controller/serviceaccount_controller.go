package controller

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/thalassa-cloud/thalassa-workload-identity-controller/internal/config"
	"github.com/thalassa-cloud/thalassa-workload-identity-controller/internal/wif"
)

// ServiceAccountReconciler reconciles annotated ServiceAccounts into Thalassa WIF.
type ServiceAccountReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	IAM    wif.IAMClient
	Config config.Config
	Now    func() time.Time
}

// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=serviceaccounts/finalizers,verbs=update

func (r *ServiceAccountReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var sa corev1.ServiceAccount
	if err := r.Get(ctx, req.NamespacedName, &sa); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if len(r.Config.WatchNamespaces) > 0 && !contains(r.Config.WatchNamespaces, sa.Namespace) {
		return ctrl.Result{}, nil
	}

	enabled := wif.IsEnabled(sa.Annotations)
	hasFinalizer := controllerutil.ContainsFinalizer(&sa, wif.FinalizerName)

	// Deletion
	if !sa.DeletionTimestamp.IsZero() {
		if !hasFinalizer {
			return ctrl.Result{}, nil
		}
		if wif.DeleteResources(sa.Annotations) {
			if err := r.deleteCloudResources(ctx, &sa); err != nil {
				logger.Error(err, "failed to delete Thalassa WIF resources")
				_ = r.patchStatus(ctx, &sa, wif.StatusError, err.Error(), nil)
				return ctrl.Result{}, err
			}
		}
		controllerutil.RemoveFinalizer(&sa, wif.FinalizerName)
		if err := r.Update(ctx, &sa); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if !enabled {
		if hasFinalizer {
			if wif.DeleteResources(sa.Annotations) {
				if err := r.deleteCloudResources(ctx, &sa); err != nil {
					_ = r.patchStatus(ctx, &sa, wif.StatusError, err.Error(), nil)
					return ctrl.Result{}, err
				}
			}
			controllerutil.RemoveFinalizer(&sa, wif.FinalizerName)
			if err := r.Update(ctx, &sa); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Opted in: ensure finalizer
	if !hasFinalizer {
		controllerutil.AddFinalizer(&sa, wif.FinalizerName)
		if err := r.Update(ctx, &sa); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	role := strings.TrimSpace(sa.Annotations[wif.AnnotationRole])
	policy := strings.TrimSpace(sa.Annotations[wif.AnnotationPolicy])
	if role == "" && policy == "" {
		errMsg := "at least one of thalassa.cloud/wif.policy or thalassa.cloud/wif.role is required"
		_ = r.patchStatus(ctx, &sa, wif.StatusError, errMsg, nil)
		return ctrl.Result{}, nil
	}
	if role == "*" || policy == "*" {
		errMsg := "wildcard policy/role is not allowed"
		_ = r.patchStatus(ctx, &sa, wif.StatusError, errMsg, nil)
		return ctrl.Result{}, nil
	}

	scopes, err := wif.ParseScopes(sa.Annotations[wif.AnnotationScopes])
	if err != nil {
		_ = r.patchStatus(ctx, &sa, wif.StatusError, err.Error(), nil)
		return ctrl.Result{}, nil
	}

	_ = r.patchStatus(ctx, &sa, wif.StatusPending, "", nil)

	result, err := wif.EnsureWorkloadIdentity(ctx, r.IAM, wif.EnsureInput{
		Namespace:        sa.Namespace,
		ServiceAccount:   sa.Name,
		RoleRef:          role,
		PolicyRef:        policy,
		NameOverride:     sa.Annotations[wif.AnnotationName],
		Scopes:           scopes,
		TrustedAudiences: r.Config.TrustedAudiences,
		ClusterIdentity:  r.Config.ClusterIdentity,
		AllowedPolicies:  r.Config.AllowedPolicies,
		AllowedRoles:     r.Config.AllowedRoles,
	})
	if err != nil {
		var notReady *wif.ErrProviderNotReady
		if errors.As(err, &notReady) {
			logger.Info("waiting for cluster OIDC identity provider", "cluster", notReady.ClusterIdentity)
			_ = r.patchStatus(ctx, &sa, wif.StatusPending, err.Error(), nil)
			return ctrl.Result{RequeueAfter: r.Config.RequeueMissingIDP}, nil
		}
		logger.Error(err, "ensure workload identity failed")
		_ = r.patchStatus(ctx, &sa, wif.StatusError, err.Error(), nil)
		return ctrl.Result{}, err
	}

	if err := r.patchStatus(ctx, &sa, wif.StatusReady, "", result); err != nil {
		return ctrl.Result{}, err
	}
	logger.Info("workload identity ready",
		"thalassaServiceAccount", result.ServiceAccountID,
		"federatedIdentity", result.FederatedIdentityID,
		"policy", result.PolicyID,
	)
	return ctrl.Result{}, nil
}

func (r *ServiceAccountReconciler) deleteCloudResources(ctx context.Context, sa *corev1.ServiceAccount) error {
	provider, err := wif.FindProviderByClusterID(ctx, r.IAM, r.Config.ClusterIdentity)
	if err != nil {
		return fmt.Errorf("resolve cluster identity provider for delete: %w", err)
	}
	if provider == nil {
		return fmt.Errorf("resolve cluster identity provider for delete: provider is nil")
	}
	issuer := wif.NormalizeIssuer(provider.ProviderIssuer)
	if issuer == "" {
		return fmt.Errorf("cluster identity provider %s has empty issuer", provider.Identity)
	}
	role, policy := "", ""
	if sa.Annotations != nil {
		role = sa.Annotations[wif.AnnotationRole]
		// Prefer resolved policy identity from status annotation.
		policy = sa.Annotations[wif.AnnotationPolicyID]
		if policy == "" {
			policy = sa.Annotations[wif.AnnotationPolicy]
		}
	}
	return wif.DeleteWorkloadIdentity(ctx, r.IAM, sa.Namespace, sa.Name, issuer, role, policy)
}

func (r *ServiceAccountReconciler) patchStatus(ctx context.Context, sa *corev1.ServiceAccount, status, lastErr string, result *wif.EnsureResult) error {
	var latest corev1.ServiceAccount
	if err := r.Get(ctx, client.ObjectKeyFromObject(sa), &latest); err != nil {
		return err
	}
	if latest.Annotations == nil {
		latest.Annotations = map[string]string{}
	}
	now := r.now().UTC().Format(time.RFC3339)
	latest.Annotations[wif.AnnotationStatus] = status
	latest.Annotations[wif.AnnotationLastReconcile] = now
	if lastErr == "" {
		delete(latest.Annotations, wif.AnnotationLastError)
	} else {
		// Cap error length to keep annotation size reasonable.
		if len(lastErr) > 1024 {
			lastErr = lastErr[:1024]
		}
		latest.Annotations[wif.AnnotationLastError] = lastErr
	}
	if result != nil {
		latest.Annotations[wif.AnnotationServiceAccountID] = result.ServiceAccountID
		latest.Annotations[wif.AnnotationFederatedIdentityID] = result.FederatedIdentityID
		latest.Annotations[wif.AnnotationProviderID] = result.ProviderID
		if result.PolicyID != "" {
			latest.Annotations[wif.AnnotationPolicyID] = result.PolicyID
		} else {
			delete(latest.Annotations, wif.AnnotationPolicyID)
		}
	}
	return r.Update(ctx, &latest)
}

func (r *ServiceAccountReconciler) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

// SetupWithManager registers the reconciler.
func (r *ServiceAccountReconciler) SetupWithManager(mgr ctrl.Manager) error {
	pred := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		sa, ok := obj.(*corev1.ServiceAccount)
		if !ok {
			return false
		}
		if len(r.Config.WatchNamespaces) > 0 && !contains(r.Config.WatchNamespaces, sa.Namespace) {
			return false
		}
		// Reconcile if enabled, or if we previously attached a finalizer / status.
		if wif.IsEnabled(sa.Annotations) {
			return true
		}
		if controllerutil.ContainsFinalizer(sa, wif.FinalizerName) {
			return true
		}
		if sa.Annotations != nil && sa.Annotations[wif.AnnotationStatus] != "" {
			return true
		}
		return false
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ServiceAccount{}).
		WithEventFilter(pred).
		Complete(r)
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
