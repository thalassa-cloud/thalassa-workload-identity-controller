package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func validControllerConfig() Config {
	return Config{
		EnableControllers:        true,
		ThalassaURL:              DefaultThalassaURL,
		OrganisationID:           "org-1",
		ClusterIdentity:          "k8s-1",
		ControllerServiceAccount: "sa-ctrl",
		SubjectTokenFile:         "/var/run/secrets/thalassa/token",
		TrustedAudiences:         []string{"https://api.thalassa.cloud"},
		RequeueMissingIDP:        time.Second,
	}
}

func validWebhookConfig() Config {
	return Config{
		EnablePodMutator: true,
		WebhookCertDir:   "/tmp/certs",
		WebhookPort:      9443,
		WebhookAudience:  DefaultWebhookAudience,
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name: "controller only",
			cfg:  validControllerConfig(),
		},
		{
			name: "webhook only",
			cfg:  validWebhookConfig(),
		},
		{
			name: "both",
			cfg: func() Config {
				c := validControllerConfig()
				c.EnablePodMutator = true
				c.WebhookCertDir = "/tmp/certs"
				c.WebhookPort = 9443
				c.WebhookAudience = DefaultWebhookAudience
				return c
			}(),
		},
		{
			name:    "neither",
			cfg:     Config{},
			wantErr: "at least one of enable-controllers or enable-pod-mutator",
		},
		{
			name: "controller missing org",
			cfg: func() Config {
				c := validControllerConfig()
				c.OrganisationID = ""
				return c
			}(),
			wantErr: "organisation is required",
		},
		{
			name: "webhook missing cert dir",
			cfg: func() Config {
				c := validWebhookConfig()
				c.WebhookCertDir = ""
				return c
			}(),
			wantErr: "webhook-cert-dir is required",
		},
		{
			name: "webhook-only does not require organisation",
			cfg: func() Config {
				c := validWebhookConfig()
				c.OrganisationID = ""
				c.ClusterIdentity = ""
				return c
			}(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestResolveWebhookAudience(t *testing.T) {
	require.Equal(t, "https://custom", (&Config{WebhookAudience: "https://custom"}).ResolveWebhookAudience())
	require.Equal(t, "https://aud", (&Config{TrustedAudiences: []string{"https://aud"}}).ResolveWebhookAudience())
	require.Equal(t, DefaultWebhookAudience, (&Config{}).ResolveWebhookAudience())
}
