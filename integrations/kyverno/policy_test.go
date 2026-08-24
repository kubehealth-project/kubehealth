package kyverno_test

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kubehealth-project/kubehealth"
	"github.com/kubehealth-project/kubehealth/integrations/kyverno"
)

func TestPolicy(t *testing.T) {
	assessor := kubehealth.NewAssessor()
	if err := kyverno.Register(assessor); err != nil {
		t.Fatal(err)
	}

	got := assess(t, assessor, policy("Policy", []any{readyCondition("True", "Succeeded", "Ready")}))
	if got.Reconciliation != kubehealth.ReconciliationReconciled || got.Availability != kubehealth.AvailabilityAvailable {
		t.Fatalf("assessment = %#v", got)
	}

	got = assess(t, assessor, policy("Policy", nil))
	if got.Reconciliation != kubehealth.ReconciliationInProgress || got.Availability != kubehealth.AvailabilityUnavailable {
		t.Fatalf("assessment = %#v", got)
	}
}

func assess(t *testing.T, assessor *kubehealth.Assessor, obj *unstructured.Unstructured) kubehealth.Assessment {
	t.Helper()
	got, err := assessor.Assess(obj)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func policy(kind string, conditions []any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "kyverno.io/v1", "kind": kind,
		"metadata": map[string]any{"name": "example"},
		"status":   map[string]any{"conditions": conditions},
	}}
}

func readyCondition(status, reason, message string) map[string]any {
	return map[string]any{"type": "Ready", "status": status, "reason": reason, "message": message}
}
