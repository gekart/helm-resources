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
