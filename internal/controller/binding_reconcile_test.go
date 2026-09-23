package controller

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/thalassa-cloud/thalassa-workload-identity-controller/internal/config"
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
	// ConfigMap sync is opt-in; zero-value and explicit false must skip the path.
	require.False(t, config.Config{}.EnableIdentityConfigMap)
	require.False(t, config.Config{EnableIdentityConfigMap: false}.EnableIdentityConfigMap)
	require.True(t, config.Config{EnableIdentityConfigMap: true}.EnableIdentityConfigMap)
}
