# `helm-resources` — Helm 4 CLI Plugin

A Helm 4 CLI plugin that renders a chart (including umbrella charts with subcharts) and reports the **total declared resource consumption** — CPU/memory requests and limits — across all workloads, before anything is deployed.

## Goal

Given a chart path (or packaged chart) and optional values files, produce a summary of total CPU and memory **requests** and **limits** that the chart will ask the cluster for once installed, with optional breakdowns by subchart, workload kind, and namespace.

This answers the question: _"If I `helm install` this umbrella chart, how much capacity does my cluster need?"_

## Background

Helm 4 (released November 2025) introduced a redesigned plugin system supporting CLI, getter, and post-renderer plugin types, with an optional WebAssembly runtime via Extism. This plugin targets the **CLI plugin** type.

## Scope

### In scope

- Read a chart from a local path, a packaged `.tgz`, or an OCI reference.
- Accept the same values-related flags `helm template` accepts: `-f/--values`, `--set`, `--set-string`, `--set-file`, `--set-json`.
- Render the chart (reuse Helm's rendering — do not reimplement templating).
- Walk all rendered manifests and sum container resources across these kinds:
  - `Pod`, `Deployment`, `StatefulSet`, `DaemonSet`, `ReplicaSet`, `Job`, `CronJob`
- For each workload, aggregate:
  - `spec.template.spec.containers[].resources`
  - `spec.template.spec.initContainers[].resources` (max of init containers, not sum — init containers run sequentially; regular containers run in parallel)
  - Multiply by `spec.replicas` (default 1 if unset)
  - For `CronJob`: use `spec.jobTemplate.spec.template.spec...`; flag as "scheduled" and report per-run totals separately from always-on totals.
  - For `DaemonSet`: report as "per node" since total depends on node count; accept an optional `--nodes N` flag to multiply.
- Normalize units:
  - CPU to millicores (integer)
  - Memory to bytes internally; display with binary prefixes (Ki, Mi, Gi)
- Support output formats: `table` (default), `json`, `yaml`, `csv`.
- Support grouping via `--group-by`:
  - `subchart` (from the `# Source: <chart>/templates/...` header Helm emits, or the `helm.sh/chart` label)
  - `kind` (Deployment, StatefulSet, etc.)
  - `namespace`
  - `none` (grand total only)
- Exit code `0` on success, non-zero on render failure.

### Out of scope

- Querying a live cluster for actual usage (use `kubectl-view-allocations` or `kube-capacity` for that).
- Cost estimation in currency (use `kubecost` or `opencost`).
- Node-fit / schedulability analysis.
- HPA-driven scaling projections beyond `spec.replicas` (see "Edge cases" for optional warning).
- Storage (PVC) totals in v1 — may add later.

## Plugin structure

Ship as a classic executable plugin for v1. Wasm port is a follow-up.

```
helm-resources/
├── plugin.yaml
├── bin/
│   └── helm-resources       # compiled binary
├── cmd/
│   └── main.go
├── internal/
│   ├── render/              # wrapper around helm template / SDK
│   ├── parse/               # YAML → workload model
│   ├── aggregate/           # sum logic, unit normalization
│   └── format/              # table/json/yaml/csv renderers
├── testdata/
│   └── charts/              # fixture umbrella charts
├── go.mod
└── README.md
```

`plugin.yaml` should declare the plugin name, version, usage, and command per Helm 4's plugin schema. Target Helm 4.0+; verify against the latest patch.

**Language:** Go, using `helm.sh/helm/v4` SDK for rendering so we don't shell out.

## CLI interface

```
helm resources CHART [flags]
```

**Flags**

| Flag                       | Description                                                                          |
| -------------------------- | ------------------------------------------------------------------------------------ |
| `-f, --values stringArray` | Values file(s), same semantics as `helm template`                                    |
| `--set stringArray`        | Inline value overrides                                                               |
| `--set-string stringArray` | Inline string overrides                                                              |
| `--set-file stringArray`   | Values from files                                                                    |
| `--set-json stringArray`   | JSON value overrides                                                                 |
| `-n, --namespace string`   | Release namespace (affects templating, not grouping)                                 |
| `--group-by string`        | `subchart` \| `kind` \| `namespace` \| `none` (default `subchart`)                   |
| `-o, --output string`      | `table` \| `json` \| `yaml` \| `csv` (default `table`)                               |
| `--nodes int`              | Node count for DaemonSet multiplication (default 1, reported as "per node" if unset) |
| `--include-init`           | Count init containers in totals (default `true`)                                     |
| `--warn-missing`           | Print warnings for containers with no `resources` block (default `true`)             |
| `--stdin`                  | Read already-rendered manifests from stdin instead of rendering                      |

**Example invocations**

```bash
# Default: total per subchart
helm resources ./my-umbrella -f values-prod.yaml

# Grand total as JSON for CI
helm resources ./my-umbrella -f values-prod.yaml --group-by none -o json

# Pipe pre-rendered manifests
helm template my-rel ./my-umbrella -f values-prod.yaml \
  | helm resources --stdin

# Factor in a 10-node cluster for DaemonSets
helm resources ./my-umbrella --nodes 10
```

## Computation logic

For each workload manifest:

1. Determine replica count (`spec.replicas`, default 1; DaemonSet → `--nodes` or 1 with "per node" flag; CronJob → 1 per concurrent run).
2. Compute per-pod resources:
   - `pod.requests.cpu = sum(containers[].resources.requests.cpu)`
   - `pod.requests.memory = sum(containers[].resources.requests.memory)`
   - Same for limits.
   - If `--include-init`: `pod.requests.cpu = max(pod.requests.cpu, max(initContainers[].resources.requests.cpu))` per Kubernetes' effective pod request rules (init max, containers sum, take the greater).
3. Multiply per-pod values by replica count.
4. Accumulate into the grouping buckets.

Unit parsing must handle: `100m`, `0.5`, `1`, `1.5` for CPU; `128Mi`, `1Gi`, `500M`, `2G`, `1024Ki`, raw byte integers for memory. Reject malformed values with a clear error.

## Output example (table, grouped by subchart)

```
SUBCHART        CPU REQ   CPU LIM   MEM REQ    MEM LIM    PODS
frontend         1500m     3000m    1.5Gi      3Gi        5
backend          2500m     5000m    2.5Gi      5Gi        5
worker           3000m    12000m    3Gi        12Gi       3
postgresql        500m     2000m    1Gi        4Gi        1
redis             250m     1000m    512Mi      2Gi        1
---
TOTAL            7750m    23000m    8.5Gi      26Gi       15

Warnings:
  - 2 containers have no resource requests declared (see --warn-missing for detail)
  - DaemonSet 'node-exporter' reported per-node; multiply by node count
```

## Edge cases & warnings

- **Containers with no `resources` block**: count as 0 but emit a warning listing the offending workload/container names. Critical for umbrella charts where subcharts often forget to set requests.
- **Limits without requests** (or vice versa): fine, Kubernetes defaults apply; report what's declared.
- **HPA present**: if an `HorizontalPodAutoscaler` targets a workload, emit an info note that `maxReplicas` could push totals higher. Do **not** automatically use `maxReplicas` — base totals on declared `replicas` only.
- **`replicas: 0`**: treat as 0, include in output with a note.
- **Pod-level resources** (Kubernetes 1.34+ `PodLevelResources` feature): read `spec.resources` at the pod level when present, and prefer it over container sums.
- **CRDs with embedded pod specs** (e.g., Argo `Rollout`, Knative `Service`): out of scope for v1; log "unknown kind, skipped".
- **Helm hooks** (`helm.sh/hook` annotated resources): exclude from totals by default; add `--include-hooks` flag if asked.

## Testing requirements

- Unit tests for unit parsing (`100m`, `1.5`, `1Gi`, `500M`, edge cases, malformed).
- Unit tests for per-pod aggregation (sum containers, max-of-init vs. sum-of-containers rule).
- Golden-file tests: render a fixture umbrella chart under `testdata/`, assert exact output for each `--output` format and each `--group-by`.
- The fixture chart should include: a Deployment with replicas, a DaemonSet, a StatefulSet, a CronJob, an init container, a container with no resources, and at least one subchart.
- Integration test: render against a real public umbrella chart (e.g., Bitnami `wordpress` or similar) and verify the command exits 0 and totals are non-zero.

## Deliverables

1. Working plugin installable via `helm plugin install <repo>`.
2. `README.md` with install instructions, usage examples, and known limitations.
3. `plugin.yaml` conforming to Helm 4's plugin schema.
4. CI workflow (GitHub Actions) running `go test ./...` and `golangci-lint` on push.
5. A `Makefile` with `build`, `test`, `install`, `lint` targets.

## Acceptance criteria

- [ ] `helm plugin install .` succeeds on Helm 4.x.
- [ ] `helm resources ./testdata/charts/umbrella -f testdata/values/prod.yaml` prints a grouped table matching the golden file.
- [ ] All four output formats produce valid, machine-parseable output (JSON/YAML parse cleanly; CSV has a header row).
- [ ] `--group-by none -o json` output contains exactly these top-level numeric fields: `cpuRequestsMilli`, `cpuLimitsMilli`, `memoryRequestsBytes`, `memoryLimitsBytes`, `pods`.
- [ ] Warnings are printed to stderr, results to stdout (pipe-safe).
- [ ] `helm resources --stdin < manifests.yaml` works without rendering.
- [ ] Unit test coverage ≥ 80% on `internal/aggregate` and `internal/parse`.

## Stretch goals (post-v1)

- Wasm/Extism build target for the sandboxed Helm 4 runtime, distributed via OCI.
- `--diff` flag comparing two values files to show delta totals.
- PVC storage totals.
- HPA-aware projected max totals.
- Grouping by arbitrary label selector.
