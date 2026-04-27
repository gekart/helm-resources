package cluster

import (
	"context"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestCountNodes(t *testing.T) {
	cs := fake.NewSimpleClientset(
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "a"}},
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "b"}},
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "c"}},
	)
	n, err := CountNodes(context.Background(), cs)
	if err != nil {
		t.Fatalf("CountNodes: %v", err)
	}
	if n != 3 {
		t.Errorf("got %d, want 3", n)
	}
}

func TestCountNodes_Empty(t *testing.T) {
	cs := fake.NewSimpleClientset()
	n, err := CountNodes(context.Background(), cs)
	if err != nil {
		t.Fatalf("CountNodes: %v", err)
	}
	if n != 0 {
		t.Errorf("got %d, want 0", n)
	}
}

func TestCountNodes_APIError(t *testing.T) {
	cs := fake.NewSimpleClientset()
	cs.PrependReactor("list", "nodes", func(_ k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("forbidden: user cannot list nodes")
	})
	if _, err := CountNodes(context.Background(), cs); err == nil {
		t.Error("expected error, got nil")
	}
}
