package certmanager_test

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kubehealth-project/kubehealth"
	"github.com/kubehealth-project/kubehealth/integrations/certmanager"
)

func TestCertificate(t *testing.T) {
	tests := []struct {
		name          string
		conditions    []any
		generation    int64
		status        map[string]any
		terminating   bool
		wantRec       kubehealth.ReconciliationStatus
		wantAvail     kubehealth.AvailabilityStatus
		wantLifecycle kubehealth.LifecycleStatus
	}{
		{name: "ready", conditions: conditions(condition("Ready", "True", "Issued", "Certificate is up to date", 1)), generation: 1, wantRec: kubehealth.ReconciliationReconciled, wantAvail: kubehealth.AvailabilityAvailable},
		{name: "renewing ready certificate", conditions: conditions(condition("Issuing", "True", "Renewing", "Renewing certificate", 2), condition("Ready", "True", "Issued", "Current certificate is valid", 1)), generation: 2, wantRec: kubehealth.ReconciliationInProgress, wantAvail: kubehealth.AvailabilityAvailable},
		{name: "issuing condition wins regardless of order", conditions: conditions(condition("Ready", "False", "DoesNotExist", "No certificate", 1), condition("Issuing", "True", "DoesNotExist", "Issuing certificate", 1)), generation: 1, wantRec: kubehealth.ReconciliationInProgress, wantAvail: kubehealth.AvailabilityUnavailable},
		{name: "issuing without readiness", conditions: conditions(condition("Issuing", "True", "DoesNotExist", "Issuing certificate", 1)), generation: 1, wantRec: kubehealth.ReconciliationInProgress, wantAvail: kubehealth.AvailabilityUnknown},
		{name: "issuing unknown", conditions: conditions(condition("Issuing", "Unknown", "Pending", "Issuance state unknown", 1), condition("Ready", "True", "Issued", "Current certificate is valid", 1)), generation: 1, wantRec: kubehealth.ReconciliationUnknown, wantAvail: kubehealth.AvailabilityAvailable},
		{name: "failed", conditions: conditions(condition("Ready", "False", "DoesNotExist", "Issuing certificate failed", 1)), generation: 1, wantRec: kubehealth.ReconciliationFailed, wantAvail: kubehealth.AvailabilityUnavailable},
		{name: "readiness unknown", conditions: conditions(condition("Ready", "Unknown", "Pending", "Readiness unknown", 1)), generation: 1, wantRec: kubehealth.ReconciliationUnknown, wantAvail: kubehealth.AvailabilityUnknown},
		{name: "missing status", wantRec: kubehealth.ReconciliationInProgress, wantAvail: kubehealth.AvailabilityUnknown},
		{name: "empty conditions", conditions: []any{}, wantRec: kubehealth.ReconciliationInProgress, wantAvail: kubehealth.AvailabilityUnknown},
		{name: "unrelated condition", conditions: conditions(condition("Other", "True", "Observed", "Observed", 1)), generation: 1, wantRec: kubehealth.ReconciliationUnknown, wantAvail: kubehealth.AvailabilityUnknown},
		{name: "duplicate ready conditions", conditions: conditions(condition("Ready", "True", "Issued", "Ready", 1), condition("Ready", "False", "Failed", "Failed", 1)), generation: 1, wantRec: kubehealth.ReconciliationUnknown, wantAvail: kubehealth.AvailabilityUnknown},
		{name: "stale ready remains available", conditions: conditions(condition("Ready", "True", "Issued", "Old generation ready", 2)), generation: 3, wantRec: kubehealth.ReconciliationInProgress, wantAvail: kubehealth.AvailabilityAvailable},
		{name: "stale failed remains unavailable", conditions: conditions(condition("Ready", "False", "Failed", "Old generation failed", 2)), generation: 3, wantRec: kubehealth.ReconciliationInProgress, wantAvail: kubehealth.AvailabilityUnavailable},
		{name: "latest renewal failed but current certificate ready", conditions: conditions(condition("Ready", "True", "Issued", "Current certificate valid", 1)), generation: 1, status: map[string]any{"lastFailureTime": "2026-08-24T10:00:00Z"}, wantRec: kubehealth.ReconciliationFailed, wantAvail: kubehealth.AvailabilityAvailable},
		{name: "active retry takes precedence over failure", conditions: conditions(condition("Issuing", "True", "Retrying", "Retrying", 1), condition("Ready", "True", "Issued", "Current certificate valid", 1)), generation: 1, status: map[string]any{"failedIssuanceAttempts": int64(2)}, wantRec: kubehealth.ReconciliationInProgress, wantAvail: kubehealth.AvailabilityAvailable},
		{name: "terminating ready certificate", conditions: conditions(condition("Ready", "True", "Issued", "Ready", 1)), generation: 1, terminating: true, wantRec: kubehealth.ReconciliationUnknown, wantAvail: kubehealth.AvailabilityAvailable, wantLifecycle: kubehealth.LifecycleTerminating},
	}

	assessor := newAssessor(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := resource("cert-manager.io/v1", "Certificate", tt.conditions)
			obj.SetGeneration(tt.generation)
			for key, value := range tt.status {
				obj.Object["status"].(map[string]any)[key] = value
			}
			if tt.terminating {
				now := metav1.NewTime(time.Now())
				obj.SetDeletionTimestamp(&now)
			}
			got := assess(t, assessor, obj)
			assertAssessment(t, got, tt.wantRec, tt.wantAvail, lifecycleOrActive(tt.wantLifecycle))
			if len(tt.conditions) > 0 && len(got.Conditions) == 0 {
				t.Error("conditions were not preserved")
			}
		})
	}
}

func TestCertificateMalformedConditions(t *testing.T) {
	obj := resource("cert-manager.io/v1", "Certificate", nil)
	obj.Object["status"] = map[string]any{"conditions": "invalid"}
	if _, err := newAssessor(t).Assess(obj); err == nil {
		t.Fatal("expected malformed conditions error")
	}
}

func newAssessor(t *testing.T) *kubehealth.Assessor {
	t.Helper()
	assessor := kubehealth.NewAssessor()
	if err := certmanager.Register(assessor); err != nil {
		t.Fatal(err)
	}
	return assessor
}

func assess(t *testing.T, assessor *kubehealth.Assessor, obj *unstructured.Unstructured) kubehealth.Assessment {
	t.Helper()
	got, err := assessor.Assess(obj)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func assertAssessment(t *testing.T, got kubehealth.Assessment, reconciliation kubehealth.ReconciliationStatus, availability kubehealth.AvailabilityStatus, lifecycle kubehealth.LifecycleStatus) {
	t.Helper()
	if got.Reconciliation != reconciliation || got.Availability != availability || got.Lifecycle != lifecycle {
		t.Fatalf("assessment = %#v, want reconciliation=%q availability=%q lifecycle=%q", got, reconciliation, availability, lifecycle)
	}
}

func lifecycleOrActive(status kubehealth.LifecycleStatus) kubehealth.LifecycleStatus {
	if status == "" {
		return kubehealth.LifecycleActive
	}
	return status
}

func resource(apiVersion, kind string, conditions []any) *unstructured.Unstructured {
	object := map[string]any{
		"apiVersion": apiVersion,
		"kind":       kind,
		"metadata":   map[string]any{"name": "example"},
	}
	if conditions != nil {
		object["status"] = map[string]any{"conditions": conditions}
	}
	return &unstructured.Unstructured{Object: object}
}

func conditions(values ...map[string]any) []any {
	result := make([]any, len(values))
	for i := range values {
		result[i] = values[i]
	}
	return result
}

func condition(conditionType, status, reason, message string, observedGeneration int64) map[string]any {
	result := map[string]any{"type": conditionType, "status": status, "reason": reason, "message": message}
	if observedGeneration >= 0 {
		result["observedGeneration"] = observedGeneration
	}
	return result
}
