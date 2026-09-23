# thalassa-workload-identity-controller

Helm chart for the Thalassa Workload Identity Controller.

Install the [CRDs chart](../thalassa-workload-identity-controller-crds/) first.

## Topologies

| Mode | Values | What runs |
| --- | --- | --- |
| Controller only (default) | `values.yaml` | Reconciler Deployment |
| Self-managed in-cluster | [`values-incluster.yaml`](values-incluster.yaml) | Controller **and** webhook Deployments |
| Webhook only (managed K8s) | [`values-webhook-only.yaml`](values-webhook-only.yaml) | Webhook Deployment; controller runs in the Thalassa control plane |

Same image; components are independent (`--enable-controllers` / `--enable-pod-mutator`).

## Install

### CRDs + controller only

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

### Self-managed (controller + webhook)

```bash
helm upgrade --install thalassa-workload-identity-controller \
  ./chart/thalassa-workload-identity-controller \
  --namespace thalassa-system \
  -f chart/thalassa-workload-identity-controller/values-incluster.yaml \
  --set thalassa.organisation="$ORG_ID" \
  --set thalassa.clusterIdentity="$CLUSTER_ID" \
  --set thalassa.serviceAccountId="$CONTROLLER_THALASSA_SA_ID"
```

### Webhook only (managed cluster)

On Thalassa-managed Kubernetes the reconciler is operated in the control plane. Install CRDs (if not preinstalled) and the in-cluster webhook:

```bash
helm upgrade --install thalassa-workload-identity-webhook \
  ./chart/thalassa-workload-identity-controller \
  --namespace thalassa-system --create-namespace \
  -f chart/thalassa-workload-identity-controller/values-webhook-only.yaml
```

## Metrics

By default metrics are served on `:8443` over HTTPS with Kubernetes authn/authz (`metrics.secure=true`).

- Grant scrape access: set `metrics.prometheusServiceAccount.name` / `.namespace`
- Prometheus Operator: `--set enableServiceMonitor=true` (creates per-component Services/ServiceMonitors)

## Webhook HA

See [`values-webhook-ha.yaml`](values-webhook-ha.yaml) (overlay on webhook-enabled values):

```bash
helm upgrade --install ... \
  -f chart/thalassa-workload-identity-controller/values-webhook-only.yaml \
  -f chart/thalassa-workload-identity-controller/values-webhook-ha.yaml \
  ...
```

Sets `webhook.replicaCount: 2`, PDB, and `failurePolicy: Fail`.

## Values

| Key | Default | Notes |
| --- | --- | --- |
| `controller.enabled` | `true` | Reconciler Deployment |
| `webhook.enabled` | `false` | Mutator Deployment + Service + MWC |
| `webhook.replicaCount` | `1` | Use `2+` with HA / PDB |
| `webhook.minAvailable` | `1` | PDB when `webhook.replicaCount > 1` |
| `webhook.failurePolicy` | `Ignore` | Use `Fail` with HA |
| `metrics.enabled` | `true` | |
| `metrics.secure` | `true` | HTTPS + authn/authz |
| `enableServiceMonitor` | `false` | Needs Prometheus Operator CRDs |
| `controller.allowedPolicies` | `[]` | Empty = allow any (tighten for prod) |
