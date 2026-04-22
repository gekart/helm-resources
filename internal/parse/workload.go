package parse

import (
	"sigs.k8s.io/yaml"
)

type objectMeta struct {
	Name        string            `json:"name,omitempty"`
	Namespace   string            `json:"namespace,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type resourceRequirements struct {
	Requests map[string]string `json:"requests,omitempty"`
	Limits   map[string]string `json:"limits,omitempty"`
}

type containerSpec struct {
	Name      string               `json:"name"`
	Resources resourceRequirements `json:"resources,omitempty"`
}

type podSpec struct {
	Containers     []containerSpec       `json:"containers,omitempty"`
	InitContainers []containerSpec       `json:"initContainers,omitempty"`
	Resources      *resourceRequirements `json:"resources,omitempty"` // PodLevelResources (K8s 1.34+)
}

type podTemplate struct {
	Metadata objectMeta `json:"metadata,omitempty"`
	Spec     podSpec    `json:"spec"`
}

type workloadSpec struct {
	Replicas    *int64      `json:"replicas,omitempty"`
	Template    podTemplate `json:"template"`
	JobTemplate *struct {
		Spec struct {
			Template podTemplate `json:"template"`
		} `json:"spec"`
	} `json:"jobTemplate,omitempty"`
}

type workloadDoc struct {
	Kind     string       `json:"kind"`
	Metadata objectMeta   `json:"metadata"`
	Spec     workloadSpec `json:"spec"`
}

// podDoc is used for bare Pod kind (different spec shape).
type podDoc struct {
	Kind     string     `json:"kind"`
	Metadata objectMeta `json:"metadata"`
	Spec     podSpec    `json:"spec"`
}

// genericDoc is just enough to dispatch on kind.
type genericDoc struct {
	Kind     string     `json:"kind"`
	Metadata objectMeta `json:"metadata"`
}

func parseWorkload(body []byte, meta genericDoc, chartHint string) (Workload, error) {
	w := Workload{
		Kind:      meta.Kind,
		Name:      meta.Metadata.Name,
		Namespace: meta.Metadata.Namespace,
		Chart:     resolveChart(meta.Metadata, chartHint),
		IsHook:    hasHookAnnotation(meta.Metadata),
	}

	if meta.Kind == "Pod" {
		var pd podDoc
		if err := yaml.Unmarshal(body, &pd); err != nil {
			return w, err
		}
		w.Containers = toContainers(pd.Spec.Containers)
		w.InitContainers = toContainers(pd.Spec.InitContainers)
		if pd.Spec.Resources != nil {
			w.PodRequests, w.PodLimits = splitPodResources(pd.Spec.Resources)
		}
		return w, nil
	}

	var wd workloadDoc
	if err := yaml.Unmarshal(body, &wd); err != nil {
		return w, err
	}
	w.Replicas = wd.Spec.Replicas

	var ps podSpec
	if meta.Kind == "CronJob" && wd.Spec.JobTemplate != nil {
		ps = wd.Spec.JobTemplate.Spec.Template.Spec
	} else {
		ps = wd.Spec.Template.Spec
	}
	w.Containers = toContainers(ps.Containers)
	w.InitContainers = toContainers(ps.InitContainers)
	if ps.Resources != nil {
		w.PodRequests, w.PodLimits = splitPodResources(ps.Resources)
	}
	return w, nil
}

func toContainers(cs []containerSpec) []Container {
	if len(cs) == 0 {
		return nil
	}
	out := make([]Container, 0, len(cs))
	for _, c := range cs {
		cc := Container{Name: c.Name}
		if len(c.Resources.Requests) > 0 {
			rv := toResourceValues(c.Resources.Requests)
			cc.Requests = &rv
		}
		if len(c.Resources.Limits) > 0 {
			rv := toResourceValues(c.Resources.Limits)
			cc.Limits = &rv
		}
		out = append(out, cc)
	}
	return out
}

func toResourceValues(m map[string]string) ResourceValues {
	return ResourceValues{CPU: m["cpu"], Memory: m["memory"]}
}

func splitPodResources(r *resourceRequirements) (*ResourceValues, *ResourceValues) {
	var req, lim *ResourceValues
	if len(r.Requests) > 0 {
		v := toResourceValues(r.Requests)
		req = &v
	}
	if len(r.Limits) > 0 {
		v := toResourceValues(r.Limits)
		lim = &v
	}
	return req, lim
}

func resolveChart(m objectMeta, hint string) string {
	if v, ok := m.Labels["helm.sh/chart"]; ok && v != "" {
		// helm.sh/chart is of the form "name-version"; strip the trailing version.
		for i := len(v) - 1; i >= 0; i-- {
			if v[i] == '-' {
				return v[:i]
			}
		}
		return v
	}
	if v, ok := m.Labels["app.kubernetes.io/part-of"]; ok && v != "" {
		return v
	}
	return hint
}

func hasHookAnnotation(m objectMeta) bool {
	_, ok := m.Annotations["helm.sh/hook"]
	return ok
}

// --- HPA ---------------------------------------------------------------------

type hpaDoc struct {
	metadata objectMeta
	spec     hpaSpec
}

type hpaSpec struct {
	ScaleTargetRef struct {
		Kind string `json:"kind"`
		Name string `json:"name"`
	} `json:"scaleTargetRef"`
	MaxReplicas *int64 `json:"maxReplicas,omitempty"`
}

type hpaRaw struct {
	Metadata objectMeta `json:"metadata"`
	Spec     hpaSpec    `json:"spec"`
}

func parseHPA(body []byte) (hpaDoc, bool) {
	var r hpaRaw
	if err := yaml.Unmarshal(body, &r); err != nil {
		return hpaDoc{}, false
	}
	return hpaDoc{metadata: r.Metadata, spec: r.Spec}, true
}
