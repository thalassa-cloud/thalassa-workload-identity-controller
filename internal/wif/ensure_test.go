package wif

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thalassa-cloud/client-go/iam"
)

type fakeIAM struct {
	providers []iam.FederatedIdentityProvider
	sas       []iam.ServiceAccount
	fis       []iam.FederatedIdentity
	roles     []iam.OrganisationRole
	bindings  map[string][]iam.OrganisationRoleBinding
	policies  []iam.IamPolicy
	pBindings map[string][]iam.IamPolicyBinding

	createSACalls int
	createFICalls int
	updateFICalls int
	createRBCalls int
	createPBCalls int
	deleteSACalls int
	deleteFICalls int
}

func (f *fakeIAM) ListFederatedIdentityProviders(context.Context, *iam.ListFederatedIdentityProvidersRequest) ([]iam.FederatedIdentityProvider, error) {
	return append([]iam.FederatedIdentityProvider(nil), f.providers...), nil
}
func (f *fakeIAM) ListServiceAccounts(context.Context, *iam.ListServiceAccountsRequest) ([]iam.ServiceAccount, error) {
	return append([]iam.ServiceAccount(nil), f.sas...), nil
}
func (f *fakeIAM) CreateServiceAccount(_ context.Context, create iam.CreateServiceAccountRequest) (*iam.ServiceAccount, error) {
	f.createSACalls++
	sa := iam.ServiceAccount{Identity: "sa-new", Name: create.Name, Labels: create.Labels, Annotations: create.Annotations}
	f.sas = append(f.sas, sa)
	return &sa, nil
}
func (f *fakeIAM) DeleteServiceAccount(_ context.Context, identity string) error {
	f.deleteSACalls++
	out := f.sas[:0]
	for _, sa := range f.sas {
		if sa.Identity != identity {
			out = append(out, sa)
		}
	}
	f.sas = out
	return nil
}
func (f *fakeIAM) ListFederatedIdentities(context.Context, *iam.ListFederatedIdentitiesRequest) ([]iam.FederatedIdentity, error) {
	return append([]iam.FederatedIdentity(nil), f.fis...), nil
}
func (f *fakeIAM) CreateFederatedIdentity(_ context.Context, create iam.CreateFederatedIdentityRequest) (*iam.FederatedIdentity, error) {
	f.createFICalls++
	fi := iam.FederatedIdentity{
		Identity:          "fi-new",
		Name:              create.Name,
		Description:       create.Description,
		Labels:            create.Labels,
		Annotations:       create.Annotations,
		ProviderSubject:   create.ProviderSubject,
		TrustedAudiences:  create.TrustedAudiences,
		AudienceMatchMode: create.AudienceMatchMode,
		AllowedScopes:     create.AllowedScopes,
	}
	f.fis = append(f.fis, fi)
	return &fi, nil
}
func (f *fakeIAM) UpdateFederatedIdentity(_ context.Context, identity string, update iam.UpdateFederatedIdentityRequest) (*iam.FederatedIdentity, error) {
	f.updateFICalls++
	for i := range f.fis {
		if f.fis[i].Identity == identity {
			f.fis[i].Name = update.Name
			f.fis[i].Description = update.Description
			f.fis[i].Labels = update.Labels
			f.fis[i].Annotations = update.Annotations
			f.fis[i].TrustedAudiences = update.TrustedAudiences
			f.fis[i].AudienceMatchMode = update.AudienceMatchMode
			f.fis[i].AllowedScopes = update.AllowedScopes
			return &f.fis[i], nil
		}
	}
	return nil, errors.New("not found")
}
func (f *fakeIAM) DeleteFederatedIdentity(_ context.Context, identity string) error {
	f.deleteFICalls++
	out := f.fis[:0]
	for _, fi := range f.fis {
		if fi.Identity != identity {
			out = append(out, fi)
		}
	}
	f.fis = out
	return nil
}
func (f *fakeIAM) GetOrganisationRole(_ context.Context, identity string) (*iam.OrganisationRole, error) {
	for i := range f.roles {
		if f.roles[i].Identity == identity {
			return &f.roles[i], nil
		}
	}
	return nil, errors.New("not found")
}
func (f *fakeIAM) ListOrganisationRoles(context.Context, *iam.ListOrganisationRolesRequest) ([]iam.OrganisationRole, error) {
	return append([]iam.OrganisationRole(nil), f.roles...), nil
}
func (f *fakeIAM) ListRoleBindings(_ context.Context, roleIdentity string, _ *iam.ListRoleBindingsRequest) ([]iam.OrganisationRoleBinding, error) {
	return append([]iam.OrganisationRoleBinding(nil), f.bindings[roleIdentity]...), nil
}
func (f *fakeIAM) CreateRoleBinding(_ context.Context, roleIdentity string, create iam.CreateRoleBinding) (*iam.OrganisationRoleBinding, error) {
	f.createRBCalls++
	b := iam.OrganisationRoleBinding{Identity: "rb-new", Name: create.Name, Labels: create.Labels}
	if create.ServiceAccountIdentity != nil {
		b.ServiceAccount = &iam.ServiceAccount{Identity: *create.ServiceAccountIdentity}
	}
	f.bindings[roleIdentity] = append(f.bindings[roleIdentity], b)
	return &b, nil
}
func (f *fakeIAM) DeleteRoleBinding(_ context.Context, roleIdentity, bindingIdentity string) error {
	out := f.bindings[roleIdentity][:0]
	for _, b := range f.bindings[roleIdentity] {
		if b.Identity != bindingIdentity {
			out = append(out, b)
		}
	}
	f.bindings[roleIdentity] = out
	return nil
}
func (f *fakeIAM) GetIamPolicy(_ context.Context, identity string) (*iam.IamPolicy, error) {
	for i := range f.policies {
		if f.policies[i].Identity == identity {
			return &f.policies[i], nil
		}
	}
	return nil, errors.New("not found")
}
func (f *fakeIAM) ListIamPolicies(context.Context, *iam.ListIamPoliciesRequest) ([]iam.IamPolicy, error) {
	return append([]iam.IamPolicy(nil), f.policies...), nil
}
func (f *fakeIAM) ListIamPolicyBindings(_ context.Context, policyIdentity string, _ *iam.ListIamPolicyBindingsRequest) ([]iam.IamPolicyBinding, error) {
	return append([]iam.IamPolicyBinding(nil), f.pBindings[policyIdentity]...), nil
}
func (f *fakeIAM) CreateIamPolicyBinding(_ context.Context, policyIdentity string, create iam.CreateIamPolicyBindingRequest) (*iam.IamPolicyBinding, error) {
	f.createPBCalls++
	b := iam.IamPolicyBinding{Identity: "pb-new", Name: create.Name, Labels: create.Labels}
	if create.ServiceAccountIdentity != nil {
		b.ServiceAccount = &iam.ServiceAccount{Identity: *create.ServiceAccountIdentity}
	}
	if f.pBindings == nil {
		f.pBindings = map[string][]iam.IamPolicyBinding{}
	}
	f.pBindings[policyIdentity] = append(f.pBindings[policyIdentity], b)
	return &b, nil
}
func (f *fakeIAM) DeleteIamPolicyBinding(_ context.Context, policyIdentity, bindingIdentity string) error {
	out := f.pBindings[policyIdentity][:0]
	for _, b := range f.pBindings[policyIdentity] {
		if b.Identity != bindingIdentity {
			out = append(out, b)
		}
	}
	f.pBindings[policyIdentity] = out
	return nil
}

func TestEnsureWorkloadIdentityCreates(t *testing.T) {
	f := &fakeIAM{
		providers: []iam.FederatedIdentityProvider{{
			Identity:       "idp-1",
			ProviderIssuer: "https://oidc.example",
			Labels:         map[string]string{LabelKubernetesClusterID: "k8s-1"},
		}},
		roles:    []iam.OrganisationRole{{Identity: "role-1", Name: "Reader", Slug: "reader"}},
		bindings: map[string][]iam.OrganisationRoleBinding{},
	}
	res, err := EnsureWorkloadIdentity(context.Background(), f, EnsureInput{
		Namespace:        "ns",
		ServiceAccount:   "app",
		RoleRef:          "reader",
		Scopes:           []iam.AccessCredentialsScope{iam.AccessCredentialsScopeAPIRead},
		TrustedAudiences: []string{"https://api.thalassa.cloud"},
		ClusterIdentity:  "k8s-1",
	})
	require.NoError(t, err)
	require.Equal(t, "idp-1", res.ProviderID)
	require.Equal(t, "sa-new", res.ServiceAccountID)
	require.Equal(t, "fi-new", res.FederatedIdentityID)
	require.Equal(t, 1, f.createSACalls)
	require.Equal(t, 1, f.createFICalls)
	require.Equal(t, 1, f.createRBCalls)

	// Idempotent second call
	_, err = EnsureWorkloadIdentity(context.Background(), f, EnsureInput{
		Namespace:        "ns",
		ServiceAccount:   "app",
		RoleRef:          "reader",
		Scopes:           []iam.AccessCredentialsScope{iam.AccessCredentialsScopeAPIRead},
		TrustedAudiences: []string{"https://api.thalassa.cloud"},
		ClusterIdentity:  "k8s-1",
	})
	require.NoError(t, err)
	require.Equal(t, 1, f.createSACalls)
	require.Equal(t, 1, f.createFICalls)
	require.Equal(t, 1, f.createRBCalls)
}

func TestEnsureWorkloadIdentityWithPolicy(t *testing.T) {
	f := &fakeIAM{
		providers: []iam.FederatedIdentityProvider{{
			Identity:       "idp-1",
			ProviderIssuer: "https://oidc.example",
			Labels:         map[string]string{LabelKubernetesClusterID: "k8s-1"},
		}},
		policies:  []iam.IamPolicy{{Identity: "pol-1", Name: "Obs Write", Slug: "obs-write"}},
		pBindings: map[string][]iam.IamPolicyBinding{},
	}
	res, err := EnsureWorkloadIdentity(context.Background(), f, EnsureInput{
		Namespace:        "ns",
		ServiceAccount:   "app",
		PolicyRef:        "obs-write",
		Scopes:           []iam.AccessCredentialsScope{iam.AccessCredentialsScopeAPIRead},
		TrustedAudiences: []string{"https://api.thalassa.cloud"},
		ClusterIdentity:  "k8s-1",
	})
	require.NoError(t, err)
	require.Equal(t, "pol-1", res.PolicyID)
	require.Equal(t, 1, f.createPBCalls)
	require.Equal(t, 0, f.createRBCalls)

	_, err = EnsureWorkloadIdentity(context.Background(), f, EnsureInput{
		Namespace:        "ns",
		ServiceAccount:   "app",
		PolicyRef:        "obs-write",
		Scopes:           []iam.AccessCredentialsScope{iam.AccessCredentialsScopeAPIRead},
		TrustedAudiences: []string{"https://api.thalassa.cloud"},
		ClusterIdentity:  "k8s-1",
	})
	require.NoError(t, err)
	require.Equal(t, 1, f.createPBCalls)
}

func TestEnsurePolicyAllowlist(t *testing.T) {
	f := &fakeIAM{
		providers: []iam.FederatedIdentityProvider{{
			Identity:       "idp-1",
			ProviderIssuer: "https://oidc.example",
			Labels:         map[string]string{LabelKubernetesClusterID: "k8s-1"},
		}},
		policies:  []iam.IamPolicy{{Identity: "pol-1", Name: "Obs Write", Slug: "obs-write"}},
		pBindings: map[string][]iam.IamPolicyBinding{},
	}
	_, err := EnsureWorkloadIdentity(context.Background(), f, EnsureInput{
		Namespace:        "ns",
		ServiceAccount:   "app",
		PolicyRef:        "obs-write",
		AllowedPolicies:  []string{"other-policy"},
		Scopes:           []iam.AccessCredentialsScope{iam.AccessCredentialsScopeAPIRead},
		TrustedAudiences: []string{"https://api.thalassa.cloud"},
		ClusterIdentity:  "k8s-1",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "allowlist")
	require.Equal(t, 0, f.createPBCalls)

	res, err := EnsureWorkloadIdentity(context.Background(), f, EnsureInput{
		Namespace:        "ns",
		ServiceAccount:   "app",
		PolicyRef:        "obs-write",
		AllowedPolicies:  []string{"pol-1"},
		Scopes:           []iam.AccessCredentialsScope{iam.AccessCredentialsScopeAPIRead},
		TrustedAudiences: []string{"https://api.thalassa.cloud"},
		ClusterIdentity:  "k8s-1",
	})
	require.NoError(t, err)
	require.Equal(t, "pol-1", res.PolicyID)
}

func TestEnsureRequiresPolicyOrRole(t *testing.T) {
	f := &fakeIAM{
		providers: []iam.FederatedIdentityProvider{{
			Identity: "idp-1", ProviderIssuer: "https://oidc.example",
			Labels: map[string]string{LabelKubernetesClusterID: "k8s-1"},
		}},
	}
	_, err := EnsureWorkloadIdentity(context.Background(), f, EnsureInput{
		Namespace:        "ns",
		ServiceAccount:   "app",
		Scopes:           []iam.AccessCredentialsScope{iam.AccessCredentialsScopeAPIRead},
		TrustedAudiences: []string{"https://api.thalassa.cloud"},
		ClusterIdentity:  "k8s-1",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "at least one of policy or role")
}

func TestFindProviderNotReady(t *testing.T) {
	f := &fakeIAM{}
	_, err := FindProviderByClusterID(context.Background(), f, "k8s-missing")
	require.Error(t, err)
	var nr *ErrProviderNotReady
	require.ErrorAs(t, err, &nr)
}

func TestEnsureUpdatesFederatedIdentity(t *testing.T) {
	key := ResourceKey("ns", "app", "system:serviceaccount:ns:app", "https://oidc.example")
	labels := OwnershipLabels(key, "ns", "app")
	f := &fakeIAM{
		providers: []iam.FederatedIdentityProvider{{
			Identity: "idp-1", ProviderIssuer: "https://oidc.example",
			Labels: map[string]string{LabelKubernetesClusterID: "k8s-1"},
		}},
		sas: []iam.ServiceAccount{{Identity: "sa-1", Labels: labels}},
		fis: []iam.FederatedIdentity{{
			Identity: "fi-1", Name: "old", Description: "old", Labels: labels,
			ProviderSubject:   "system:serviceaccount:ns:app",
			Annotations:       map[string]string{AnnotationProviderSubject: "system:serviceaccount:ns:app"},
			AllowedScopes:     []iam.AccessCredentialsScope{iam.AccessCredentialsScopeAPIRead},
			TrustedAudiences:  []string{"https://api.thalassa.cloud"},
			AudienceMatchMode: iam.AudienceMatchModeAny,
		}},
		roles:    []iam.OrganisationRole{{Identity: "role-1", Slug: "reader"}},
		bindings: map[string][]iam.OrganisationRoleBinding{"role-1": {{Identity: "rb-1", ServiceAccount: &iam.ServiceAccount{Identity: "sa-1"}, Labels: labels}}},
	}
	_, err := EnsureWorkloadIdentity(context.Background(), f, EnsureInput{
		Namespace:        "ns",
		ServiceAccount:   "app",
		RoleRef:          "reader",
		Scopes:           []iam.AccessCredentialsScope{iam.AccessCredentialsScopeAPIRead, iam.AccessCredentialsScopeAPIWrite},
		TrustedAudiences: []string{"https://api.thalassa.cloud"},
		ClusterIdentity:  "k8s-1",
	})
	require.NoError(t, err)
	require.Equal(t, 1, f.updateFICalls)
	require.Equal(t, 0, f.createFICalls)
}
