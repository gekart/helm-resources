package aggregate

import (
	"testing"

	"github.com/gekart/helm-resources/internal/parse"
)

func ptr[T any](v T) *T { return &v }

func rv(cpu, mem string) *parse.ResourceValues {
	return &parse.ResourceValues{CPU: cpu, Memory: mem}
}

// TestPodResources_InitMaxRule checks that per-pod effective request is
// max(sum(containers), max(initContainers)).
func TestPodResources_InitMaxRule(t *testing.T) {
	w := parse.Workload{
		Kind: "Deployment",
		Name: "x",
		Containers: []parse.Container{
			{Name: "a", Requests: rv("100m", "128Mi")},
			{Name: "b", Requests: rv("50m", "64Mi")},
		},
		InitContainers: []parse.Container{
			{Name: "init-big-cpu", Requests: rv("500m", "16Mi")},
			{Name: "init-big-mem", Requests: rv("10m", "1Gi")},
		},
	}
	req, _, _, err := podResources(w, true)
	if err != nil {
		t.Fatalf("podResources: %v", err)
	}
	// sum(containers) cpu = 150m, max init cpu = 500m → want 500m
	if req.CPUMilli != 500 {
		t.Errorf("cpu: got %d, want 500", req.CPUMilli)
	}
	// sum(containers) mem = 192Mi, max init mem = 1Gi → want 1Gi
	if req.MemoryBytes != 1<<30 {
		t.Errorf("mem: got %d, want %d", req.MemoryBytes, 1<<30)
	}
}

// TestPodResources_InitDisabled confirms init containers are ignored when
// includeInit=false.
func TestPodResources_InitDisabled(t *testing.T) {
	w := parse.Workload{
		Kind: "Deployment",
		Containers: []parse.Container{
			{Name: "a", Requests: rv("100m", "128Mi")},
		},
		InitContainers: []parse.Container{
			{Name: "init", Requests: rv("500m", "1Gi")},
		},
	}
	req, _, _, err := podResources(w, false)
	if err != nil {
		t.Fatalf("podResources: %v", err)
	}
	if req.CPUMilli != 100 {
		t.Errorf("cpu: got %d, want 100", req.CPUMilli)
	}
}

// TestPodResources_PodLevelWins checks PodLevelResources overrides container
// sums when present.
func TestPodResources_PodLevelWins(t *testing.T) {
	w := parse.Workload{
		Kind:        "Pod",
		PodRequests: rv("2", "4Gi"),
		Containers: []parse.Container{
			{Name: "a", Requests: rv("100m", "128Mi")},
		},
	}
	req, _, _, err := podResources(w, true)
	if err != nil {
		t.Fatalf("podResources: %v", err)
	}
	if req.CPUMilli != 2000 {
		t.Errorf("cpu: got %d, want 2000", req.CPUMilli)
	}
	if req.MemoryBytes != 4<<30 {
		t.Errorf("mem: got %d, want %d", req.MemoryBytes, 4<<30)
	}
}

// TestPodResources_MissingContainer reports containers with no resources block.
func TestPodResources_MissingContainer(t *testing.T) {
	w := parse.Workload{
		Kind: "Deployment",
		Containers: []parse.Container{
			{Name: "a", Requests: rv("100m", "128Mi")},
			{Name: "leaky"}, // no requests or limits
		},
	}
	_, _, missing, err := podResources(w, true)
	if err != nil {
		t.Fatalf("podResources: %v", err)
	}
	if len(missing) != 1 || missing[0] != "leaky" {
		t.Errorf("missing: got %v, want [leaky]", missing)
	}
}

// TestAggregate_DaemonSetPerNode confirms DaemonSet multiplier is 1 without
// --nodes and N with --nodes N, and that the per-node note is emitted when
// nodes is 0.
func TestAggregate_DaemonSetPerNode(t *testing.T) {
	ws := []parse.Workload{
		{
			Kind:       "DaemonSet",
			Name:       "ds",
			Containers: []parse.Container{{Name: "c", Requests: rv("100m", "128Mi")}},
		},
	}
	opts := Options{GroupBy: "kind", IncludeInit: true, WarnMissing: false}
	rep, err := Aggregate(ws, opts)
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	if rep.Grand.Requests.CPUMilli != 100 {
		t.Errorf("per-node cpu: got %d, want 100", rep.Grand.Requests.CPUMilli)
	}
	if len(rep.Notes) == 0 {
		t.Error("expected per-node note when --nodes is 0")
	}

	opts.Nodes = 5
	rep, err = Aggregate(ws, opts)
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	if rep.Grand.Requests.CPUMilli != 500 {
		t.Errorf("with nodes=5 cpu: got %d, want 500", rep.Grand.Requests.CPUMilli)
	}
	if rep.Grand.Pods != 5 {
		t.Errorf("with nodes=5 pods: got %d, want 5", rep.Grand.Pods)
	}
}

// TestAggregate_ReplicasMultiply confirms Deployment replicas are applied.
func TestAggregate_ReplicasMultiply(t *testing.T) {
	ws := []parse.Workload{
		{
			Kind:     "Deployment",
			Name:     "web",
			Replicas: ptr[int64](3),
			Containers: []parse.Container{
				{Name: "web", Requests: rv("100m", "128Mi")},
			},
		},
	}
	rep, err := Aggregate(ws, Options{GroupBy: "none", IncludeInit: true})
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	if rep.Grand.Requests.CPUMilli != 300 {
		t.Errorf("cpu: got %d, want 300", rep.Grand.Requests.CPUMilli)
	}
	if rep.Grand.Pods != 3 {
		t.Errorf("pods: got %d, want 3", rep.Grand.Pods)
	}
}

// TestAggregate_HookExclusion confirms hook-annotated workloads are skipped by
// default.
func TestAggregate_HookExclusion(t *testing.T) {
	ws := []parse.Workload{
		{
			Kind:       "Job",
			Name:       "migrate",
			IsHook:     true,
			Containers: []parse.Container{{Name: "c", Requests: rv("100m", "128Mi")}},
		},
	}
	rep, err := Aggregate(ws, Options{GroupBy: "none"})
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	if rep.Grand.Pods != 0 {
		t.Errorf("hook excluded: got pods=%d, want 0", rep.Grand.Pods)
	}

	rep, err = Aggregate(ws, Options{GroupBy: "none", IncludeHooks: true})
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	if rep.Grand.Pods != 1 {
		t.Errorf("hook included: got pods=%d, want 1", rep.Grand.Pods)
	}
}
