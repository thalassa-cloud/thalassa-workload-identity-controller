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
		cfg     config.Config
		wantErr string
	}{
		{
			name: "policy without project uses org root",
			desired: bindingDesired{
				Namespace: "ns", ServiceAccount: "sa", PolicyRef: "obs-write",
			},
			cfg: config.Config{},
		},
		{
			name: "policy with project ok",
			desired: bindingDesired{
				Namespace: "ns", ServiceAccount: "sa", PolicyRef: "obs-write",
			},
			cfg: config.Config{ProjectIdentity: "proj-1"},
		},
		{
			name: "role only ok",
			desired: bindingDesired{
				Namespace: "ns", ServiceAccount: "sa", RoleRef: "reader",
			},
			cfg: config.Config{},
		},
		{
			name: "missing policy and role",
			desired: bindingDesired{
				Namespace: "ns", ServiceAccount: "sa",
			},
			cfg:     config.Config{},
			wantErr: "at least one of policyRef or roleRef",
		},
		{
			name: "wildcard denied",
			desired: bindingDesired{
				Namespace: "ns", ServiceAccount: "sa", PolicyRef: "*",
			},
			cfg:     config.Config{},
			wantErr: "wildcard",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDesired(tt.desired, tt.cfg)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
