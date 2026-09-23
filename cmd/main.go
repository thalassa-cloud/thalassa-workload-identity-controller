package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	iamv1beta1 "github.com/thalassa-cloud/thalassa-workload-identity-controller/api/v1beta1"
	"github.com/thalassa-cloud/thalassa-workload-identity-controller/internal/config"
	"github.com/thalassa-cloud/thalassa-workload-identity-controller/internal/controller"
	ithalassa "github.com/thalassa-cloud/thalassa-workload-identity-controller/internal/thalassa"
	"github.com/thalassa-cloud/thalassa-workload-identity-controller/internal/webhook/podmutator"
)

var scheme = runtime.NewScheme()

const (
	defaultWebhookCertDir = "/tmp/k8s-webhook-server/serving-certs"
	defaultWebhookPort    = 9443
	podMutatorPath        = "/mutate-v1-pod"
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(iamv1beta1.AddToScheme(scheme))
}

func main() {
	var (
		metricsAddr                     string
		probeAddr                       string
		secureMetrics                   bool
		enableLeaderElection            bool
		thalassaURL                     string
		organisation                    string
		projectIdentity                 string
		clusterIdentity                 string
		serviceAccountID                string
		subjectTokenFile                string
		trustedAudiencesRaw             string
		watchNamespacesRaw              string
		requeueMissingIDP               time.Duration
		enableServiceAccountAnnotations bool
		enableIdentityConfigMap         bool
		enablePodMutator                bool
		webhookCertDir                  string
		webhookPort                     int
		allowedPoliciesRaw              string
		allowedRolesRaw                 string
	)

	flag.StringVar(&metricsAddr, "metrics-bind-address", config.DefaultMetricsAddr,
		"The address the metric endpoint binds to. Use :8443 for HTTPS or :8080 for HTTP, or 0 to disable.")
	flag.BoolVar(&secureMetrics, "metrics-secure", true,
		"Serve metrics over HTTPS with Kubernetes authn/authz. Set false for plain HTTP.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", config.DefaultProbeAddr,
		"The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", true,
		"Enable leader election for controller manager.")
	flag.StringVar(&thalassaURL, "thalassa-url", config.DefaultThalassaURL,
		"Thalassa Cloud API base URL")
	flag.StringVar(&organisation, "organisation", "",
		"Thalassa organisation identity")
	flag.StringVar(&projectIdentity, "project", "",
		"Thalassa project identity for IAM policy APIs (empty = organisation root)")
	flag.StringVar(&clusterIdentity, "cluster-identity", "",
		"Thalassa Kubernetes cluster identity (for OIDC IdP lookup)")
	flag.StringVar(&serviceAccountID, "thalassa-service-account-id", "",
		"Thalassa SA used by the controller for token exchange")
	flag.StringVar(&subjectTokenFile, "thalassa-subject-token-file",
		ithalassa.DefaultSubjectTokenFile, "Path to projected Kubernetes SA JWT")
	flag.StringVar(&trustedAudiencesRaw, "trusted-audiences", "",
		"Comma-separated JWT audiences (default: thalassa-url)")
	flag.StringVar(&watchNamespacesRaw, "watch-namespaces", "",
		"Comma-separated namespaces to watch (empty = all)")
	flag.DurationVar(&requeueMissingIDP, "requeue-missing-idp",
		config.DefaultRequeueMissingIDP, "Requeue when cluster IdP is not ready")
	flag.BoolVar(&enableServiceAccountAnnotations, "enable-serviceaccount-annotations", false,
		"Enable legacy ServiceAccount annotation reconciliation (default: disabled)")
	flag.BoolVar(&enableIdentityConfigMap, "enable-identity-configmap", false,
		"Sync wif-<sa> ConfigMap with exchange IDs when a binding is Ready (default: disabled)")
	flag.BoolVar(&enablePodMutator, "enable-pod-mutator", false,
		"Enable mutating webhook that injects WIF env and projected token (default: disabled)")
	flag.StringVar(&webhookCertDir, "webhook-cert-dir", defaultWebhookCertDir,
		"Directory containing tls.crt and tls.key for the pod mutator webhook")
	flag.IntVar(&webhookPort, "webhook-port", defaultWebhookPort,
		"Port for the pod mutator webhook server")
	flag.StringVar(&allowedPoliciesRaw, "allowed-policies", "",
		"Comma-separated allowlist of IAM policy identities/slugs/names (empty = allow any)")
	flag.StringVar(&allowedRolesRaw, "allowed-roles", "",
		"Comma-separated allowlist of organisation role identities/slugs/names (empty = allow any)")

	opts := zap.Options{Development: false}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	setupLog := ctrl.Log.WithName("setup")

	cfg := config.Config{
		ThalassaURL:                     config.NormalizeURL(thalassaURL),
		OrganisationID:                  strings.TrimSpace(organisation),
		ProjectIdentity:                 strings.TrimSpace(projectIdentity),
		ClusterIdentity:                 strings.TrimSpace(clusterIdentity),
		ControllerServiceAccount:        strings.TrimSpace(serviceAccountID),
		SubjectTokenFile:                strings.TrimSpace(subjectTokenFile),
		TrustedAudiences:                splitCSV(trustedAudiencesRaw),
		WatchNamespaces:                 splitCSV(watchNamespacesRaw),
		ProbeAddr:                       probeAddr,
		MetricsAddr:                     metricsAddr,
		EnableLeaderElection:            enableLeaderElection,
		RequeueMissingIDP:               requeueMissingIDP,
		EnableServiceAccountAnnotations: enableServiceAccountAnnotations,
		EnableIdentityConfigMap:         enableIdentityConfigMap,
		EnablePodMutator:                enablePodMutator,
		WebhookCertDir:                  strings.TrimSpace(webhookCertDir),
		WebhookPort:                     webhookPort,
		AllowedPolicies:                 splitCSV(allowedPoliciesRaw),
		AllowedRoles:                    splitCSV(allowedRolesRaw),
	}
	if len(cfg.TrustedAudiences) == 0 {
		cfg.TrustedAudiences = []string{cfg.ThalassaURL}
	}
	if err := cfg.Validate(); err != nil {
		setupLog.Error(err, "invalid configuration")
		os.Exit(1)
	}

	tc, err := ithalassa.NewClient(cfg)
	if err != nil {
		setupLog.Error(err, "unable to create Thalassa client")
		os.Exit(1)
	}

	metricsServerOptions := metricsserver.Options{
		BindAddress:   metricsAddr,
		SecureServing: secureMetrics,
	}
	if secureMetrics {
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	mgrOpts := ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsServerOptions,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "thalassa-workload-identity-controller",
	}
	if cfg.EnablePodMutator {
		mgrOpts.WebhookServer = webhook.NewServer(webhook.Options{
			Port:    cfg.WebhookPort,
			CertDir: cfg.WebhookCertDir,
		})
	}
	if len(cfg.WatchNamespaces) > 0 {
		nsCache := map[string]cache.Config{}
		for _, ns := range cfg.WatchNamespaces {
			nsCache[ns] = cache.Config{}
		}
		mgrOpts.Cache = cache.Options{DefaultNamespaces: nsCache}
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), mgrOpts)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err := (&controller.WorkloadIdentityBindingReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		IAM:    tc.IAM(),
		Config: cfg,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "WorkloadIdentityBinding")
		os.Exit(1)
	}

	if cfg.EnableServiceAccountAnnotations {
		setupLog.Info("ServiceAccount annotation reconciliation enabled")
		if err := (&controller.ServiceAccountReconciler{
			Client: mgr.GetClient(),
			Scheme: mgr.GetScheme(),
			IAM:    tc.IAM(),
			Config: cfg,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "ServiceAccount")
			os.Exit(1)
		}
	}

	if cfg.EnablePodMutator {
		audience := cfg.TrustedAudiences[0]
		setupLog.Info("pod mutator webhook enabled", "path", podMutatorPath, "audience", audience)
		mgr.GetWebhookServer().Register(podMutatorPath, &admission.Webhook{
			Handler: &podmutator.Handler{
				Client:   mgr.GetClient(),
				Decoder:  admission.NewDecoder(mgr.GetScheme()),
				Audience: audience,
			},
		})
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}
	if cfg.EnablePodMutator {
		if err := mgr.AddReadyzCheck("webhook-cert", webhookCertReady(cfg.WebhookCertDir)); err != nil {
			setupLog.Error(err, "unable to set up webhook cert ready check")
			os.Exit(1)
		}
	}

	setupLog.Info("starting manager",
		"organisation", cfg.OrganisationID,
		"cluster", cfg.ClusterIdentity,
		"serviceAccountAnnotations", cfg.EnableServiceAccountAnnotations,
		"identityConfigMap", cfg.EnableIdentityConfigMap,
		"podMutator", cfg.EnablePodMutator,
		"metricsSecure", secureMetrics,
	)
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func webhookCertReady(certDir string) healthz.Checker {
	return func(_ *http.Request) error {
		certPath := filepath.Join(certDir, "tls.crt")
		keyPath := filepath.Join(certDir, "tls.key")
		if _, err := os.Stat(certPath); err != nil {
			return fmt.Errorf("webhook tls.crt not ready: %w", err)
		}
		if _, err := os.Stat(keyPath); err != nil {
			return fmt.Errorf("webhook tls.key not ready: %w", err)
		}
		return nil
	}
}

func splitCSV(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
