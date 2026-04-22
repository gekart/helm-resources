// Package parse converts rendered Kubernetes manifests (multi-document YAML)
// into a flat slice of Workload records that aggregation can consume.
package parse

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"

	"sigs.k8s.io/yaml"
)

// ResourceValues mirrors a Kubernetes ResourceRequirements CPU/memory entry.
type ResourceValues struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}

// Container is one container or init container's resources.
type Container struct {
	Name     string
	Requests *ResourceValues
	Limits   *ResourceValues
}

// Workload is a normalized view of a rendered Kubernetes workload.
type Workload struct {
	Kind      string
	Name      string
	Namespace string
	Chart     string // best-effort subchart name, from `# Source:` header or label

	Replicas *int64

	Containers     []Container
	InitContainers []Container

	// Pod-level resources (PodLevelResources, K8s 1.34+). When non-nil these
	// take precedence over container sums.
	PodRequests *ResourceValues
	PodLimits   *ResourceValues

	IsHook bool // helm.sh/hook annotation present

	// HPAInfo is set on workloads that are targeted by an HPA (post-processing).
	HPAMaxReplicas *int64
}

// ParseOptions controls parsing behavior.
type ParseOptions struct {
	// IncludeHooks is informational only — hook workloads are always parsed;
	// aggregate is responsible for filtering.
}

var sourceHeaderRE = regexp.MustCompile(`^#\s*Source:\s*([^/\s]+)/`)

// Parse reads multi-document YAML from r and returns parsed workloads plus any
// unknown-kind notes emitted to the caller for stderr reporting.
func Parse(r io.Reader, _ ParseOptions) ([]Workload, []string, error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, fmt.Errorf("read manifests: %w", err)
	}
	docs, err := splitDocs(buf)
	if err != nil {
		return nil, nil, err
	}

	var workloads []Workload
	var notes []string

	// First pass: collect workloads + HPAs.
	type hpaRef struct {
		targetKind  string
		targetName  string
		namespace   string
		maxReplicas *int64
	}
	var hpas []hpaRef

	for _, d := range docs {
		if len(bytes.TrimSpace(d.body)) == 0 {
			continue
		}
		var meta genericDoc
		if err := yaml.Unmarshal(d.body, &meta); err != nil {
			return nil, nil, fmt.Errorf("unmarshal doc: %w", err)
		}
		if meta.Kind == "" {
			continue
		}

		switch meta.Kind {
		case "Pod", "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet", "Job", "CronJob":
			w, err := parseWorkload(d.body, meta, d.chartHint)
			if err != nil {
				return nil, nil, fmt.Errorf("%s/%s: %w", meta.Kind, meta.Metadata.Name, err)
			}
			workloads = append(workloads, w)
		case "HorizontalPodAutoscaler":
			ref, ok := parseHPA(d.body)
			if ok {
				hpas = append(hpas, hpaRef{
					targetKind:  ref.spec.ScaleTargetRef.Kind,
					targetName:  ref.spec.ScaleTargetRef.Name,
					namespace:   ref.metadata.Namespace,
					maxReplicas: ref.spec.MaxReplicas,
				})
			}
		default:
			// Skipped kinds (Services, ConfigMaps, CRDs, etc.) — silent unless
			// it's something we might plausibly be expected to understand.
			if isMaybePodCarrier(meta.Kind) {
				notes = append(notes, fmt.Sprintf("unknown kind %q (skipped)", meta.Kind))
			}
		}
	}

	// Second pass: attach HPA info.
	for i := range workloads {
		w := &workloads[i]
		for _, h := range hpas {
			if h.targetKind == w.Kind && h.targetName == w.Name && h.namespace == w.Namespace {
				w.HPAMaxReplicas = h.maxReplicas
				break
			}
		}
	}
	return workloads, notes, nil
}

// --- helpers -----------------------------------------------------------------

type rawDoc struct {
	body      []byte
	chartHint string // subchart name parsed from a `# Source: <chart>/...` header
}

// splitDocs splits a YAML stream on `^---` boundaries while recording the last
// `# Source: <chart>/...` header as a per-doc hint.
func splitDocs(buf []byte) ([]rawDoc, error) {
	var docs []rawDoc
	var current bytes.Buffer
	chartHint := ""
	lastHint := ""

	scanner := bufio.NewScanner(bytes.NewReader(buf))
	scanner.Buffer(make([]byte, 1024*1024), 32*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "---") {
			docs = append(docs, rawDoc{body: append([]byte(nil), current.Bytes()...), chartHint: chartHint})
			current.Reset()
			chartHint = lastHint
			continue
		}
		if m := sourceHeaderRE.FindStringSubmatch(line); m != nil {
			lastHint = m[1]
			if chartHint == "" {
				chartHint = m[1]
			}
		}
		current.WriteString(line)
		current.WriteByte('\n')
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if current.Len() > 0 {
		docs = append(docs, rawDoc{body: current.Bytes(), chartHint: chartHint})
	}
	return docs, nil
}

// isMaybePodCarrier returns true for kinds that commonly embed a pod spec; used
// to surface a skip note rather than silently ignoring them.
func isMaybePodCarrier(kind string) bool {
	switch kind {
	case "Rollout", "Service" /* Knative */, "TaskRun", "PipelineRun":
		return true
	}
	return false
}
