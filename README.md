# Thalassa Workload Identity Controller

> **Experimental** (`v1beta1`). APIs and behaviour may change.

Provisions Thalassa Cloud IAM (service account, federated identity, policy binding) for a Kubernetes ServiceAccount so pods can exchange a projected JWT for a Thalassa bearer token via [Workload Identity Federation](https://docs.thalassa.cloud/docs/iam/oidc/).

**Does not** create the cluster OIDC identity provider. On Thalassa-managed clusters that IdP already exists (`kubernetes_cluster_id=<cluster identity>`). For other clusters, create the IdP via the API first.

## Prerequisites

| Item | Notes |
| --- | --- |
| Kubernetes cluster with OIDC IdP registered in Thalassa | Same cluster identity you use with `tcloud` |
| `tcloud` CLI | Bootstrap the controller’s own WIF once |
| Helm 3 | Chart under `chart/thalassa-workload-identity-controller` |
| cert-manager (optional) | Only if you enable the pod mutator webhook with default TLS |

Collect these IDs before install:

```bash
ORG_ID=...           # Thalassa organisation
CLUSTER_ID=...       # Thalassa Kubernetes cluster identity
PROJECT_ID=...       # optional; empty = organisation-root IAM for policyRef
```

## Install

### 1. Bootstrap the controller service account (once)

```bash
tcloud iam workload-identity-federation bootstrap kubernetes \
  --cluster "$CLUSTER_ID" \
  --namespace thalassa-system \
  --service-account thalassa-workload-identity-controller \
  --role iam:FullAccess \
  --scope api:read,api:write
```

Save the printed Thalassa service account ID as `CONTROLLER_THALASSA_SA_ID`.

### 2. Install the chart

```bash
helm upgrade --install thalassa-workload-identity-controller \
  ./chart/thalassa-workload-identity-controller \
  --namespace thalassa-system --create-namespace \
  --set thalassa.organisation="$ORG_ID" \
  --set thalassa.clusterIdentity="$CLUSTER_ID" \
  --set thalassa.serviceAccountId="$CONTROLLER_THALASSA_SA_ID" \
  --set thalassa.project="$PROJECT_ID"   # omit or empty for org-root policies
```

| Helm value | Required | Purpose |
| --- | --- | --- |
| `thalassa.organisation` | yes | Organisation for API calls / token exchange |
| `thalassa.clusterIdentity` | yes | Lookup cluster OIDC IdP |
| `thalassa.serviceAccountId` | yes | Controller’s Thalassa SA (from bootstrap) |
| `thalassa.project` | no | Scope for `policyRef` (`X-Project-Identity`); empty = org root |
| `webhook.enabled` | no | Inject env + projected token into labeled pods (needs cert-manager by default) |
| `controller.enableIdentityConfigMap` | no | Sync `wif-<sa>` ConfigMap for **all** Ready bindings |
| `controller.allowedPolicies` / `allowedRoles` | no | Allowlists (empty = allow any) |
| `rbac.watchNamespaces` | no | Limit watch scope; empty = all namespaces |

Enable the recommended pod webhook:

```bash
--set webhook.enabled=true
```

Without cert-manager, provide a TLS secret (`tls.crt` / `tls.key`) and set:

```bash
--set webhook.certManager.enabled=false \
--set webhook.tls.secretName=my-webhook-certs
```

### 3. Verify the controller

```bash
kubectl -n thalassa-system rollout status deploy/thalassa-workload-identity-controller
kubectl -n thalassa-system logs deploy/thalassa-workload-identity-controller -c manager -f
```

## Day-2: bind a workload

Desired fields are at the **root** of the CR (like `RoleBinding`), not under `spec`.

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: observability-proxy
  namespace: monitoring
---
apiVersion: iam.thalassa.cloud/v1beta1
kind: WorkloadIdentityBinding
metadata:
  name: observability-proxy
  namespace: monitoring
serviceAccountName: observability-proxy
policyRef: observability:RemoteWriteAccess   # prefer stable policy identity
# roleRef: ...                                # transitional alternative
# scopes: ["openid"]                          # default if omitted
# identityConfigMap: true                     # optional per-binding ConfigMap
# deleteResources: true                       # delete Thalassa resources on CR delete
```

Full example (including pod label): [examples/serviceaccount.yaml](examples/serviceaccount.yaml).

### Check status

```bash
kubectl get wib -n monitoring
kubectl describe wib observability-proxy -n monitoring
```

Ready when `status.phase=Ready` and condition `Ready=True`. Useful status fields: `serviceAccountID`, `organisationID`, `projectID`, `policyID`, `federatedIdentityID`.

JWT subject used for federation: `system:serviceaccount:<namespace>:<name>`.

Projected token audience for exchange: `https://api.thalassa.cloud` (override via `thalassa.projectedToken.audience` / trusted audiences).

## How pods get credentials

Token exchange needs **organisation ID + Thalassa service account ID + projected JWT**. When Ready, the controller always annotates the Kubernetes ServiceAccount:

| Annotation | When |
| --- | --- |
| `thalassa.cloud/wif.organisation-id` | always |
| `thalassa.cloud/wif.service-account-id` | always |
| `thalassa.cloud/wif.project-id` | only if controller `--project` / `thalassa.project` is set |

Pick one consumption path:

### A. Pod mutator webhook (recommended)

1. Helm: `webhook.enabled=true`
2. Label the pod: `thalassa.cloud/wif.use: "true"` (label, not annotation — required for webhook `objectSelector`)
3. Webhook injects env + volume when the SA annotations are present:

| Env | Value |
| --- | --- |
| `THALASSA_ORGANISATION_ID` | from SA annotation |
| `THALASSA_SERVICE_ACCOUNT_ID` | from SA annotation |
| `THALASSA_PROJECT_ID` | if annotated |
| `THALASSA_SUBJECT_TOKEN_FILE` | `/var/run/secrets/thalassa/token` |

Default webhook `failurePolicy` is `Ignore` (scheduling continues if the webhook is down). Set `webhook.failurePolicy=Fail` for stricter clusters.

### B. Identity ConfigMap

Set `identityConfigMap: true` on the binding (and/or `controller.enableIdentityConfigMap=true`). Creates ConfigMap `wif-<serviceAccountName>`:

| Key | Required for exchange |
| --- | --- |
| `organisation-id` | yes |
| `service-account-id` | yes |
| `project-id` | no |

Mount it next to a projected SA token (see comments in the example). Turning the flag off does not delete existing ConfigMaps; deleting the binding still GC’s owner-referenced ones.

### C. Explicit config

Copy IDs from `kubectl get wib … -o yaml` / SA annotations into your app Helm values; mount only the projected JWT.

## RBAC (who may create bindings)

Limit `workloadidentitybindings` write access to platform / GitOps identities. App teams can own ServiceAccounts without cloud IAM rights.

```yaml
apiGroups: ["iam.thalassa.cloud"]
resources: ["workloadidentitybindings"]
verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
```

CEL on the CR rejects missing `policyRef`/`roleRef` and wildcards (`*`) at apply time. Optional controller allowlists further restrict which policies/roles may be referenced.

## Controller Thalassa permissions

The controller calls the Thalassa API as its bootstrapped SA. Recommended policy: [`iam:FullAccess`](https://docs.thalassa.cloud/docs/iam/iam-policies/default-policies/) at the project and/or org-root scope where it manages bindings. Docs: [API authorization](https://docs.thalassa.cloud/docs/iam/api-authorization/).

| API area | Access needed |
| --- | --- |
| Service accounts | list, create, delete |
| Federated identities | list, create, update, delete |
| Federated identity providers | list only (never create) |
| IAM policies & bindings | get/list policies; list/create/delete bindings |
| Organisation roles & bindings | only if using `roleRef` |

Controller federated identity scopes: at least `api:read` and `api:write`.

## Troubleshooting

| Symptom | Check |
| --- | --- |
| Binding stuck `Pending` / Ready=False `MissingIDP` | Cluster IdP exists and `thalassa.clusterIdentity` matches `kubernetes_cluster_id` |
| `EnsureFailed` / policy errors | `policyRef` exists at project or org root (matches whether `thalassa.project` is set); allowlists |
| Webhook pods have no env | Label `thalassa.cloud/wif.use=true`; binding Ready; SA has `wif.organisation-id` + `wif.service-account-id`; cert-manager Certificate Ready |
| Pod cannot exchange token | Audience `https://api.thalassa.cloud`; subject is `system:serviceaccount:ns:name`; org + SA IDs match binding status |

```bash
kubectl get wib -A
kubectl describe wib <name> -n <ns>
kubectl get sa <name> -n <ns> -o yaml   # look for thalassa.cloud/wif.*
kubectl get mutatingwebhookconfiguration | grep wif
```

## Development

```bash
make generate   # deepcopy + CRDs
make test
make lint
make build
```
