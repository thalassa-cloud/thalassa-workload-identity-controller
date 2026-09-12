package wif

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thalassa-cloud/client-go/iam"
)

func TestBuildKubernetesSubject(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		sa        string
		want      string
		wantErr   string
	}{
		{name: "ok", namespace: "monitoring", sa: "prometheus", want: "system:serviceaccount:monitoring:prometheus"},
		{name: "empty ns", namespace: "", sa: "x", wantErr: "required"},
		{name: "slash", namespace: "a/b", sa: "x", wantErr: "must not contain"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildKubernetesSubject(tt.namespace, tt.sa)
			if tt.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestResourceKeyStable(t *testing.T) {
	sub := "system:serviceaccount:ns:sa"
	a := ResourceKey("ns", "sa", sub, "https://oidc.example/")
	b := ResourceKey("ns", "sa", sub, "https://oidc.example")
	require.Equal(t, a, b)
	require.Len(t, a, 16)
	c := ResourceKey("other", "sa", sub, "https://oidc.example")
	require.NotEqual(t, a, c)
}

func TestParseScopes(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    []iam.AccessCredentialsScope
		wantErr string
	}{
		{
			name: "default",
			raw:  "",
			want: []iam.AccessCredentialsScope{iam.AccessCredentialsScopeAPIRead},
		},
		{
			name: "custom",
			raw:  "api:read,kubernetes",
			want: []iam.AccessCredentialsScope{iam.AccessCredentialsScopeAPIRead, iam.AccessCredentialsScopeKubernetes},
		},
		{name: "wildcard", raw: "*", wantErr: "wildcard"},
		{name: "unknown", raw: "nope", wantErr: "unsupported"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseScopes(tt.raw)
			if tt.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestIsEnabledAndDeleteResources(t *testing.T) {
	require.False(t, IsEnabled(nil))
	require.True(t, IsEnabled(map[string]string{AnnotationEnabled: "true"}))
	require.False(t, DeleteResources(nil))
	require.True(t, DeleteResources(map[string]string{AnnotationDeleteResources: "true"}))
}
