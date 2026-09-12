package wif

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/thalassa-cloud/client-go/iam"
)

// BuildKubernetesSubject returns system:serviceaccount:<namespace>:<name>.
func BuildKubernetesSubject(namespace, name string) (string, error) {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" || name == "" {
		return "", fmt.Errorf("namespace and name are required")
	}
	if strings.Contains(namespace, "/") || strings.Contains(name, "/") {
		return "", fmt.Errorf("namespace and name must not contain '/'")
	}
	return fmt.Sprintf("system:serviceaccount:%s:%s", namespace, name), nil
}

// NormalizeIssuer trims space and trailing slash.
func NormalizeIssuer(issuer string) string {
	return strings.TrimSuffix(strings.TrimSpace(issuer), "/")
}

// ResourceKey is a stable id for controller-owned Thalassa resources.
// Hash input: kubernetes\n<ns>/<sa>\n<subject>\n<normalized issuer>
func ResourceKey(namespace, saName, subject, issuer string) string {
	repo := strings.TrimSpace(namespace) + "/" + strings.TrimSpace(saName)
	sum := sha256.Sum256([]byte(
		strings.ToLower(ValueVCSKubernetes) + "\n" +
			repo + "\n" +
			subject + "\n" +
			NormalizeIssuer(issuer),
	))
	return hex.EncodeToString(sum[:8])
}

// OwnershipLabels returns labels applied to Thalassa SA / FI / role binding.
func OwnershipLabels(key, namespace, saName string) map[string]string {
	return map[string]string{
		LabelManagedBy:         ValueManagedBy,
		LabelWIFKey:            key,
		LabelWIFVCS:            ValueVCSKubernetes,
		LabelK8sNamespace:      namespace,
		LabelK8sServiceAccount: saName,
	}
}

// LabelsMatch reports whether have contains every key/value in want.
func LabelsMatch(have, want map[string]string) bool {
	if len(want) == 0 {
		return false
	}
	for k, v := range want {
		if have[k] != v {
			return false
		}
	}
	return true
}

// DefaultResourceNames returns Thalassa SA and FI display names.
func DefaultResourceNames(key, override string) (saName, fiName string) {
	override = strings.TrimSpace(override)
	if override != "" {
		saName = sanitizeName(override)
		fiName = sanitizeName(override + "-fi")
		return saName, fiName
	}
	saName = sanitizeName(fmt.Sprintf("wif-k8s-%s", key))
	fiName = sanitizeName(fmt.Sprintf("wif-k8s-%s-fi", key))
	return saName, fiName
}

func sanitizeName(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}

// ParseScopes parses comma-separated scopes; empty → api:read.
func ParseScopes(raw string) ([]iam.AccessCredentialsScope, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ParseScopeList(nil)
	}
	return ParseScopeList(strings.Split(raw, ","))
}

// ParseScopeList parses a list of scopes; empty/nil → api:read.
func ParseScopeList(parts []string) ([]iam.AccessCredentialsScope, error) {
	if len(parts) == 0 {
		return []iam.AccessCredentialsScope{
			iam.AccessCredentialsScopeAPIRead,
		}, nil
	}
	out := make([]iam.AccessCredentialsScope, 0, len(parts))
	seen := map[string]struct{}{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if p == "*" {
			return nil, fmt.Errorf("wildcard scope is not allowed")
		}
		scope, ok := knownScope(p)
		if !ok {
			return nil, fmt.Errorf("unsupported scope %q", p)
		}
		if _, dup := seen[p]; dup {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, scope)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("at least one scope is required")
	}
	return out, nil
}

func knownScope(s string) (iam.AccessCredentialsScope, bool) {
	switch iam.AccessCredentialsScope(s) {
	case iam.AccessCredentialsScopeAPIRead,
		iam.AccessCredentialsScopeAPIWrite,
		iam.AccessCredentialsScopeKubernetes,
		iam.AccessCredentialsScopeObjectStorage:
		return iam.AccessCredentialsScope(s), true
	default:
		return "", false
	}
}

// IsEnabled reports whether wif.enabled is true.
func IsEnabled(annotations map[string]string) bool {
	if annotations == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(annotations[AnnotationEnabled]), "true")
}

// DeleteResources reports whether delete-resources is true.
func DeleteResources(annotations map[string]string) bool {
	if annotations == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(annotations[AnnotationDeleteResources]), "true")
}
