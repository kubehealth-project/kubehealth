package externalsecrets_test

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kubehealth-project/kubehealth"
	"github.com/kubehealth-project/kubehealth/integrations/externalsecrets"
)

func newAssessor(t *testing.T) *kubehealth.Assessor {
	t.Helper()
	assessor := kubehealth.NewAssessor()
	if err := externalsecrets.Register(assessor); err != nil {
		t.Fatal(err)
	}
	return assessor
}

func resource(apiVersion, kind string, status map[string]any) *unstructured.Unstructured {
	object := map[string]any{
		"apiVersion": apiVersion,
		"kind":       kind,
		"metadata":   map[string]any{"name": "example", "generation": int64(2)},
	}
	if status != nil {
		object["status"] = status
	}
	return &unstructured.Unstructured{Object: object}
}

func condition(conditionType, status, reason, message string) map[string]any {
	return map[string]any{"type": conditionType, "status": status, "reason": reason, "message": message}
}

func conditions(items ...map[string]any) map[string]any {
	values := make([]any, len(items))
	for i := range items {
		values[i] = items[i]
	}
	return map[string]any{"conditions": values}
}

func assertAssessment(t *testing.T, got kubehealth.Assessment, reconciliation kubehealth.ReconciliationStatus, availability kubehealth.AvailabilityStatus) {
	t.Helper()
	if got.Reconciliation.Status != reconciliation {
		t.Errorf("reconciliation = %q, want %q", got.Reconciliation, reconciliation)
	}
	if got.Availability.Status != availability {
		t.Errorf("availability = %q, want %q", got.Availability, availability)
	}
	if got.Lifecycle.Status != kubehealth.LifecycleActive {
		t.Errorf("lifecycle = %q, want %q", got.Lifecycle, kubehealth.LifecycleActive)
	}
}

func markTerminating(obj *unstructured.Unstructured) {
	obj.SetDeletionTimestamp(&metav1.Time{Time: time.Unix(1, 0)})
}
