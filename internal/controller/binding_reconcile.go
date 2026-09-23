package controller

import (
	"context"
	"errors"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/thalassa-cloud/thalassa-workload-identity-controller/internal/config"
	"github.com/thalassa-cloud/thalassa-workload-identity-controller/internal/wif"
)

// bindingDesired is the normalised desired state from a namespaced CR.
type bindingDesired struct {
	Namespace         string
	ServiceAccount    string
	PolicyRef         string
	PolicyID          string // resolved identity from status; preferred on delete
	RoleRef           string
	Scopes            []string
	NameOverride      string
	DeleteResources   bool
	IdentityConfigMap bool
}

// bindingObserved is written back to CR status.
type bindingObserved struct {
	Phase               string
	ServiceAccountID    string
	FederatedIdentityID string
	ProviderID          string
	PolicyID            string
	OrganisationID      string
	ProjectID           string
	Ready               bool
	Message             string
	Reason              string
}

type bindingReconciler struct {
	Client client.Client
	Scheme *runtime.Scheme
	IAM    wif.IAMClient
	Config config.Config
}

func (r *bindingReconciler) reconcileBinding(
	ctx context.Context,
	obj client.Object,
	desired bindingDesired,
	hasFinalizer bool,
) (ctrl.Result, *bindingObserved, error) {
	logger := log.FromContext(ctx)

	if !obj.GetDeletionTimestamp().IsZero() {
		if !hasFinalizer {
			return ctrl.Result{}, nil, nil
		}
		if err := r.clearServiceAccountIdentityAnnotations(ctx, desired.Namespace, desired.ServiceAccount); err != nil {
			logger.Error(err, "failed to clear ServiceAccount WIF identity annotations")
			obs := &bindingObserved{
				Phase:   wif.StatusError,
				Ready:   false,
				Reason:  "CleanupFailed",
				Message: err.Error(),
			}
			return ctrl.Result{}, obs, err
		}
		if desired.DeleteResources {
			if err := r.deleteCloudResources(ctx, desired); err != nil {
				logger.Error(err, "failed to delete Thalassa WIF resources")
				obs := &bindingObserved{
					Phase:   wif.StatusError,
					Ready:   false,
					Reason:  "DeleteFailed",
					Message: err.Error(),
				}
				return ctrl.Result{}, obs, err
			}
		}
		controllerutil.RemoveFinalizer(obj, wif.BindingFinalizerName)
		if err := r.Client.Update(ctx, obj); err != nil {
			return ctrl.Result{}, nil, err
		}
		return ctrl.Result{}, nil, nil
	}

	if !hasFinalizer {
		controllerutil.AddFinalizer(obj, wif.BindingFinalizerName)
		if err := r.Client.Update(ctx, obj); err != nil {
			return ctrl.Result{}, nil, err
		}
		return ctrl.Result{Requeue: true}, &bindingObserved{Phase: wif.StatusPending, Reason: "FinalizerAdded"}, nil
	}

	if err := validateDesired(desired); err != nil {
		return ctrl.Result{}, &bindingObserved{
			Phase:   wif.StatusError,
			Ready:   false,
			Reason:  "InvalidSpec",
			Message: err.Error(),
		}, nil
	}

	if len(r.Config.WatchNamespaces) > 0 && !contains(r.Config.WatchNamespaces, desired.Namespace) {
		msg := fmt.Sprintf("service account namespace %q is outside controller watch namespaces", desired.Namespace)
		return ctrl.Result{}, &bindingObserved{
			Phase:   wif.StatusError,
			Ready:   false,
			Reason:  "NamespaceNotWatched",
			Message: msg,
		}, nil
	}

	var sa corev1.ServiceAccount
	if err := r.Client.Get(ctx, types.NamespacedName{Namespace: desired.Namespace, Name: desired.ServiceAccount}, &sa); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, &bindingObserved{
				Phase:   wif.StatusError,
				Ready:   false,
				Reason:  "ServiceAccountNotFound",
				Message: err.Error(),
			}, nil
		}
		return ctrl.Result{}, nil, err
	}

	scopes, err := wif.ParseScopeList(desired.Scopes)
	if err != nil {
		return ctrl.Result{}, &bindingObserved{
			Phase:   wif.StatusError,
			Ready:   false,
			Reason:  "InvalidScopes",
			Message: err.Error(),
		}, nil
	}

	result, err := wif.EnsureWorkloadIdentity(ctx, r.IAM, wif.EnsureInput{
		Namespace:        desired.Namespace,
		ServiceAccount:   desired.ServiceAccount,
		RoleRef:          desired.RoleRef,
		PolicyRef:        desired.PolicyRef,
		NameOverride:     desired.NameOverride,
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
			return ctrl.Result{RequeueAfter: r.Config.RequeueMissingIDP}, &bindingObserved{
				Phase:   wif.StatusPending,
				Ready:   false,
				Reason:  "ProviderNotReady",
				Message: err.Error(),
			}, nil
		}
		logger.Error(err, "ensure workload identity failed")
		return ctrl.Result{}, &bindingObserved{
			Phase:   wif.StatusError,
			Ready:   false,
			Reason:  "EnsureFailed",
			Message: err.Error(),
		}, err
	}

	orgID := strings.TrimSpace(r.Config.OrganisationID)
	projectID := strings.TrimSpace(r.Config.ProjectIdentity)
	if desired.IdentityConfigMap || r.Config.EnableIdentityConfigMap {
		if err := r.syncIdentityConfigMap(ctx, obj, desired, result.ServiceAccountID, orgID, projectID); err != nil {
			logger.Error(err, "failed to sync WIF identity ConfigMap")
			return ctrl.Result{}, &bindingObserved{
				Phase:   wif.StatusError,
				Ready:   false,
				Reason:  "IdentityConfigMapFailed",
				Message: err.Error(),
			}, err
		}
	}
	if err := r.annotateServiceAccountIdentity(ctx, &sa, result.ServiceAccountID, orgID, projectID); err != nil {
		logger.Error(err, "failed to annotate ServiceAccount with WIF identity")
		return ctrl.Result{}, &bindingObserved{
			Phase:   wif.StatusError,
			Ready:   false,
			Reason:  "ServiceAccountAnnotateFailed",
			Message: err.Error(),
		}, err
	}

	logger.Info("workload identity ready",
		"thalassaServiceAccount", result.ServiceAccountID,
		"federatedIdentity", result.FederatedIdentityID,
		"policy", result.PolicyID,
	)
	return ctrl.Result{}, &bindingObserved{
		Phase:               wif.StatusReady,
		Ready:               true,
		Reason:              "Ready",
		Message:             "Thalassa workload identity resources are ready",
		ServiceAccountID:    result.ServiceAccountID,
		FederatedIdentityID: result.FederatedIdentityID,
		ProviderID:          result.ProviderID,
		PolicyID:            result.PolicyID,
		OrganisationID:      orgID,
		ProjectID:           projectID,
	}, nil
}

func (r *bindingReconciler) syncIdentityConfigMap(
	ctx context.Context,
	owner client.Object,
	desired bindingDesired,
	serviceAccountID, organisationID, projectID string,
) error {
	data := IdentityConfigMapData(organisationID, serviceAccountID, projectID)
	if err := ValidateIdentityConfigMapData(data); err != nil {
		return err
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      IdentityConfigMapName(desired.ServiceAccount),
			Namespace: desired.Namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, cm, func() error {
		if r.Scheme != nil {
			if err := controllerutil.SetControllerReference(owner, cm, r.Scheme); err != nil {
				return err
			}
		}
		if cm.Labels == nil {
			cm.Labels = map[string]string{}
		}
		cm.Labels[wif.LabelManagedBy] = wif.ValueManagedBy
		cm.Labels[wif.LabelK8sNamespace] = desired.Namespace
		cm.Labels[wif.LabelK8sServiceAccount] = desired.ServiceAccount
		cm.Data = data
		return nil
	})
	return err
}

func (r *bindingReconciler) annotateServiceAccountIdentity(
	ctx context.Context,
	sa *corev1.ServiceAccount,
	serviceAccountID, organisationID, projectID string,
) error {
	var latest corev1.ServiceAccount
	if err := r.Client.Get(ctx, types.NamespacedName{Namespace: sa.Namespace, Name: sa.Name}, &latest); err != nil {
		return err
	}
	if latest.Annotations == nil {
		latest.Annotations = map[string]string{}
	}
	latest.Annotations[wif.AnnotationServiceAccountID] = serviceAccountID
	latest.Annotations[wif.AnnotationOrganisationID] = organisationID
	if projectID != "" {
		latest.Annotations[wif.AnnotationProjectID] = projectID
	} else {
		delete(latest.Annotations, wif.AnnotationProjectID)
	}
	return r.Client.Update(ctx, &latest)
}

func (r *bindingReconciler) clearServiceAccountIdentityAnnotations(ctx context.Context, namespace, name string) error {
	var sa corev1.ServiceAccount
	if err := r.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &sa); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if sa.Annotations == nil {
		return nil
	}
	keys := []string{
		wif.AnnotationServiceAccountID,
		wif.AnnotationOrganisationID,
		wif.AnnotationProjectID,
		wif.AnnotationFederatedIdentityID,
		wif.AnnotationProviderID,
		wif.AnnotationPolicyID,
	}
	changed := false
	for _, k := range keys {
		if _, ok := sa.Annotations[k]; ok {
			delete(sa.Annotations, k)
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return r.Client.Update(ctx, &sa)
}

func validateDesired(d bindingDesired) error {
	if strings.TrimSpace(d.Namespace) == "" || strings.TrimSpace(d.ServiceAccount) == "" {
		return fmt.Errorf("service account namespace and name are required")
	}
	role := strings.TrimSpace(d.RoleRef)
	policy := strings.TrimSpace(d.PolicyRef)
	if role == "" && policy == "" {
		return fmt.Errorf("at least one of policyRef or roleRef is required")
	}
	if role == "*" || policy == "*" {
		return fmt.Errorf("wildcard policy/role is not allowed")
	}
	return nil
}

func (r *bindingReconciler) deleteCloudResources(ctx context.Context, desired bindingDesired) error {
	logger := log.FromContext(ctx)
	provider, err := wif.FindProviderByClusterID(ctx, r.IAM, r.Config.ClusterIdentity)
	if err != nil {
		var notReady *wif.ErrProviderNotReady
		if errors.As(err, &notReady) {
			// Unblock finalizer removal when the cluster IdP is gone (resources may
			// already be unreachable or were cleaned up out-of-band).
			logger.Info("skipping Thalassa resource delete; cluster IdP not found",
				"cluster", r.Config.ClusterIdentity)
			return nil
		}
		return fmt.Errorf("resolve cluster identity provider for delete: %w", err)
	}
	if provider == nil {
		logger.Info("skipping Thalassa resource delete; cluster IdP is nil",
			"cluster", r.Config.ClusterIdentity)
		return nil
	}
	issuer := wif.NormalizeIssuer(provider.ProviderIssuer)
	if issuer == "" {
		logger.Info("skipping Thalassa resource delete; cluster IdP has empty issuer",
			"provider", provider.Identity)
		return nil
	}
	// Prefer resolved policy identity when available.
	policyRef := strings.TrimSpace(desired.PolicyID)
	if policyRef == "" {
		policyRef = desired.PolicyRef
	}
	return wif.DeleteWorkloadIdentity(ctx, r.IAM, desired.Namespace, desired.ServiceAccount, issuer, desired.RoleRef, policyRef)
}

func setReadyCondition(conditions []metav1.Condition, obs *bindingObserved, generation int64, now metav1.Time) []metav1.Condition {
	if obs == nil {
		return conditions
	}
	status := metav1.ConditionFalse
	if obs.Ready {
		status = metav1.ConditionTrue
	}
	msg := obs.Message
	if len(msg) > 1024 {
		msg = msg[:1024]
	}
	cond := metav1.Condition{
		Type:               "Ready",
		Status:             status,
		Reason:             obs.Reason,
		Message:            msg,
		ObservedGeneration: generation,
		LastTransitionTime: now,
	}
	return setCondition(conditions, cond)
}

func setCondition(conditions []metav1.Condition, neu metav1.Condition) []metav1.Condition {
	for i := range conditions {
		if conditions[i].Type != neu.Type {
			continue
		}
		if conditions[i].Status == neu.Status &&
			conditions[i].Reason == neu.Reason &&
			conditions[i].Message == neu.Message &&
			conditions[i].ObservedGeneration == neu.ObservedGeneration {
			return conditions
		}
		if conditions[i].Status == neu.Status {
			neu.LastTransitionTime = conditions[i].LastTransitionTime
		}
		conditions[i] = neu
		return conditions
	}
	return append(conditions, neu)
}
