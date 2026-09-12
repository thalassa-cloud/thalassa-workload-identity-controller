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
	DefaultMetricsAddr       = ":8080"
	DefaultRequeueMissingIDP = 30 * time.Second
)

// Config holds validated controller configuration.
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
}

// Validate fails closed on missing required fields.
func (c *Config) Validate() error {
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

// NormalizeURL trims trailing slashes from a base URL.
func NormalizeURL(u string) string {
	return strings.TrimSuffix(strings.TrimSpace(u), "/")
}
