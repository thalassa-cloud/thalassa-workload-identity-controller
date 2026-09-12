package config

import (
	"fmt"
	"strings"
)

// RefAllowed reports whether ref (and optional resolved identity/slug/name) is
// permitted. An empty allowlist means all refs are allowed.
func RefAllowed(ref string, allowlist []string, resolved ...string) bool {
	if len(allowlist) == 0 {
		return true
	}
	candidates := make([]string, 0, 1+len(resolved))
	if r := strings.TrimSpace(ref); r != "" {
		candidates = append(candidates, r)
	}
	for _, r := range resolved {
		if t := strings.TrimSpace(r); t != "" {
			candidates = append(candidates, t)
		}
	}
	for _, allowed := range allowlist {
		allowed = strings.TrimSpace(allowed)
		if allowed == "" {
			continue
		}
		for _, c := range candidates {
			if strings.EqualFold(allowed, c) {
				return true
			}
		}
	}
	return false
}

// CheckRefAllowlist returns an error when allowlist is non-empty and ref is not allowed.
func CheckRefAllowlist(kind, ref string, allowlist []string, resolved ...string) error {
	ref = strings.TrimSpace(ref)
	if ref == "" || len(allowlist) == 0 {
		return nil
	}
	if RefAllowed(ref, allowlist, resolved...) {
		return nil
	}
	return fmt.Errorf("%s %q is not in the configured allowlist", kind, ref)
}
