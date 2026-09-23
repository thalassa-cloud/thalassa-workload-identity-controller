package controller

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thalassa-cloud/client-go/iam"

	"github.com/thalassa-cloud/thalassa-workload-identity-controller/internal/config"
	"github.com/thalassa-cloud/thalassa-workload-identity-controller/internal/wif"
)

func TestValidateDesired(t *testing.T) {
	tests := []struct {
		name    string
		desired bindingDesired
		wantErr string
	}{
		{
			name: "policy without project uses org root",
			desired: bindingDesired{
				Namespace: "ns", ServiceAccount: "sa", PolicyRef: "obs-write",
			},
		},
		{
			name: "role only ok",
			desired: bindingDesired{
				Namespace: "ns", ServiceAccount: "sa", RoleRef: "reader",
			},
		},
		{
			name: "missing policy and role",
			desired: bindingDesired{
				Namespace: "ns", ServiceAccount: "sa",
			},
			wantErr: "at least one of policyRef or roleRef",
		},
		{
			name: "wildcard denied",
			desired: bindingDesired{
				Namespace: "ns", ServiceAccount: "sa", PolicyRef: "*",
			},
			wantErr: "wildcard",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDesired(tt.desired)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestIdentityConfigMapGatedByConfig(t *testing.T) {
	require.False(t, config.Config{}.EnableIdentityConfigMap)
	require.False(t, config.Config{EnableIdentityConfigMap: false}.EnableIdentityConfigMap)
	require.True(t, config.Config{EnableIdentityConfigMap: true}.EnableIdentityConfigMap)
}

func TestDeleteCloudResourcesSkipsMissingIDP(t *testing.T) {
	r := &bindingReconciler{
		IAM: &emptyProvidersIAM{},
		Config: config.Config{
			ClusterIdentity: "k8s-missing",
		},
	}
	err := r.deleteCloudResources(context.Background(), bindingDesired{
		Namespace: "ns", ServiceAccount: "sa", PolicyRef: "pol-1",
	})
	require.NoError(t, err)
}

// emptyProvidersIAM returns no IdPs so FindProviderByClusterID yields ErrProviderNotReady.
type emptyProvidersIAM struct{}

func (emptyProvidersIAM) ListFederatedIdentityProviders(context.Context, *iam.ListFederatedIdentityProvidersRequest) ([]iam.FederatedIdentityProvider, error) {
	return nil, nil
}
func (emptyProvidersIAM) ListServiceAccounts(context.Context, *iam.ListServiceAccountsRequest) ([]iam.ServiceAccount, error) {
	return nil, nil
}
func (emptyProvidersIAM) CreateServiceAccount(context.Context, iam.CreateServiceAccountRequest) (*iam.ServiceAccount, error) {
	panic("unexpected")
}
func (emptyProvidersIAM) DeleteServiceAccount(context.Context, string) error { panic("unexpected") }
func (emptyProvidersIAM) ListFederatedIdentities(context.Context, *iam.ListFederatedIdentitiesRequest) ([]iam.FederatedIdentity, error) {
	return nil, nil
}
func (emptyProvidersIAM) CreateFederatedIdentity(context.Context, iam.CreateFederatedIdentityRequest) (*iam.FederatedIdentity, error) {
	panic("unexpected")
}
func (emptyProvidersIAM) UpdateFederatedIdentity(context.Context, string, iam.UpdateFederatedIdentityRequest) (*iam.FederatedIdentity, error) {
	panic("unexpected")
}
func (emptyProvidersIAM) DeleteFederatedIdentity(context.Context, string) error {
	panic("unexpected")
}
func (emptyProvidersIAM) GetOrganisationRole(context.Context, string) (*iam.OrganisationRole, error) {
	panic("unexpected")
}
func (emptyProvidersIAM) ListOrganisationRoles(context.Context, *iam.ListOrganisationRolesRequest) ([]iam.OrganisationRole, error) {
	return nil, nil
}
func (emptyProvidersIAM) ListRoleBindings(context.Context, string, *iam.ListRoleBindingsRequest) ([]iam.OrganisationRoleBinding, error) {
	return nil, nil
}
func (emptyProvidersIAM) CreateRoleBinding(context.Context, string, iam.CreateRoleBinding) (*iam.OrganisationRoleBinding, error) {
	panic("unexpected")
}
func (emptyProvidersIAM) DeleteRoleBinding(context.Context, string, string) error {
	panic("unexpected")
}
func (emptyProvidersIAM) GetIamPolicy(context.Context, string) (*iam.IamPolicy, error) {
	panic("unexpected")
}
func (emptyProvidersIAM) ListIamPolicies(context.Context, *iam.ListIamPoliciesRequest) ([]iam.IamPolicy, error) {
	return nil, nil
}
func (emptyProvidersIAM) ListIamPolicyBindings(context.Context, string, *iam.ListIamPolicyBindingsRequest) ([]iam.IamPolicyBinding, error) {
	return nil, nil
}
func (emptyProvidersIAM) CreateIamPolicyBinding(context.Context, string, iam.CreateIamPolicyBindingRequest) (*iam.IamPolicyBinding, error) {
	panic("unexpected")
}
func (emptyProvidersIAM) DeleteIamPolicyBinding(context.Context, string, string) error {
	panic("unexpected")
}

var _ wif.IAMClient = emptyProvidersIAM{}
