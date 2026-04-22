package aggregate

import (
	"fmt"
	"sort"

	"github.com/gekart/helm-resources/internal/parse"
)

// Resources is a pair of CPU (millicores) and memory (bytes).
type Resources struct {
	CPUMilli    int64 `json:"cpuMilli" yaml:"cpuMilli"`
	MemoryBytes int64 `json:"memoryBytes" yaml:"memoryBytes"`
}

func (r *Resources) add(o Resources) {
	r.CPUMilli += o.CPUMilli
	r.MemoryBytes += o.MemoryBytes
}

// Totals is the request + limit + pod count for a workload or bucket.
type Totals struct {
	Requests Resources `json:"requests" yaml:"requests"`
	Limits   Resources `json:"limits" yaml:"limits"`
	Pods     int64     `json:"pods" yaml:"pods"`
}

func (t *Totals) add(o Totals) {
	t.Requests.add(o.Requests)
	t.Limits.add(o.Limits)
	t.Pods += o.Pods
}

// Bucket is a grouped total (by subchart, kind, namespace, etc.).
type Bucket struct {
	Name   string `json:"name" yaml:"name"`
	Totals Totals `json:"totals" yaml:"totals"`
}

// Report is the output of aggregation.
type Report struct {
	GroupBy  string   `json:"groupBy" yaml:"groupBy"`
	Groups   []Bucket `json:"groups" yaml:"groups"`
	Grand    Totals   `json:"total" yaml:"total"`
	Warnings []string `json:"warnings,omitempty" yaml:"warnings,omitempty"`
	Notes    []string `json:"notes,omitempty" yaml:"notes,omitempty"`
}

// Options controls aggregation behavior.
type Options struct {
	GroupBy      string // subchart | kind | namespace | none
	Nodes        int    // DaemonSet multiplier; 0 means "per node" (treated as 1 with a note)
	IncludeInit  bool
	WarnMissing  bool
	IncludeHooks bool
}

// Aggregate walks workloads and produces a Report.
func Aggregate(workloads []parse.Workload, opts Options) (Report, error) {
	rep := Report{GroupBy: opts.GroupBy}
	buckets := map[string]*Bucket{}
	seenPerNode := false

	for _, w := range workloads {
		if w.IsHook && !opts.IncludeHooks {
			continue
		}

		replicas, note := effectiveReplicas(w, opts.Nodes)
		if note != "" {
			seenPerNode = true
		}

		podReq, podLim, missing, err := podResources(w, opts.IncludeInit)
		if err != nil {
			return rep, fmt.Errorf("%s/%s: %w", w.Kind, w.Name, err)
		}
		if opts.WarnMissing {
			for _, m := range missing {
				rep.Warnings = append(rep.Warnings,
					fmt.Sprintf("%s/%s: container %q has no resources declared", w.Kind, w.Name, m))
			}
		}

		wlTotals := Totals{
			Requests: Resources{CPUMilli: podReq.CPUMilli * replicas, MemoryBytes: podReq.MemoryBytes * replicas},
			Limits:   Resources{CPUMilli: podLim.CPUMilli * replicas, MemoryBytes: podLim.MemoryBytes * replicas},
			Pods:     replicas,
		}

		key := bucketKey(w, opts.GroupBy)
		b, ok := buckets[key]
		if !ok {
			b = &Bucket{Name: key}
			buckets[key] = b
		}
		b.Totals.add(wlTotals)
		rep.Grand.add(wlTotals)
	}

	for _, b := range buckets {
		rep.Groups = append(rep.Groups, *b)
	}
	sort.Slice(rep.Groups, func(i, j int) bool { return rep.Groups[i].Name < rep.Groups[j].Name })

	if seenPerNode {
		if opts.Nodes <= 0 {
			rep.Notes = append(rep.Notes, "DaemonSet totals are per-node; pass --nodes N to multiply")
		} else {
			rep.Notes = append(rep.Notes, fmt.Sprintf("DaemonSet totals multiplied by --nodes=%d", opts.Nodes))
		}
	}
	return rep, nil
}

// effectiveReplicas returns the replica multiplier for a workload and a
// non-empty note string if the result is "per node" (DaemonSet with no --nodes).
func effectiveReplicas(w parse.Workload, nodes int) (int64, string) {
	switch w.Kind {
	case "DaemonSet":
		if nodes <= 0 {
			return 1, "per-node"
		}
		return int64(nodes), ""
	case "CronJob":
		// Per-run total; one concurrent invocation assumed.
		return 1, ""
	case "Pod":
		return 1, ""
	}
	if w.Replicas == nil {
		return 1, ""
	}
	return *w.Replicas, ""
}

func bucketKey(w parse.Workload, groupBy string) string {
	switch groupBy {
	case "kind":
		return w.Kind
	case "namespace":
		if w.Namespace == "" {
			return "default"
		}
		return w.Namespace
	case "none":
		return "total"
	default: // subchart
		if w.Chart == "" {
			return "(root)"
		}
		return w.Chart
	}
}

// podResources computes per-pod request/limit resources using the Kubernetes
// effective-pod-request rule:
//
//	effective = max(sum(containers), max(initContainers))
//
// applied independently to CPU and memory, for requests and limits.
//
// When pod-level resources are set (PodLevelResources, K8s 1.34+), they win.
// Returns the list of container names with no resources block.
func podResources(w parse.Workload, includeInit bool) (req, lim Resources, missing []string, err error) {
	if w.PodRequests != nil || w.PodLimits != nil {
		if w.PodRequests != nil {
			r, err := toResources(*w.PodRequests)
			if err != nil {
				return req, lim, nil, err
			}
			req = r
		}
		if w.PodLimits != nil {
			r, err := toResources(*w.PodLimits)
			if err != nil {
				return req, lim, nil, err
			}
			lim = r
		}
		return req, lim, nil, nil
	}

	var sumReq, sumLim Resources
	for _, c := range w.Containers {
		if c.Requests == nil && c.Limits == nil {
			missing = append(missing, c.Name)
			continue
		}
		if c.Requests != nil {
			r, err := toResources(*c.Requests)
			if err != nil {
				return req, lim, nil, err
			}
			sumReq.add(r)
		}
		if c.Limits != nil {
			r, err := toResources(*c.Limits)
			if err != nil {
				return req, lim, nil, err
			}
			sumLim.add(r)
		}
	}

	var maxInitReq, maxInitLim Resources
	if includeInit {
		for _, c := range w.InitContainers {
			if c.Requests != nil {
				r, err := toResources(*c.Requests)
				if err != nil {
					return req, lim, nil, err
				}
				if r.CPUMilli > maxInitReq.CPUMilli {
					maxInitReq.CPUMilli = r.CPUMilli
				}
				if r.MemoryBytes > maxInitReq.MemoryBytes {
					maxInitReq.MemoryBytes = r.MemoryBytes
				}
			}
			if c.Limits != nil {
				r, err := toResources(*c.Limits)
				if err != nil {
					return req, lim, nil, err
				}
				if r.CPUMilli > maxInitLim.CPUMilli {
					maxInitLim.CPUMilli = r.CPUMilli
				}
				if r.MemoryBytes > maxInitLim.MemoryBytes {
					maxInitLim.MemoryBytes = r.MemoryBytes
				}
			}
		}
	}

	req = Resources{
		CPUMilli:    maxInt64(sumReq.CPUMilli, maxInitReq.CPUMilli),
		MemoryBytes: maxInt64(sumReq.MemoryBytes, maxInitReq.MemoryBytes),
	}
	lim = Resources{
		CPUMilli:    maxInt64(sumLim.CPUMilli, maxInitLim.CPUMilli),
		MemoryBytes: maxInt64(sumLim.MemoryBytes, maxInitLim.MemoryBytes),
	}
	return req, lim, missing, nil
}

func toResources(r parse.ResourceValues) (Resources, error) {
	cpu, err := ParseCPU(r.CPU)
	if err != nil {
		return Resources{}, err
	}
	mem, err := ParseMemory(r.Memory)
	if err != nil {
		return Resources{}, err
	}
	return Resources{CPUMilli: cpu, MemoryBytes: mem}, nil
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
