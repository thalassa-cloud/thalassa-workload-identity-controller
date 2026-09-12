package wif

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/thalassa-cloud/client-go/filters"
	"github.com/thalassa-cloud/client-go/iam"
	"github.com/thalassa-cloud/thalassa-workload-identity-controller/internal/config"
)

// Ensure *iam.Client satisfies IAMClient at compile time.
var _ IAMClient = (*iam.Client)(nil)

// IAMClient is the subset of client-go IAM used by the reconciler.
type IAMClient interface {
	ListFederatedIdentityProviders(ctx context.Context, request *iam.ListFederatedIdentityProvidersRequest) ([]iam.FederatedIdentityProvider, error)
	ListServiceAccounts(ctx context.Context, request *iam.ListServiceAccountsRequest) ([]iam.ServiceAccount, error)
	CreateServiceAccount(ctx context.Context, create iam.CreateServiceAccountRequest) (*iam.ServiceAccount, error)
	DeleteServiceAccount(ctx context.Context, identity string) error
	ListFederatedIdentities(ctx context.Context, request *iam.ListFederatedIdentitiesRequest) ([]iam.FederatedIdentity, error)
	CreateFederatedIdentity(ctx context.Context, create iam.CreateFederatedIdentityRequest) (*iam.FederatedIdentity, error)
	UpdateFederatedIdentity(ctx context.Context, identity string, update iam.UpdateFederatedIdentityRequest) (*iam.FederatedIdentity, error)
	DeleteFederatedIdentity(ctx context.Context, identity string) error
	GetOrganisationRole(ctx context.Context, identity string) (*iam.OrganisationRole, error)
	ListOrganisationRoles(ctx context.Context, request *iam.ListOrganisationRolesRequest) ([]iam.OrganisationRole, error)
	ListRoleBindings(ctx context.Context, roleIdentity string, request *iam.ListRoleBindingsRequest) ([]iam.OrganisationRoleBinding, error)
	CreateRoleBinding(ctx context.Context, roleIdentity string, create iam.CreateRoleBinding) (*iam.OrganisationRoleBinding, error)
	DeleteRoleBinding(ctx context.Context, roleIdentity, bindingIdentity string) error
	ListIamPolicies(ctx context.Context, request *iam.ListIamPoliciesRequest) ([]iam.IamPolicy, error)
	GetIamPolicy(ctx context.Context, identity string) (*iam.IamPolicy, error)
	ListIamPolicyBindings(ctx context.Context, policyIdentity string, request *iam.ListIamPolicyBindingsRequest) ([]iam.IamPolicyBinding, error)
	CreateIamPolicyBinding(ctx context.Context, policyIdentity string, create iam.CreateIamPolicyBindingRequest) (*iam.IamPolicyBinding, error)
	DeleteIamPolicyBinding(ctx context.Context, policyIdentity, bindingIdentity string) error
}

// EnsureInput is the desired WIF state for one Kubernetes ServiceAccount.
type EnsureInput struct {
	Namespace        string
	ServiceAccount   string
	RoleRef          string // optional organisation role
	PolicyRef        string // optional IAM policy (preferred)
	NameOverride     string
	Scopes           []iam.AccessCredentialsScope
	TrustedAudiences []string
	ClusterIdentity  string
	AllowedPolicies  []string // empty = allow any
	AllowedRoles     []string // empty = allow any
}

// EnsureResult holds identities of ensured resources.
type EnsureResult struct {
	ProviderID          string
	ServiceAccountID    string
	FederatedIdentityID string
	PolicyID            string
	Issuer              string
	Subject             string
	Key                 string
}

// ErrProviderNotReady means the cluster OIDC IdP is not provisioned yet.
type ErrProviderNotReady struct {
	ClusterIdentity string
}

func (e *ErrProviderNotReady) Error() string {
	return fmt.Sprintf("no federated identity provider for kubernetes cluster %s (label %s)", e.ClusterIdentity, LabelKubernetesClusterID)
}

// FindProviderByClusterID finds exactly one IdP labelled with the cluster identity.
func FindProviderByClusterID(ctx context.Context, c IAMClient, clusterIdentity string) (*iam.FederatedIdentityProvider, error) {
	clusterIdentity = strings.TrimSpace(clusterIdentity)
	if clusterIdentity == "" {
		return nil, fmt.Errorf("cluster identity is required")
	}
	want := map[string]string{LabelKubernetesClusterID: clusterIdentity}
	list, err := c.ListFederatedIdentityProviders(ctx, &iam.ListFederatedIdentityProvidersRequest{
		Filters: []filters.Filter{&filters.LabelFilter{MatchLabels: want}},
	})
	if err != nil {
		return nil, err
	}
	var found *iam.FederatedIdentityProvider
	n := 0
	for i := range list {
		p := &list[i]
		if LabelsMatch(p.Labels, want) {
			n++
			found = p
		}
	}
	if n == 0 {
		return nil, &ErrProviderNotReady{ClusterIdentity: clusterIdentity}
	}
	if n > 1 {
		return nil, fmt.Errorf("multiple federated identity providers match label %s=%s", LabelKubernetesClusterID, clusterIdentity)
	}
	return found, nil
}

// EnsureWorkloadIdentity creates or updates Thalassa SA + FI + role binding.
func EnsureWorkloadIdentity(ctx context.Context, c IAMClient, in EnsureInput) (*EnsureResult, error) {
	if err := validateEnsureInput(in); err != nil {
		return nil, err
	}

	provider, err := FindProviderByClusterID(ctx, c, in.ClusterIdentity)
	if err != nil {
		return nil, err
	}
	issuer := NormalizeIssuer(provider.ProviderIssuer)
	if issuer == "" {
		return nil, fmt.Errorf("federated identity provider %s has an empty issuer", provider.Identity)
	}

	subject, err := BuildKubernetesSubject(in.Namespace, in.ServiceAccount)
	if err != nil {
		return nil, err
	}
	key := ResourceKey(in.Namespace, in.ServiceAccount, subject, issuer)
	saName, fiName := DefaultResourceNames(key, in.NameOverride)
	labels := OwnershipLabels(key, in.Namespace, in.ServiceAccount)

	sa, err := ensureServiceAccount(ctx, c, saName, subject, in.Namespace, in.ServiceAccount, labels)
	if err != nil {
		return nil, err
	}

	fi, err := ensureFederatedIdentity(ctx, c, provider, sa, fiName, subject, in.Scopes, in.TrustedAudiences, labels)
	if err != nil {
		return nil, err
	}

	result := &EnsureResult{
		ProviderID:          provider.Identity,
		ServiceAccountID:    sa.Identity,
		FederatedIdentityID: fi.Identity,
		Issuer:              issuer,
		Subject:             subject,
		Key:                 key,
	}

	if policyRef := strings.TrimSpace(in.PolicyRef); policyRef != "" {
		policy, err := resolveIamPolicy(ctx, c, policyRef)
		if err != nil {
			return nil, err
		}
		if err := config.CheckRefAllowlist("policy", policyRef, in.AllowedPolicies, policy.Identity, policy.Slug, policy.Name); err != nil {
			return nil, err
		}
		if err := ensurePolicyBinding(ctx, c, policy, sa, key, labels); err != nil {
			return nil, err
		}
		// Always persist the stable identity (never slug/name) in the result.
		result.PolicyID = policy.Identity
	}

	if roleRef := strings.TrimSpace(in.RoleRef); roleRef != "" {
		role, err := resolveOrganisationRole(ctx, c, roleRef)
		if err != nil {
			return nil, err
		}
		if err := config.CheckRefAllowlist("role", roleRef, in.AllowedRoles, role.Identity, role.Slug, role.Name); err != nil {
			return nil, err
		}
		if err := ensureRoleBinding(ctx, c, role, sa, key, labels); err != nil {
			return nil, err
		}
	}

	return result, nil
}

// DeleteWorkloadIdentity removes controller-owned FI, bindings, and SA when present.
func DeleteWorkloadIdentity(ctx context.Context, c IAMClient, namespace, saName, issuer, roleRef, policyRef string) error {
	subject, err := BuildKubernetesSubject(namespace, saName)
	if err != nil {
		return err
	}
	key := ResourceKey(namespace, saName, subject, issuer)
	want := OwnershipLabels(key, namespace, saName)

	fi, err := findByLabelsFI(ctx, c, want)
	if err != nil {
		return err
	}
	if fi != nil {
		if err := c.DeleteFederatedIdentity(ctx, fi.Identity); err != nil {
			return fmt.Errorf("delete federated identity: %w", err)
		}
	}

	sa, err := findByLabelsSA(ctx, c, want)
	if err != nil {
		return err
	}
	if sa != nil && strings.TrimSpace(policyRef) != "" {
		policy, err := resolveIamPolicy(ctx, c, policyRef)
		if err == nil && policy != nil {
			if err := deletePolicyBindingForSA(ctx, c, policy.Identity, sa.Identity); err != nil {
				return err
			}
		}
	}
	if sa != nil && strings.TrimSpace(roleRef) != "" {
		role, err := resolveOrganisationRole(ctx, c, roleRef)
		if err == nil && role != nil {
			if err := deleteRoleBindingForSA(ctx, c, role.Identity, sa.Identity); err != nil {
				return err
			}
		}
	}
	if sa != nil {
		if err := c.DeleteServiceAccount(ctx, sa.Identity); err != nil {
			return fmt.Errorf("delete service account: %w", err)
		}
	}
	return nil
}

func validateEnsureInput(in EnsureInput) error {
	if strings.TrimSpace(in.Namespace) == "" || strings.TrimSpace(in.ServiceAccount) == "" {
		return fmt.Errorf("kubernetes namespace and service account are required")
	}
	role := strings.TrimSpace(in.RoleRef)
	policy := strings.TrimSpace(in.PolicyRef)
	if role == "" && policy == "" {
		return fmt.Errorf("at least one of policy or role is required")
	}
	if role == "*" || policy == "*" {
		return fmt.Errorf("wildcard policy/role is not allowed")
	}
	if strings.TrimSpace(in.ClusterIdentity) == "" {
		return fmt.Errorf("cluster identity is required")
	}
	if len(in.TrustedAudiences) == 0 {
		return fmt.Errorf("trusted audiences are required")
	}
	if len(in.Scopes) == 0 {
		return fmt.Errorf("scopes are required")
	}
	return nil
}

func ensureServiceAccount(
	ctx context.Context,
	c IAMClient,
	name, subject, ns, k8sSA string,
	labels map[string]string,
) (*iam.ServiceAccount, error) {
	existing, err := findByLabelsSA(ctx, c, labels)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}
	repo := ns + "/" + k8sSA
	desc := fmt.Sprintf("Workload identity for Kubernetes %s; JWT sub: %s", repo, subject)
	return c.CreateServiceAccount(ctx, iam.CreateServiceAccountRequest{
		Name:        name,
		Description: &desc,
		Labels:      labels,
		Annotations: map[string]string{
			AnnotationRepository:      repo,
			AnnotationProviderSubject: subject,
		},
	})
}

func ensureFederatedIdentity(
	ctx context.Context,
	c IAMClient,
	provider *iam.FederatedIdentityProvider,
	sa *iam.ServiceAccount,
	fiName, subject string,
	scopes []iam.AccessCredentialsScope,
	audiences []string,
	labels map[string]string,
) (*iam.FederatedIdentity, error) {
	desc := fmt.Sprintf("WIF binding for %s", subject)
	existing, err := findByLabelsFI(ctx, c, labels)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return c.CreateFederatedIdentity(ctx, iam.CreateFederatedIdentityRequest{
			Name:                   fiName,
			Description:            desc,
			Labels:                 labels,
			Annotations:            map[string]string{AnnotationProviderSubject: subject},
			ServiceAccountIdentity: sa.Identity,
			ProviderIdentity:       provider.Identity,
			ProviderSubject:        subject,
			TrustedAudiences:       append([]string(nil), audiences...),
			AudienceMatchMode:      iam.AudienceMatchModeAny,
			AllowedScopes:          scopes,
		})
	}
	if !fiNeedsUpdate(existing, fiName, desc, subject, scopes, audiences, labels) {
		return existing, nil
	}
	return c.UpdateFederatedIdentity(ctx, existing.Identity, iam.UpdateFederatedIdentityRequest{
		Name:              fiName,
		Description:       desc,
		Labels:            labels,
		Annotations:       map[string]string{AnnotationProviderSubject: subject},
		TrustedAudiences:  append([]string(nil), audiences...),
		AudienceMatchMode: iam.AudienceMatchModeAny,
		AllowedScopes:     scopes,
	})
}

func fiNeedsUpdate(
	fi *iam.FederatedIdentity,
	name, desc, subject string,
	scopes []iam.AccessCredentialsScope,
	audiences []string,
	labels map[string]string,
) bool {
	annSubj := ""
	if fi.Annotations != nil {
		annSubj = fi.Annotations[AnnotationProviderSubject]
	}
	return !scopesEqual(fi.AllowedScopes, scopes) ||
		!audiencesEqual(fi.TrustedAudiences, audiences) ||
		(fi.AudienceMatchMode != "" && fi.AudienceMatchMode != iam.AudienceMatchModeAny) ||
		fi.Name != name ||
		fi.Description != desc ||
		!LabelsMatch(fi.Labels, labels) ||
		annSubj != subject ||
		fi.ProviderSubject != subject
}

func ensureRoleBinding(ctx context.Context, c IAMClient, role *iam.OrganisationRole, sa *iam.ServiceAccount, key string, labels map[string]string) error {
	ok, err := hasRoleBindingForSA(ctx, c, role.Identity, sa.Identity)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	name := fmt.Sprintf("wif-k8s-%s", key)
	if len(name) > 63 {
		name = name[:63]
	}
	saID := sa.Identity
	_, err = c.CreateRoleBinding(ctx, role.Identity, iam.CreateRoleBinding{
		Name:                   name,
		Description:            fmt.Sprintf("WIF binding for Thalassa SA %s", sa.Identity),
		Labels:                 labels,
		ServiceAccountIdentity: &saID,
	})
	return err
}

func ensurePolicyBinding(ctx context.Context, c IAMClient, policy *iam.IamPolicy, sa *iam.ServiceAccount, key string, labels map[string]string) error {
	ok, err := hasPolicyBindingForSA(ctx, c, policy.Identity, sa.Identity)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	name := fmt.Sprintf("wif-k8s-%s", key)
	if len(name) > 63 {
		name = name[:63]
	}
	saID := sa.Identity
	_, err = c.CreateIamPolicyBinding(ctx, policy.Identity, iam.CreateIamPolicyBindingRequest{
		Name:                   name,
		Description:            fmt.Sprintf("WIF policy binding for Thalassa SA %s", sa.Identity),
		Labels:                 labels,
		ServiceAccountIdentity: &saID,
	})
	return err
}

func resolveOrganisationRole(ctx context.Context, c IAMClient, ref string) (*iam.OrganisationRole, error) {
	ref = strings.TrimSpace(ref)
	if role, err := c.GetOrganisationRole(ctx, ref); err == nil && role != nil {
		return role, nil
	}
	roles, err := c.ListOrganisationRoles(ctx, &iam.ListOrganisationRolesRequest{})
	if err != nil {
		return nil, fmt.Errorf("list organisation roles: %w", err)
	}
	for i := range roles {
		r := &roles[i]
		if strings.EqualFold(r.Identity, ref) || strings.EqualFold(r.Slug, ref) || strings.EqualFold(r.Name, ref) {
			return r, nil
		}
	}
	return nil, fmt.Errorf("organisation role not found: %s", ref)
}

func resolveIamPolicy(ctx context.Context, c IAMClient, ref string) (*iam.IamPolicy, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf("iam policy ref is required")
	}
	// Prefer direct identity lookup.
	if policy, err := c.GetIamPolicy(ctx, ref); err == nil && policy != nil && strings.TrimSpace(policy.Identity) != "" {
		return policy, nil
	}
	policies, err := c.ListIamPolicies(ctx, &iam.ListIamPoliciesRequest{})
	if err != nil {
		return nil, fmt.Errorf("list iam policies: %w", err)
	}
	var bySlugOrName *iam.IamPolicy
	for i := range policies {
		p := &policies[i]
		if strings.EqualFold(p.Identity, ref) {
			return p, nil
		}
		if bySlugOrName == nil && (strings.EqualFold(p.Slug, ref) || strings.EqualFold(p.Name, ref)) {
			bySlugOrName = p
		}
	}
	if bySlugOrName != nil {
		return bySlugOrName, nil
	}
	return nil, fmt.Errorf("iam policy not found: %s", ref)
}

func findByLabelsSA(ctx context.Context, c IAMClient, want map[string]string) (*iam.ServiceAccount, error) {
	list, err := c.ListServiceAccounts(ctx, &iam.ListServiceAccountsRequest{
		Filters: []filters.Filter{&filters.LabelFilter{MatchLabels: want}},
	})
	if err != nil {
		return nil, err
	}
	for i := range list {
		if LabelsMatch(list[i].Labels, want) {
			return &list[i], nil
		}
	}
	return nil, nil
}

func findByLabelsFI(ctx context.Context, c IAMClient, want map[string]string) (*iam.FederatedIdentity, error) {
	list, err := c.ListFederatedIdentities(ctx, &iam.ListFederatedIdentitiesRequest{
		Filters: []filters.Filter{&filters.LabelFilter{MatchLabels: want}},
	})
	if err != nil {
		return nil, err
	}
	for i := range list {
		if LabelsMatch(list[i].Labels, want) {
			return &list[i], nil
		}
	}
	return nil, nil
}

func hasRoleBindingForSA(ctx context.Context, c IAMClient, roleIdentity, saIdentity string) (bool, error) {
	bindings, err := c.ListRoleBindings(ctx, roleIdentity, &iam.ListRoleBindingsRequest{})
	if err != nil {
		return false, err
	}
	for _, b := range bindings {
		if b.ServiceAccount != nil && b.ServiceAccount.Identity == saIdentity {
			return true, nil
		}
	}
	return false, nil
}

func hasPolicyBindingForSA(ctx context.Context, c IAMClient, policyIdentity, saIdentity string) (bool, error) {
	bindings, err := c.ListIamPolicyBindings(ctx, policyIdentity, &iam.ListIamPolicyBindingsRequest{})
	if err != nil {
		return false, err
	}
	for _, b := range bindings {
		if b.ServiceAccount != nil && b.ServiceAccount.Identity == saIdentity {
			return true, nil
		}
	}
	return false, nil
}

func deleteRoleBindingForSA(ctx context.Context, c IAMClient, roleIdentity, saIdentity string) error {
	bindings, err := c.ListRoleBindings(ctx, roleIdentity, &iam.ListRoleBindingsRequest{})
	if err != nil {
		return fmt.Errorf("list role bindings: %w", err)
	}
	for _, b := range bindings {
		if b.ServiceAccount != nil && b.ServiceAccount.Identity == saIdentity {
			if LabelsMatch(b.Labels, map[string]string{LabelManagedBy: ValueManagedBy}) {
				if err := c.DeleteRoleBinding(ctx, roleIdentity, b.Identity); err != nil {
					return fmt.Errorf("delete role binding: %w", err)
				}
			}
		}
	}
	return nil
}

func deletePolicyBindingForSA(ctx context.Context, c IAMClient, policyIdentity, saIdentity string) error {
	bindings, err := c.ListIamPolicyBindings(ctx, policyIdentity, &iam.ListIamPolicyBindingsRequest{})
	if err != nil {
		return fmt.Errorf("list iam policy bindings: %w", err)
	}
	for _, b := range bindings {
		if b.ServiceAccount != nil && b.ServiceAccount.Identity == saIdentity {
			if LabelsMatch(b.Labels, map[string]string{LabelManagedBy: ValueManagedBy}) {
				if err := c.DeleteIamPolicyBinding(ctx, policyIdentity, b.Identity); err != nil {
					return fmt.Errorf("delete iam policy binding: %w", err)
				}
			}
		}
	}
	return nil
}

func scopesEqual(a, b []iam.AccessCredentialsScope) bool {
	toSorted := func(s []iam.AccessCredentialsScope) []string {
		out := make([]string, 0, len(s))
		for _, x := range s {
			out = append(out, string(x))
		}
		slices.Sort(out)
		return out
	}
	return slices.Equal(toSorted(a), toSorted(b))
}

func audiencesEqual(a, b []string) bool {
	aa := append([]string(nil), a...)
	bb := append([]string(nil), b...)
	slices.Sort(aa)
	slices.Sort(bb)
	return slices.Equal(aa, bb)
}
