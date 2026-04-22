# CLAUDE.md

Helm 4 CLI plugin that sums declared CPU/memory requests and limits across a rendered chart. See `requirements.md` for the full spec.

## Commands

```bash
make build         # compile to bin/helm-resources
make test          # go test ./...
make lint          # golangci-lint run
make install       # helm plugin install .
```

Run a single test: `go test ./internal/aggregate -run TestName -v`

## Architecture

- `cmd/` — CLI entrypoint and flag parsing
- `internal/render/` — wraps the `helm.sh/helm/v4` SDK to produce manifests
- `internal/parse/` — YAML → workload model (Deployment, StatefulSet, DaemonSet, Job, CronJob, Pod, ReplicaSet)
- `internal/aggregate/` — sum logic, unit normalization, grouping
- `internal/format/` — table / json / yaml / csv renderers

Data flow: `render → parse → aggregate → format`. Keep these boundaries clean; no format-specific logic in `aggregate`.

## Conventions

- CPU is millicores (int64) internally; memory is bytes (int64). Convert at the edges only.
- Init container aggregation uses Kubernetes' effective-request rule: `max(sum(containers), max(initContainers))` per pod — not a naive sum.
- DaemonSet totals are "per node" unless `--nodes N` is passed. Never assume a node count.
- HPA `maxReplicas` is informational only; totals use `spec.replicas`.
- Warnings go to stderr, results to stdout. The tool must be pipe-safe.
- Unknown kinds (CRDs with embedded pod specs, etc.) are skipped with a stderr note, never a hard error.

## Testing

- Golden-file tests in `testdata/` drive the output-format and grouping assertions. Update goldens with `go test ./... -update`.
- Fixture umbrella chart under `testdata/charts/umbrella` must cover: replicas, DaemonSet, StatefulSet, CronJob, init container, container with no `resources`, and at least one subchart.
- Unit test coverage target: ≥80% on `internal/aggregate` and `internal/parse`.

## Gotchas

- Don't shell out to `helm template` — use the SDK. Shelling out breaks the Wasm port path.
- `helm template` emits `# Source: <chart>/...` headers; use those (or the `helm.sh/chart` label) for subchart grouping. Don't rely on file paths.
- `CronJob` pod spec is at `spec.jobTemplate.spec.template.spec`, not `spec.template.spec`.
- Pod-level resources (K8s 1.34+ `PodLevelResources`): if `spec.resources` exists at the pod level, prefer it over summing containers.
- Helm hooks (annotated `helm.sh/hook`) are excluded from totals by default.
