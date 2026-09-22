package controller

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/thalassa-cloud/thalassa-workload-identity-controller/internal/wif"
)

const (
	testOrgID = "org-1"
	testSAID  = "sa-1"
)

func TestIdentityConfigMapName(t *testing.T) {
	require.Equal(t, "wif-observability-proxy", IdentityConfigMapName("observability-proxy"))
	require.Equal(t, "wif-app", IdentityConfigMapName("  app  "))
}

func TestIdentityConfigMapData(t *testing.T) {
	tests := []struct {
		name             string
		organisationID   string
		serviceAccountID string
		projectID        string
		wantKeys         []string
		wantMissing      []string
	}{
		{
			name:             "without project",
			organisationID:   testOrgID,
			serviceAccountID: testSAID,
			wantKeys:         []string{wif.ConfigMapKeyOrganisationID, wif.ConfigMapKeyServiceAccountID},
			wantMissing:      []string{wif.ConfigMapKeyProjectID},
		},
		{
			name:             "with project",
			organisationID:   testOrgID,
			serviceAccountID: testSAID,
			projectID:        "proj-1",
			wantKeys: []string{
				wif.ConfigMapKeyOrganisationID,
				wif.ConfigMapKeyServiceAccountID,
				wif.ConfigMapKeyProjectID,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := IdentityConfigMapData(tt.organisationID, tt.serviceAccountID, tt.projectID)
			require.NoError(t, ValidateIdentityConfigMapData(data))
			for _, k := range tt.wantKeys {
				require.NotEmpty(t, data[k], k)
			}
			for _, k := range tt.wantMissing {
				_, ok := data[k]
				require.False(t, ok, k)
			}
			require.Equal(t, tt.organisationID, data[wif.ConfigMapKeyOrganisationID])
			require.Equal(t, tt.serviceAccountID, data[wif.ConfigMapKeyServiceAccountID])
			if tt.projectID != "" {
				require.Equal(t, tt.projectID, data[wif.ConfigMapKeyProjectID])
			}
		})
	}
}

func TestValidateIdentityConfigMapData(t *testing.T) {
	require.Error(t, ValidateIdentityConfigMapData(map[string]string{
		wif.ConfigMapKeyServiceAccountID: testSAID,
	}))
	require.Error(t, ValidateIdentityConfigMapData(map[string]string{
		wif.ConfigMapKeyOrganisationID: testOrgID,
	}))
	require.NoError(t, ValidateIdentityConfigMapData(map[string]string{
		wif.ConfigMapKeyOrganisationID:   testOrgID,
		wif.ConfigMapKeyServiceAccountID: testSAID,
	}))
}
