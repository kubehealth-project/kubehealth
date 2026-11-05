package main

import (
	"testing"

	"github.com/kubehealth-project/kubehealth"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestFailedUpdateCanRemainAvailable(t *testing.T) {
	deployment := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]any{
			"name": "kubehealth-demo", "namespace": "default", "generation": int64(2),
		},
		"spec": map[string]any{"replicas": int64(2)},
		"status": map[string]any{
			"observedGeneration": int64(2),
			"updatedReplicas":    int64(0),
			"replicas":           int64(2),
			"availableReplicas":  int64(2),
			"conditions": []any{
				map[string]any{
					"type": "Progressing", "status": "False",
					"reason":  "ProgressDeadlineExceeded",
					"message": "ReplicaSet failed to make progress",
				},
			},
		},
	}}

	assessment, err := kubehealth.Assess(deployment)
	if err != nil {
		t.Fatalf("assess Deployment: %v", err)
	}
	if assessment.Reconciliation.Status != kubehealth.ReconciliationFailed {
		t.Errorf("reconciliation = %q, want %q", assessment.Reconciliation.Status, kubehealth.ReconciliationFailed)
	}
	if assessment.Availability.Status != kubehealth.AvailabilityAvailable {
		t.Errorf("availability = %q, want %q", assessment.Availability.Status, kubehealth.AvailabilityAvailable)
	}
	if assessment.Lifecycle.Status != kubehealth.LifecycleActive {
		t.Errorf("lifecycle = %q, want %q", assessment.Lifecycle.Status, kubehealth.LifecycleActive)
	}
}
