# thalassa-workload-identity-controller

Helm chart for the Thalassa Workload Identity Controller.

Install the [CRDs chart](../thalassa-workload-identity-controller-crds/) first.

## Install

```bash
helm upgrade --install thalassa-workload-identity-controller-crds \
  ./chart/thalassa-workload-identity-controller-crds \
  --namespace thalassa-system --create-namespace

helm upgrade --install thalassa-workload-identity-controller \
  ./chart/thalassa-workload-identity-controller \
  --namespace thalassa-system \
  --set thalassa.organisation="$ORG_ID" \
  --set thalassa.clusterIdentity="$CLUSTER_ID" \
  --set thalassa.serviceAccountId="$CONTROLLER_THALASSA_SA_ID"
```

## Metrics

By default metrics are served on `:8443` over HTTPS with Kubernetes authn/authz (`metrics.secure=true`).

- Grant scrape access: set `metrics.prometheusServiceAccount.name` / `.namespace`
- Prometheus Operator: `--set enableServiceMonitor=true`

## Webhook HA

See [`values-webhook-ha.yaml`](values-webhook-ha.yaml):

```bash
helm upgrade --install ... \
  -f chart/thalassa-workload-identity-controller/values-webhook-ha.yaml \
  ...
```

That sets `replicaCount: 2`, creates a PDB, enables the mutator, and uses `failurePolicy: Fail`. Requires `controller.leaderElect: true` (enforced by the chart).

## Values

| Key | Default | Notes |
| --- | --- | --- |
| `replicaCount` | `1` | Use `2+` with webhook HA |
| `minAvailable` | `1` | PDB when `replicaCount > 1` |
| `metrics.enabled` | `true` | |
| `metrics.secure` | `true` | HTTPS + authn/authz |
| `enableServiceMonitor` | `false` | Needs Prometheus Operator CRDs |
| `webhook.enabled` | `false` | Opt-in pod mutator |
| `webhook.failurePolicy` | `Ignore` | Use `Fail` with HA |
| `controller.allowedPolicies` | `[]` | Empty = allow any (tighten for prod) |
