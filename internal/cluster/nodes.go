// Package cluster contains optional Kubernetes API helpers used to enrich
// resource totals with cluster-side context (node count, etc.). Cluster
// contact is best-effort: callers are expected to degrade gracefully when
// kubeconfig is missing, the API is unreachable, or RBAC is denied.
package cluster

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
)

// CountNodes returns the number of Node objects visible to cs. It does not
// filter for schedulability; callers that care about effective DaemonSet
// placement should consult node taints/selectors separately.
func CountNodes(ctx context.Context, cs kubernetes.Interface) (int, error) {
	nl, err := cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return 0, err
	}
	return len(nl.Items), nil
}

// ClientFromGetter builds a Kubernetes clientset from a Helm RESTClientGetter
// (typically obtained via cli.New().RESTClientGetter()).
func ClientFromGetter(getter genericclioptions.RESTClientGetter) (kubernetes.Interface, error) {
	cfg, err := getter.ToRESTConfig()
	if err != nil {
		return nil, fmt.Errorf("rest config: %w", err)
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("kube client: %w", err)
	}
	return cs, nil
}

// CountWithTimeout is the convenience used by the CLI: build a clientset from
// the getter, list nodes under a bounded context.
func CountWithTimeout(parent context.Context, getter genericclioptions.RESTClientGetter, d time.Duration) (int, error) {
	cs, err := ClientFromGetter(getter)
	if err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(parent, d)
	defer cancel()
	return CountNodes(ctx, cs)
}
