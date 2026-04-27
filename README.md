# helm-resources

Helm 4 CLI plugin that renders a chart and reports total declared CPU/memory
requests and limits across every workload — before anything is deployed.

## Install

```bash
helm plugin install https://github.com/gekart/helm-resources
```

Or from a local clone:

```bash
make build
helm plugin install .
```

## Usage

```bash
helm resources CHART [flags]
```

Examples:

```bash
# Per-subchart table
helm resources ./my-umbrella -f values-prod.yaml

# Grand total as JSON for CI
helm resources ./my-umbrella -f values-prod.yaml --group-by none -o json

# Consume pre-rendered manifests
helm template my-rel ./my-umbrella | helm resources --stdin

# Multiply DaemonSets by an explicit node count (overrides cluster lookup)
helm resources ./my-umbrella --nodes 10

# Skip the cluster query entirely; report DaemonSets per-node
helm resources ./my-umbrella --local-only
```

By default, `helm-resources` queries the cluster pointed to by your kubeconfig
to count nodes for DaemonSet totals. If the lookup fails (no kubeconfig, no
network, RBAC denied) it falls back to per-node mode and prints a warning to
stderr; pass `--local-only` to suppress the lookup, or `--nodes N` to override.

See `requirements.md` for the full spec.

## Develop

```bash
make build   # compile to bin/helm-resources
make test    # run unit + golden tests
make lint    # golangci-lint
```
