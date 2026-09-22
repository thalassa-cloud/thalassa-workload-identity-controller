package controller

import (
	"fmt"
	"strings"

	"github.com/thalassa-cloud/thalassa-workload-identity-controller/internal/wif"
)

// IdentityConfigMapName returns the ConfigMap name for a Kubernetes ServiceAccount.
func IdentityConfigMapName(serviceAccountName string) string {
	return "wif-" + strings.TrimSpace(serviceAccountName)
}

// IdentityConfigMapData builds ConfigMap data for OIDC token exchange.
// projectID is omitted when empty.
func IdentityConfigMapData(organisationID, serviceAccountID, projectID string) map[string]string {
	data := map[string]string{
		wif.ConfigMapKeyOrganisationID:   strings.TrimSpace(organisationID),
		wif.ConfigMapKeyServiceAccountID: strings.TrimSpace(serviceAccountID),
	}
	if p := strings.TrimSpace(projectID); p != "" {
		data[wif.ConfigMapKeyProjectID] = p
	}
	return data
}

// ValidateIdentityConfigMapData fails closed when required exchange IDs are missing.
func ValidateIdentityConfigMapData(data map[string]string) error {
	if strings.TrimSpace(data[wif.ConfigMapKeyOrganisationID]) == "" {
		return fmt.Errorf("organisation-id is required")
	}
	if strings.TrimSpace(data[wif.ConfigMapKeyServiceAccountID]) == "" {
		return fmt.Errorf("service-account-id is required")
	}
	return nil
}
