package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ConditionReady indicates the Thalassa WIF resources are provisioned.
	ConditionReady = "Ready"

	// Phase values for status.phase.
	PhasePending = "Pending"
	PhaseReady   = "Ready"
	PhaseError   = "Error"
)

// WorkloadIdentityBindingSpec defines the desired WIF state for a ServiceAccount
// in the same namespace as this resource.
//
// +kubebuilder:validation:XValidation:rule="(has(self.policyRef) && size(self.policyRef) > 0) || (has(self.roleRef) && size(self.roleRef) > 0)",message="at least one of policyRef or roleRef is required"
// +kubebuilder:validation:XValidation:rule="!has(self.policyRef) || self.policyRef != '*'",message="wildcard policyRef is not allowed"
// +kubebuilder:validation:XValidation:rule="!has(self.roleRef) || self.roleRef != '*'",message="wildcard roleRef is not allowed"
// +kubebuilder:validation:XValidation:rule="!has(self.scopes) || self.scopes.all(s, s != '*')",message="wildcard scope is not allowed"
type WorkloadIdentityBindingSpec struct {
	// ServiceAccountName is the Kubernetes ServiceAccount in this namespace.
	// +kubebuilder:validation:MinLength=1
	ServiceAccountName string `json:"serviceAccountName"`

	// PolicyRef is an IAM policy identity (preferred), slug, or name.
	// Prefer the stable policy identity; status.policyID always records the resolved identity.
	// +optional
	PolicyRef string `json:"policyRef,omitempty"`

	// RoleRef is an organisation role identity, slug, or name (transitional).
	// At least one of policyRef or roleRef is required.
	// +optional
	RoleRef string `json:"roleRef,omitempty"`

	// Scopes are OIDC token scopes. Empty defaults to api:read.
	// +optional
	Scopes []string `json:"scopes,omitempty"`

	// NameOverride overrides the Thalassa service account display name.
	// +optional
	NameOverride string `json:"nameOverride,omitempty"`

	// DeleteResources deletes managed Thalassa resources when this binding is deleted.
	// +optional
	DeleteResources bool `json:"deleteResources,omitempty"`
}

// WorkloadIdentityBindingStatus is the observed state.
type WorkloadIdentityBindingStatus struct {
	// Phase is Pending, Ready, or Error.
	// +optional
	Phase string `json:"phase,omitempty"`

	// ServiceAccountID is the Thalassa service account identity.
	// +optional
	ServiceAccountID string `json:"serviceAccountID,omitempty"`

	// FederatedIdentityID is the Thalassa federated identity identity.
	// +optional
	FederatedIdentityID string `json:"federatedIdentityID,omitempty"`

	// ProviderID is the cluster OIDC identity provider identity.
	// +optional
	ProviderID string `json:"providerID,omitempty"`

	// PolicyID is the resolved Thalassa IAM policy identity (never slug/name).
	// Prefer this value over spec.policyRef for lookups and cleanup.
	// +optional
	PolicyID string `json:"policyID,omitempty"`

	// ObservedGeneration is the generation last processed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// LastReconcileTime is when the controller last finished a reconcile.
	// +optional
	LastReconcileTime *metav1.Time `json:"lastReconcileTime,omitempty"`

	// Conditions represent the latest available observations.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=wib
// +kubebuilder:printcolumn:name="SA",type=string,JSONPath=`.spec.serviceAccountName`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Policy",type=string,JSONPath=`.status.policyID`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="Last Transition",type=date,JSONPath=`.status.conditions[?(@.type=="Ready")].lastTransitionTime`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// WorkloadIdentityBinding binds a namespaced ServiceAccount to Thalassa WIF.
type WorkloadIdentityBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkloadIdentityBindingSpec   `json:"spec,omitempty"`
	Status WorkloadIdentityBindingStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// WorkloadIdentityBindingList contains a list of WorkloadIdentityBinding.
type WorkloadIdentityBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WorkloadIdentityBinding `json:"items"`
}

func init() {
	SchemeBuilder.Register(&WorkloadIdentityBinding{}, &WorkloadIdentityBindingList{})
}
