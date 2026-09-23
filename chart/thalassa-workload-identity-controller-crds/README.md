# thalassa-workload-identity-controller-crds

![Version: 0.1.0](https://img.shields.io/badge/Version-0.1.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.1.0](https://img.shields.io/badge/AppVersion-0.1.0-informational?style=flat-square)

Helm chart for deploying the Thalassa Workload Identity Controller CRDs.

Install **before** the controller chart. CRDs live in `templates/` so Helm can upgrade them with the release (same pattern as `thalassa-dbaas-manager-crds`).

## Source Code

* <https://github.com/thalassa-cloud/thalassa-workload-identity-controller>

## Install

```bash
helm upgrade --install thalassa-workload-identity-controller-crds \
  ./chart/thalassa-workload-identity-controller-crds \
  --namespace thalassa-system --create-namespace
```

Published OCI (after release):

```bash
helm upgrade --install thalassa-workload-identity-controller-crds \
  oci://ghcr.io/thalassa-cloud/charts/thalassa-workload-identity-controller-crds:<version> \
  --namespace thalassa-system --create-namespace
```
