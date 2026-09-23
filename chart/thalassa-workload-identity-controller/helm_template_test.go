package chart_test

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func chartDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Dir(file)
}

func requireHelm(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skip("helm not installed")
	}
}

func helmTemplate(t *testing.T, release string, extraArgs ...string) string {
	t.Helper()
	requireHelm(t)
	args := append([]string{
		"template", release, chartDir(t),
		"--namespace", "thalassa-system",
	}, extraArgs...)
	cmd := exec.Command("helm", args...)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "helm template failed: %s", string(out))
	return string(out)
}

func TestHelmTemplateControllerOnly(t *testing.T) {
	out := helmTemplate(t, "twic",
		"--set", "thalassa.organisation=org-1",
		"--set", "thalassa.clusterIdentity=k8s-1",
		"--set", "thalassa.serviceAccountId=sa-1",
	)
	require.Contains(t, out, "app.kubernetes.io/component: controller")
	require.NotContains(t, out, "app.kubernetes.io/component: webhook")
	require.Contains(t, out, "--enable-controllers=true")
	require.Contains(t, out, "--enable-pod-mutator=false")
	require.Equal(t, 1, strings.Count(out, "\nkind: Deployment\n"))
}

func TestHelmTemplateWebhookOnly(t *testing.T) {
	values := filepath.Join(chartDir(t), "values-webhook-only.yaml")
	out := helmTemplate(t, "twic-wh", "-f", values)
	require.Contains(t, out, "app.kubernetes.io/component: webhook")
	require.NotContains(t, out, "app.kubernetes.io/component: controller")
	require.Contains(t, out, "--enable-controllers=false")
	require.Contains(t, out, "--enable-pod-mutator=true")
	require.Contains(t, out, "--leader-elect=false")
	require.Equal(t, 1, strings.Count(out, "\nkind: Deployment\n"))
	require.Contains(t, out, "kind: MutatingWebhookConfiguration")
}

func TestHelmTemplateInclusterBoth(t *testing.T) {
	values := filepath.Join(chartDir(t), "values-incluster.yaml")
	out := helmTemplate(t, "twic-all",
		"-f", values,
		"--set", "thalassa.organisation=org-1",
		"--set", "thalassa.clusterIdentity=k8s-1",
		"--set", "thalassa.serviceAccountId=sa-1",
	)
	require.Contains(t, out, "app.kubernetes.io/component: controller")
	require.Contains(t, out, "app.kubernetes.io/component: webhook")
	require.Equal(t, 2, strings.Count(out, "\nkind: Deployment\n"))
	require.Contains(t, out, "--enable-controllers=true")
	require.Contains(t, out, "--enable-controllers=false")
}

func TestHelmTemplateNeitherFails(t *testing.T) {
	requireHelm(t)
	cmd := exec.Command("helm", "template", "twic-none", chartDir(t),
		"--namespace", "thalassa-system",
		"--set", "controller.enabled=false",
		"--set", "webhook.enabled=false",
	)
	out, err := cmd.CombinedOutput()
	require.Error(t, err)
	require.Contains(t, string(out), "at least one of controller.enabled or webhook.enabled")
}
