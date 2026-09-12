package controller

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	Namespace       string
	ServiceAccount  string
	PolicyRef       string
	PolicyID        string // resolved identity from status; preferred on delete
	RoleRef         string
	Scopes          []string
	NameOverride    string
	DeleteResources bool
}

// bindingObserved is written back to CR status.
type bindingObserved struct {
	Phase               string
	ServiceAccountID    string
	FederatedIdentityID string
	ProviderID          string
	PolicyID            string
	Ready               bool
	Message             string
	Reason              string
}

type bindingReconciler struct {
	Client client.Client
	IAM    wif.IAMClient
	Config config.Config
	Now    func() time.Time
}

func (r *bindingReconciler) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
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

	if err := validateDesired(desired, r.Config); err != nil {
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
	}, nil
}

func validateDesired(d bindingDesired, cfg config.Config) error {
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
