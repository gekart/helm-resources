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

# Multiply DaemonSets by node count
helm resources ./my-umbrella --nodes 10
```

See `requirements.md` for the full spec.

## Develop

```bash
make build   # compile to bin/helm-resources
make test    # run unit + golden tests
make lint    # golangci-lint
```
