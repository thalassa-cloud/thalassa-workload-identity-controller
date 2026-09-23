# Flux CD (Helm) example

Example manifests to install **thalassa-workload-identity-controller** and **thalassa-workload-identity-controller-crds** from published OCI charts using Flux `HelmRepository` + `HelmRelease`.

## Layout

| File | Purpose |
|------|---------|
| [`helmrepository.yaml`](helmrepository.yaml) | OCI source `oci://ghcr.io/thalassa-cloud/charts` |
| [`helmrelease-controller-crds.yaml`](helmrelease-controller-crds.yaml) | CRDs chart (install first) |
| [`helmrelease-controller.yaml`](helmrelease-controller.yaml) | Controller; `dependsOn` the CRDs release |

Both releases use `targetNamespace: thalassa-system` and `install.createNamespace: true`.

## Prerequisites

- Flux installed (source-controller + helm-controller)
- Controller WIF bootstrapped; see the [root README](../../README.md)

## Apply

```bash
kubectl apply -k deploy/flux/
```

## Values

Replace placeholder `organisation`, `clusterIdentity`, and `serviceAccountId` in [`helmrelease-controller.yaml`](helmrelease-controller.yaml). For a HA webhook, merge settings from [`values-webhook-ha.yaml`](../../chart/thalassa-workload-identity-controller/values-webhook-ha.yaml).
