package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const testPolicySlug = "obs-write"

func TestRefAllowed(t *testing.T) {
	tests := []struct {
		name      string
		ref       string
		allowlist []string
		resolved  []string
		want      bool
	}{
		{name: "empty allowlist allows all", ref: "anything", want: true},
		{name: "match ref", ref: testPolicySlug, allowlist: []string{testPolicySlug}, want: true},
		{name: "match resolved identity", ref: testPolicySlug, allowlist: []string{"pol-1"}, resolved: []string{"pol-1"}, want: true},
		{name: "case insensitive", ref: "Obs-Write", allowlist: []string{testPolicySlug}, want: true},
		{name: "denied", ref: "iam:FullAccess", allowlist: []string{testPolicySlug}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, RefAllowed(tt.ref, tt.allowlist, tt.resolved...))
		})
	}
}

func TestCheckRefAllowlist(t *testing.T) {
	require.NoError(t, CheckRefAllowlist("policy", "x", nil))
	require.NoError(t, CheckRefAllowlist("policy", "obs", []string{"obs"}, "pol-1"))
	err := CheckRefAllowlist("policy", "bad", []string{"obs"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "allowlist")
}
