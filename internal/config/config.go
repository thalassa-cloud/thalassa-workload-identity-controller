package config

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	DefaultThalassaURL       = "https://api.thalassa.cloud"
	DefaultProbeAddr         = ":8081"
	DefaultMetricsAddr       = ":8443"
	DefaultRequeueMissingIDP = 30 * time.Second
	DefaultWebhookAudience   = DefaultThalassaURL
)

// Config holds validated process configuration for controller and/or webhook.
type Config struct {
	ThalassaURL                     string
	OrganisationID                  string
	ProjectIdentity                 string
	ClusterIdentity                 string
	ControllerServiceAccount        string
	SubjectTokenFile                string
	TrustedAudiences                []string
	WatchNamespaces                 []string
	AllowedPolicies                 []string // empty = allow any policyRef
	AllowedRoles                    []string // empty = allow any roleRef
	ProbeAddr                       string
	MetricsAddr                     string
	EnableLeaderElection            bool
	RequeueMissingIDP               time.Duration
	EnableServiceAccountAnnotations bool
	EnableIdentityConfigMap         bool // sync ConfigMap for all bindings when true
	EnableControllers               bool
	EnablePodMutator                bool
	WebhookCertDir                  string
	WebhookPort                     int
	WebhookAudience                 string // projected token audience for mutator injection
}

// Validate fails closed based on which components are enabled.
func (c *Config) Validate() error {
	if !c.EnableControllers && !c.EnablePodMutator {
		return fmt.Errorf("at least one of enable-controllers or enable-pod-mutator is required")
	}
	if c.EnableControllers {
		if err := c.ValidateController(); err != nil {
			return err
		}
	}
	if c.EnablePodMutator {
		if err := c.ValidateWebhook(); err != nil {
			return err
		}
	}
	return nil
}

// ValidateController fails closed on Thalassa reconcile settings.
func (c *Config) ValidateController() error {
	if strings.TrimSpace(c.ThalassaURL) == "" {
		return fmt.Errorf("thalassa-url is required")
	}
	if _, err := url.ParseRequestURI(c.ThalassaURL); err != nil {
		return fmt.Errorf("thalassa-url is invalid: %w", err)
	}
	if strings.TrimSpace(c.OrganisationID) == "" {
		return fmt.Errorf("organisation is required")
	}
	if strings.TrimSpace(c.ClusterIdentity) == "" {
		return fmt.Errorf("cluster-identity is required")
	}
	if strings.TrimSpace(c.ControllerServiceAccount) == "" {
		return fmt.Errorf("thalassa-service-account-id is required")
	}
	if strings.TrimSpace(c.SubjectTokenFile) == "" {
		return fmt.Errorf("thalassa-subject-token-file is required")
	}
	if len(c.TrustedAudiences) == 0 {
		return fmt.Errorf("at least one trusted audience is required")
	}
	if c.RequeueMissingIDP <= 0 {
		return fmt.Errorf("requeue-missing-idp must be positive")
	}
	return nil
}

// ValidateWebhook fails closed on mutator settings.
func (c *Config) ValidateWebhook() error {
	if strings.TrimSpace(c.WebhookCertDir) == "" {
		return fmt.Errorf("webhook-cert-dir is required")
	}
	if c.WebhookPort <= 0 {
		return fmt.Errorf("webhook-port must be positive")
	}
	if strings.TrimSpace(c.WebhookAudience) == "" {
		return fmt.Errorf("webhook-audience is required")
	}
	if _, err := url.ParseRequestURI(c.WebhookAudience); err != nil {
		return fmt.Errorf("webhook-audience is invalid: %w", err)
	}
	return nil
}

// NormalizeURL trims trailing slashes from a base URL.
func NormalizeURL(u string) string {
	return strings.TrimSuffix(strings.TrimSpace(u), "/")
}

// ResolveWebhookAudience returns the audience used for injected projected tokens.
func (c *Config) ResolveWebhookAudience() string {
	if a := strings.TrimSpace(c.WebhookAudience); a != "" {
		return a
	}
	if len(c.TrustedAudiences) > 0 {
		return strings.TrimSpace(c.TrustedAudiences[0])
	}
	if u := strings.TrimSpace(c.ThalassaURL); u != "" {
		return u
	}
	return DefaultWebhookAudience
}
