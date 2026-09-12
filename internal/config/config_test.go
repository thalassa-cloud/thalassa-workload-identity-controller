package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConfigValidate(t *testing.T) {
	valid := Config{
		ThalassaURL:              DefaultThalassaURL,
		OrganisationID:           "org-1",
		ClusterIdentity:          "k8s-1",
		ControllerServiceAccount: "sa-ctrl",
		SubjectTokenFile:         "/var/run/secrets/thalassa/token",
		TrustedAudiences:         []string{"https://api.thalassa.cloud"},
		RequeueMissingIDP:        time.Second,
	}
	require.NoError(t, valid.Validate())

	bad := valid
	bad.OrganisationID = ""
	require.Error(t, bad.Validate())
}
