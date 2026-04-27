package parse

import (
	"strings"
	"testing"
)

const sampleManifest = `# Source: umbrella/templates/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  labels:
    helm.sh/chart: umbrella-0.1.0
spec:
  replicas: 3
  template:
    spec:
      initContainers:
        - name: wait
          resources:
            requests:
              cpu: 50m
              memory: 32Mi
      containers:
        - name: web
          resources:
            requests:
              cpu: 250m
              memory: 256Mi
            limits:
              cpu: 500m
              memory: 512Mi
---
# Source: umbrella/charts/subchart-a/templates/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: cache
  labels:
    helm.sh/chart: subchart-a-0.1.0
spec:
  replicas: 1
  template:
    spec:
      containers:
        - name: cache
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
---
# Source: umbrella/templates/cronjob.yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: backup
spec:
  schedule: "0 2 * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
            - name: backup
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cfg
data:
  x: y
`

func TestParse_Basic(t *testing.T) {
	ws, _, err := Parse(strings.NewReader(sampleManifest), ParseOptions{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(ws) != 3 {
		t.Fatalf("workloads: got %d, want 3", len(ws))
	}

	// Deployment web: replicas=3, one container + one init container, chart=umbrella
	web := findWorkload(ws, "web")
	if web == nil {
		t.Fatal("missing workload web")
	}
	if web.Chart != "umbrella" {
		t.Errorf("web chart: got %q, want umbrella", web.Chart)
	}
	if web.Replicas == nil || *web.Replicas != 3 {
		t.Errorf("web replicas: got %v, want 3", web.Replicas)
	}
	if len(web.Containers) != 1 || web.Containers[0].Name != "web" {
		t.Errorf("web containers: got %+v", web.Containers)
	}
	if len(web.InitContainers) != 1 {
		t.Errorf("web init: got %+v", web.InitContainers)
	}

	// subchart-a cache
	cache := findWorkload(ws, "cache")
	if cache == nil {
		t.Fatal("missing workload cache")
	}
	if cache.Chart != "subchart-a" {
		t.Errorf("cache chart: got %q, want subchart-a", cache.Chart)
	}

	// CronJob backup: parsed via jobTemplate
	backup := findWorkload(ws, "backup")
	if backup == nil {
		t.Fatal("missing workload backup")
	}
	if backup.Kind != "CronJob" {
		t.Errorf("backup kind: got %q", backup.Kind)
	}
	if len(backup.Containers) != 1 || backup.Containers[0].Name != "backup" {
		t.Errorf("cronjob containers: got %+v", backup.Containers)
	}
	if backup.Containers[0].Requests != nil {
		t.Error("cronjob container should have no requests (missing resources block)")
	}
}

func findWorkload(ws []Workload, name string) *Workload {
	for i := range ws {
		if ws[i].Name == name {
			return &ws[i]
		}
	}
	return nil
}

// TestParse_PodKindWithPodLevelResources covers the Pod-specific branch of
// parseWorkload plus splitPodResources (PodLevelResources, K8s 1.34+).
func TestParse_PodKindWithPodLevelResources(t *testing.T) {
	const m = `apiVersion: v1
kind: Pod
metadata:
  name: standalone
  namespace: ns-x
spec:
  resources:
    requests:
      cpu: 500m
      memory: 1Gi
    limits:
      cpu: "1"
      memory: 2Gi
  containers:
    - name: main
      resources:
        requests:
          cpu: 100m
          memory: 128Mi
`
	ws, _, err := Parse(strings.NewReader(m), ParseOptions{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(ws) != 1 {
		t.Fatalf("workloads: got %d, want 1", len(ws))
	}
	w := ws[0]
	if w.Kind != "Pod" || w.Name != "standalone" || w.Namespace != "ns-x" {
		t.Errorf("identity: got %+v", w)
	}
	if w.PodRequests == nil || w.PodRequests.CPU != "500m" || w.PodRequests.Memory != "1Gi" {
		t.Errorf("pod-level requests: got %+v", w.PodRequests)
	}
	if w.PodLimits == nil || w.PodLimits.CPU != "1" || w.PodLimits.Memory != "2Gi" {
		t.Errorf("pod-level limits: got %+v", w.PodLimits)
	}
	if len(w.Containers) != 1 {
		t.Errorf("containers: got %+v", w.Containers)
	}
}

// TestParse_HookAnnotation flags helm.sh/hook-annotated workloads.
func TestParse_HookAnnotation(t *testing.T) {
	const m = `apiVersion: batch/v1
kind: Job
metadata:
  name: migrate
  annotations:
    helm.sh/hook: pre-install
spec:
  template:
    spec:
      containers:
        - name: migrator
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
`
	ws, _, err := Parse(strings.NewReader(m), ParseOptions{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(ws) != 1 || !ws[0].IsHook {
		t.Errorf("expected one hook workload; got %+v", ws)
	}
}

// TestParse_HPALinkage attaches HPA maxReplicas onto the matching workload.
func TestParse_HPALinkage(t *testing.T) {
	const m = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
  namespace: prod
spec:
  replicas: 2
  template:
    spec:
      containers:
        - name: api
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: api
  namespace: prod
spec:
  scaleTargetRef:
    kind: Deployment
    name: api
  maxReplicas: 10
`
	ws, _, err := Parse(strings.NewReader(m), ParseOptions{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(ws) != 1 {
		t.Fatalf("workloads: got %d, want 1", len(ws))
	}
	if ws[0].HPAMaxReplicas == nil || *ws[0].HPAMaxReplicas != 10 {
		t.Errorf("HPA linkage: got %v, want 10", ws[0].HPAMaxReplicas)
	}
}

// TestParse_UnknownPodCarrierEmitsNote covers isMaybePodCarrier's positive
// path (a Knative Service) and ConfigMap silent-skip path.
func TestParse_UnknownPodCarrierEmitsNote(t *testing.T) {
	const m = `apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: knative-svc
spec: {}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cfg
data: {}
`
	ws, notes, err := Parse(strings.NewReader(m), ParseOptions{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(ws) != 0 {
		t.Errorf("workloads: got %d, want 0", len(ws))
	}
	if len(notes) != 1 || !strings.Contains(notes[0], "Service") {
		t.Errorf("notes: got %v", notes)
	}
}

// TestParse_PartOfLabelFallback covers resolveChart's app.kubernetes.io/part-of
// branch when helm.sh/chart is absent.
func TestParse_PartOfLabelFallback(t *testing.T) {
	const m = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: thing
  labels:
    app.kubernetes.io/part-of: my-platform
spec:
  template:
    spec:
      containers:
        - name: c
          resources:
            requests:
              cpu: 10m
              memory: 16Mi
`
	ws, _, err := Parse(strings.NewReader(m), ParseOptions{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(ws) != 1 || ws[0].Chart != "my-platform" {
		t.Errorf("chart fallback: got %v", ws)
	}
}

// TestParse_BadYAML returns an error rather than panicking.
func TestParse_BadYAML(t *testing.T) {
	const m = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: bad
spec:
  replicas: not-a-number
`
	if _, _, err := Parse(strings.NewReader(m), ParseOptions{}); err == nil {
		t.Error("expected error on malformed yaml, got nil")
	}
}
