# Thalassa Workload Identity Controller

> **Experimental.** APIs (`v1beta1`), behaviour, and configuration may change without a stable compatibility guarantee. Use with caution in production.

Kubernetes controller that provisions Thalassa Cloud service accounts, federated identities, and IAM policy bindings. Making it easy to support workloads that can authenticate with projected JWTs via Workload Identity Federation without per-SA `tcloud` bootstrap runs.

> The controller does not create the cluster OIDC identity provider. On Thalassa-managed clusters that IdP already exists (label `kubernetes_cluster_id=<cluster identity>`), same as `tcloud iam workload-identity-federation bootstrap kubernetes --cluster …`. Non managed clusters need to create the Identity Provider through API.

## WorkloadIdentityBinding

```yaml
apiVersion: iam.thalassa.cloud/v1beta1
kind: WorkloadIdentityBinding
metadata:
  name: observability-proxy
  namespace: monitoring
spec:
  serviceAccountName: observability-proxy
  policyRef: observability:RemoteWriteAccess   # preferred
  # roleRef: observability:RemoteWriteAccess   # transitional
  # scopes: ["api:read", "api:write"]          # default: ["api:read"]
  # deleteResources: false
```

Prefer `policyRef` using the stable policy identity. `status.policyID` always stores the resolved identity (never slug/name) and is used for cleanup.

- With `thalassa.project` / `--project` set, `policyRef` resolves in that project.
- With project unset, `policyRef` resolves at the organisation root IAM scope.

Optional controller allowlists (`--allowed-policies` / `--allowed-roles`, Helm `controller.allowedPolicies` / `allowedRoles`): when set, refs must match by identity, slug, or name. Empty = allow any.

CRD CEL rejects missing `policyRef`/`roleRef` and wildcards at apply time.

Status: `phase`, `serviceAccountID`, `organisationID`, `projectID`, `federatedIdentityID`, `providerID`, `policyID`, `conditions`.

```bash
kubectl get wib -n monitoring
# NAME                  SA                    PHASE   POLICY   READY   REASON   AGE
```

JWT subject: `system:serviceaccount:<namespace>:<name>`.

### Pod identity files (token exchange)

OIDC token exchange needs `organisation_id` and `service_account_id` in addition to the projected Kubernetes JWT. When Ready, the controller syncs ConfigMap `wif-<serviceAccountName>` in the binding namespace:

| Key | File (example mount) | Required for exchange |
| --- | --- | --- |
| `organisation-id` | `/var/run/secrets/thalassa/organisation-id` | yes |
| `service-account-id` | `/var/run/secrets/thalassa/service-account-id` | yes |
| `project-id` | `/var/run/secrets/thalassa/project-id` | no (API project scope; only when `--project` is set) |

The same IDs are written as annotations on the target ServiceAccount. Mount the ConfigMap next to the projected token (see [examples/serviceaccount.yaml](examples/serviceaccount.yaml)).

### Kubernetes RBAC (who may bind)

Grant `create`/`update`/`delete` on `workloadidentitybindings` only to platform/GitOps identities. App teams can own ServiceAccounts without minting cloud IAM.

```yaml
apiGroups: ["iam.thalassa.cloud"]
resources: ["workloadidentitybindings"]
verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
```

## Required Thalassa permissions

The controller authenticates as a Thalassa service account (via WIF token exchange). See [API authorization](https://docs.thalassa.cloud/docs/iam/api-authorization/) and [Default IAM policies](https://docs.thalassa.cloud/docs/iam/iam-policies/default-policies/).

| API area | Scope | What the controller does |
| --- | --- | --- |
| Service accounts | Project / org | List, create, delete |
| Federated identities | Project / org | List, create, update, delete |
| Federated identity providers | Project / org | List (lookup cluster IdP; never create) |
| IAM policies & bindings | Project or org root | Get/list policies; list/create/delete bindings |
| Organisation roles & bindings | Organisation | Legacy `roleRef` path only |

Recommended: bind the controller SA to [`iam:FullAccess`](https://docs.thalassa.cloud/docs/iam/iam-policies/default-policies/) at the scope where it will manage policies (project and/or org root).

OIDC scopes on the controller federated identity: at least `api:read` and `api:write` (controller needs write to provision resources).

## Install

### 1. Bootstrap the controller’s own WIF (once)

```bash
tcloud iam workload-identity-federation bootstrap kubernetes \
  --cluster "$CLUSTER_ID" \
  --namespace thalassa-system \
  --service-account thalassa-workload-identity-controller \
  --role iam:FullAccess \
  --scope api:read,api:write
```

### 2. Helm

```bash
helm upgrade --install thalassa-workload-identity-controller \
  ./chart/thalassa-workload-identity-controller \
  --namespace thalassa-system --create-namespace \
  --set thalassa.organisation="$ORG_ID" \
  --set thalassa.project="$PROJECT_ID" \
  --set thalassa.clusterIdentity="$CLUSTER_ID" \
  --set thalassa.serviceAccountId="$CONTROLLER_THALASSA_SA_ID"
```


Optional hardening:

```bash
--set controller.allowedPolicies="{obs-write,pol-abc123}" \
--set controller.allowedRoles="{reader}"
```

### 3. Create a binding

See [examples/serviceaccount.yaml](examples/serviceaccount.yaml). After `status.phase=Ready`, mount the projected JWT (audience `https://api.thalassa.cloud`) and ConfigMap `wif-<sa-name>` for organisation/service-account IDs used in token exchange.

## Development

```bash
make generate   # deepcopy + CRDs (controller-gen)
make test
make lint
make build
```
