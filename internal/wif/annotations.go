package wif

// Annotation keys on Kubernetes ServiceAccounts.
const (
	AnnotationEnabled         = "thalassa.cloud/wif.enabled"
	AnnotationRole            = "thalassa.cloud/wif.role"
	AnnotationPolicy          = "thalassa.cloud/wif.policy"
	AnnotationScopes          = "thalassa.cloud/wif.scopes"
	AnnotationName            = "thalassa.cloud/wif.name"
	AnnotationDeleteResources = "thalassa.cloud/wif.delete-resources"

	AnnotationStatus              = "thalassa.cloud/wif.status"
	AnnotationServiceAccountID    = "thalassa.cloud/wif.service-account-id"
	AnnotationFederatedIdentityID = "thalassa.cloud/wif.federated-identity-id"
	AnnotationProviderID          = "thalassa.cloud/wif.provider-id"
	AnnotationPolicyID            = "thalassa.cloud/wif.policy-id"
	AnnotationOrganisationID      = "thalassa.cloud/wif.organisation-id"
	AnnotationProjectID           = "thalassa.cloud/wif.project-id"
	AnnotationLastError           = "thalassa.cloud/wif.last-error"
	AnnotationLastReconcile       = "thalassa.cloud/wif.last-reconcile"
	AnnotationProviderSubject     = "thalassa.cloud/wif.provider-subject"
	AnnotationRepository          = "thalassa.cloud/wif.repository"
)

const (
	ConfigMapKeyOrganisationID   = "organisation-id"
	ConfigMapKeyServiceAccountID = "service-account-id"
	ConfigMapKeyProjectID        = "project-id"
)

// FinalizerName is added to opted-in ServiceAccounts (annotation path) and bindings.
const FinalizerName = "thalassa.cloud/wif-finalizer"

// BindingFinalizerName is added to WorkloadIdentityBinding resources.
const BindingFinalizerName = FinalizerName

// Status values written to AnnotationStatus.
const (
	StatusPending = "Pending"
	StatusReady   = "Ready"
	StatusError   = "Error"
)

// Ownership labels on Thalassa IAM resources.
const (
	LabelManagedBy           = "thalassa.cloud/managed-by"
	ValueManagedBy           = "workload-identity-controller"
	LabelWIFKey              = "thalassa.cloud/wif-key"
	LabelWIFVCS              = "thalassa.cloud/wif-vcs"
	ValueVCSKubernetes       = "kubernetes"
	LabelKubernetesClusterID = "kubernetes_cluster_id"
	LabelK8sNamespace        = "thalassa.cloud/k8s-namespace"
	LabelK8sServiceAccount   = "thalassa.cloud/k8s-service-account"
)
