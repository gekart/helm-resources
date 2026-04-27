package aggregate

import (
	"strings"
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

// TestBucketKey_AllBranches exercises every grouping mode + empty-value fallbacks.
func TestBucketKey_AllBranches(t *testing.T) {
	cases := []struct {
		name    string
		w       parse.Workload
		groupBy string
		want    string
	}{
		{"kind", parse.Workload{Kind: "Deployment"}, "kind", "Deployment"},
		{"namespace_set", parse.Workload{Namespace: "prod"}, "namespace", "prod"},
		{"namespace_empty_falls_to_default", parse.Workload{}, "namespace", "default"},
		{"none", parse.Workload{}, "none", "total"},
		{"subchart_set", parse.Workload{Chart: "foo"}, "subchart", "foo"},
		{"subchart_empty_falls_to_root", parse.Workload{}, "subchart", "(root)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := bucketKey(tc.w, tc.groupBy); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestEffectiveReplicas_KindBranches covers Pod, CronJob, and missing-replicas
// branches.
func TestEffectiveReplicas_KindBranches(t *testing.T) {
	cases := []struct {
		name string
		w    parse.Workload
		want int64
	}{
		{"pod_always_one", parse.Workload{Kind: "Pod"}, 1},
		{"cronjob_always_one", parse.Workload{Kind: "CronJob"}, 1},
		{"deployment_nil_replicas_defaults_to_one", parse.Workload{Kind: "Deployment"}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n, _ := effectiveReplicas(tc.w, 0)
			if n != tc.want {
				t.Errorf("got %d, want %d", n, tc.want)
			}
		})
	}
}

// TestPodResources_Limits exercises the limits-only path including the
// containers-and-init-limits max rule.
func TestPodResources_Limits(t *testing.T) {
	w := parse.Workload{
		Kind: "Deployment",
		Containers: []parse.Container{
			{Name: "a", Limits: rv("100m", "256Mi")},
		},
		InitContainers: []parse.Container{
			{Name: "init", Limits: rv("500m", "128Mi")},
		},
	}
	_, lim, _, err := podResources(w, true)
	if err != nil {
		t.Fatalf("podResources: %v", err)
	}
	if lim.CPUMilli != 500 {
		t.Errorf("limit cpu: got %d, want 500", lim.CPUMilli)
	}
	if lim.MemoryBytes != 256*1024*1024 {
		t.Errorf("limit mem: got %d, want 256Mi", lim.MemoryBytes)
	}
}

// TestPodResources_PodLevelLimitsOnly covers PodLevelResources where only
// limits are set.
func TestPodResources_PodLevelLimitsOnly(t *testing.T) {
	w := parse.Workload{
		Kind:      "Pod",
		PodLimits: rv("2", "4Gi"),
	}
	_, lim, _, err := podResources(w, true)
	if err != nil {
		t.Fatalf("podResources: %v", err)
	}
	if lim.CPUMilli != 2000 || lim.MemoryBytes != 4<<30 {
		t.Errorf("got %+v", lim)
	}
}

// TestPodResources_InvalidQuantity propagates parse errors with the right path.
func TestPodResources_InvalidQuantity(t *testing.T) {
	w := parse.Workload{
		Kind:       "Deployment",
		Containers: []parse.Container{{Name: "bad", Requests: rv("not-a-quantity", "")}},
	}
	if _, _, _, err := podResources(w, true); err == nil {
		t.Error("expected error on invalid quantity, got nil")
	}
}

// TestAggregate_InvalidQuantityErrorPath confirms Aggregate wraps the workload
// path into the error.
func TestAggregate_InvalidQuantityErrorPath(t *testing.T) {
	ws := []parse.Workload{
		{
			Kind:       "Deployment",
			Name:       "broken",
			Containers: []parse.Container{{Name: "c", Requests: rv("xyz", "")}},
		},
	}
	_, err := Aggregate(ws, Options{GroupBy: "none"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Deployment/broken") {
		t.Errorf("error should name workload: %v", err)
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
