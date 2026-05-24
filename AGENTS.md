# external-dns-watcher — Agent Guide

This document provides guidance for AI coding agents (GitHub Copilot, Codex,
Claude, etc.) working in this repository.

---

## Git Workflow — MANDATORY

- **Never commit changes** unless explicitly instructed to do so.
- **Never create a branch** unless explicitly instructed to do so.
- **Never open a pull request** unless explicitly instructed to do so.
- Leave all changes as unstaged working-tree modifications by default.

---

## Project Overview

`external-dns-watcher` is a **Kubernetes controller** (operator) written in Go
using [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime)
(kubebuilder pattern). It watches a configurable set of arbitrary Kubernetes
resources and automatically generates
[`DNSEndpoint`](https://github.com/kubernetes-sigs/external-dns) resources for
[External DNS](https://kubernetes-sigs.github.io/external-dns/).

The primary use case is resources such as `HetznerCluster` (Cluster API) where
IP addresses are stored in `.status` fields and need to become DNS records
without manual intervention.

### How it works

1. A YAML config file defines one or more **watch rules** (`watches`).
2. For each rule, the controller watches the specified `group/version/kind`.
3. On reconcile, it evaluates **JSONPath expressions** against the resource to
   extract target values (IPs, hostnames).
4. It renders the DNS record name from a **Go template** (`recordTemplate`).
5. It creates or updates a `DNSEndpoint` resource (owned by the watched object)
   in the same namespace.

---

## Repository Layout

```text
cmd/
  main.go                  # Entrypoint — sets up controller-runtime manager

controllers/
  endpoint_reconciler.go   # Core reconciler: watches resources, manages DNSEndpoints

pkg/
  config/
    config.go              # Config structs (WatchConfig, PathConfig) and YAML loader

api/                       # Reserved for CRD API types (kubebuilder scaffold)

config/                    # Kustomize manifests (RBAC, deployment, CRD bases)
  crd/bases/
  rbac/
  manager/
  default/

chart/                     # Helm chart (generated via kubebuilder helm plugin)

test/
  e2e/                     # End-to-end tests using Ginkgo + Kind
  utils/                   # Shared test helpers

grafana/                   # Grafana dashboard definitions

dist/                      # Generated install.yaml (kustomize build output)

flake.nix                  # Nix dev shell definition
Makefile                   # All build, test, lint, and deploy targets
```

---

## Shell Environment

This repository uses a **Nix flake** (`flake.nix`) for the dev shell. With
[Direnv](https://direnv.net/) configured, the shell activates automatically.
Without Nix, ensure Go >= 1.24.0 and `make` are available on your `PATH`.

To activate manually:

```bash
nix develop
```

---

## Configuration Schema

The controller reads a YAML config file (default:
`/etc/external-dns-watcher/config.yaml`) with the following structure:

```yaml
watches:
  - group: infrastructure.cluster.x-k8s.io
    version: v1beta1
    kind: HetznerCluster
    namespace: ""                    # optional — restricts to one namespace
    recordTemplate: "{{ .Name }}-control-plane.example.com"
    paths:
      - path: "$.status.controlPlaneLoadBalancer.ipv4"
        type: A
      - path: "$.status.controlPlaneLoadBalancer.ipv6"
        type: AAAA
```

- `recordTemplate` is a Go template with `.Name` and `.Namespace` available.
- `path` is a [JSONPath](https://goessner.net/articles/JsonPath/) expression
  evaluated against the unstructured resource object.
- `type` is the DNS record type (`A`, `AAAA`, `CNAME`, etc.).

---

## Key Types

| Type | File | Description |
|------|------|-------------|
| `Config` | `pkg/config/config.go` | Root config with `Watches []WatchConfig` |
| `WatchConfig` | `pkg/config/config.go` | One watch rule (GVK + template + paths) |
| `PathConfig` | `pkg/config/config.go` | One JSONPath → record type mapping |
| `EndpointReconciler` | `controllers/endpoint_reconciler.go` | controller-runtime reconciler |

---

## Build & Development Commands

All commands are defined in `Makefile`. Key targets:

| Target | Description |
|--------|-------------|
| `make build` | Build the `bin/manager` binary |
| `make run` | Run the controller locally (requires kubeconfig) |
| `make test` | Run unit/integration tests via envtest |
| `make test-e2e` | Run e2e tests against a temporary Kind cluster |
| `make lint` | Run `golangci-lint` |
| `make lint-fix` | Run linter with auto-fix |
| `make manifests` | Regenerate RBAC/CRD manifests via `controller-gen` |
| `make generate` | Regenerate DeepCopy methods via `controller-gen` |
| `make fmt` | Run `go fmt ./...` |
| `make vet` | Run `go vet ./...` |
| `make docker-build` | Build the container image |
| `make build-installer` | Generate `dist/install.yaml` via kustomize |

All tooling (`controller-gen`, `kustomize`, `golangci-lint`, `setup-envtest`)
is downloaded locally into `bin/` by the Makefile — no manual installation
needed.

---

## Testing

### Unit / integration tests

Uses [Ginkgo](https://onsi.github.io/ginkgo/) +
[Gomega](https://onsi.github.io/gomega/) with `controller-runtime`'s
`envtest` to run a real API server without a full cluster.

```bash
make test
```

### End-to-end tests

Uses a [Kind](https://kind.sigs.k8s.io/) cluster spun up automatically.

```bash
make test-e2e
```

The Kind cluster is named `external-dns-watcher-test-e2e` and is torn down
after the test run (`make cleanup-test-e2e`).

---

## CI Workflows

| Workflow | File | Purpose |
|----------|------|---------|
| General | `.github/workflows/general.yml` | Build, test, lint |
| Docker | `.github/workflows/docker.yml` | Build and push container image |
| Release | `.github/workflows/release.yml` | Semantic release and changelog |
| Helm docs | `.github/workflows/helmdocs.yml` | Regenerate chart documentation |
| Automerge | `.github/workflows/automerge.yml` | Renovate/Dependabot automation |
| Flake | `.github/workflows/flake.yml` | Scheduled Nix flake lock updates |

---

## Deployment

The preferred installation method is the Helm chart:

```bash
helm install external-dns-watcher \
  oci://ghcr.io/kubehippie/charts/external-dns-watcher \
  --values values.yaml
```

Raw Kustomize manifests can be generated with:

```bash
make build-installer   # outputs dist/install.yaml
```

---

## RBAC Considerations

The controller needs RBAC permissions for:
- `externaldns.k8s.io/dnsendpoints` — get, list, watch, create, update, patch, delete
- Each watched resource type — get, list, watch (must be added via `rbac.extraRules` in the Helm chart)

When adding a new watch rule, always add the corresponding RBAC entry.

---

## Contribution Conventions

- Use pull requests for all changes.
- Run `make fmt vet lint` before pushing.
- After changing controller logic, regenerate manifests with `make manifests generate`.
- Keep the Helm chart in sync with config schema changes.
- For security issues, contact `thomas@webhippie.de`.
