package kyverno_test

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kubehealth-project/kubehealth"
	"github.com/kubehealth-project/kubehealth/integrations/kyverno"
)

func TestPolicy(t *testing.T) {
	testPolicyVersions(t, "Policy")
}

func testPolicyVersions(t *testing.T, kind string) {
	tests := []struct {
		name          string
		conditions    []any
		generation    int64
		terminating   bool
		wantRec       kubehealth.ReconciliationStatus
		wantAvail     kubehealth.AvailabilityStatus
		wantLifecycle kubehealth.LifecycleStatus
	}{
		{name: "ready", conditions: conditions(readyCondition("True", "Succeeded", "Ready", 1)), generation: 1, wantRec: kubehealth.ReconciliationReconciled, wantAvail: kubehealth.AvailabilityAvailable},
		{name: "ready with arbitrary message", conditions: conditions(readyCondition("True", "Succeeded", "Loaded admission rules", 1)), generation: 1, wantRec: kubehealth.ReconciliationReconciled, wantAvail: kubehealth.AvailabilityAvailable},
		{name: "ready without reason", conditions: conditions(readyCondition("True", "", "Loaded", 1)), generation: 1, wantRec: kubehealth.ReconciliationReconciled, wantAvail: kubehealth.AvailabilityAvailable},
		{name: "failed", conditions: conditions(readyCondition("False", "Failed", "Invalid policy", 1)), generation: 1, wantRec: kubehealth.ReconciliationFailed, wantAvail: kubehealth.AvailabilityUnavailable},
		{name: "unknown", conditions: conditions(readyCondition("Unknown", "Pending", "Unknown", 1)), generation: 1, wantRec: kubehealth.ReconciliationUnknown, wantAvail: kubehealth.AvailabilityUnknown},
		{name: "missing status", wantRec: kubehealth.ReconciliationInProgress, wantAvail: kubehealth.AvailabilityUnknown},
		{name: "empty conditions", conditions: []any{}, wantRec: kubehealth.ReconciliationInProgress, wantAvail: kubehealth.AvailabilityUnknown},
		{name: "unrelated condition", conditions: conditions(condition("Other", "True", "Observed", "Observed", 1)), generation: 1, wantRec: kubehealth.ReconciliationUnknown, wantAvail: kubehealth.AvailabilityUnknown},
		{name: "contradictory true failed", conditions: conditions(readyCondition("True", "Failed", "Contradictory", 1)), generation: 1, wantRec: kubehealth.ReconciliationUnknown, wantAvail: kubehealth.AvailabilityUnknown},
		{name: "contradictory false succeeded", conditions: conditions(readyCondition("False", "Succeeded", "Contradictory", 1)), generation: 1, wantRec: kubehealth.ReconciliationUnknown, wantAvail: kubehealth.AvailabilityUnknown},
		{name: "duplicate ready", conditions: conditions(readyCondition("True", "Succeeded", "Ready", 1), readyCondition("False", "Failed", "Failed", 1)), generation: 1, wantRec: kubehealth.ReconciliationUnknown, wantAvail: kubehealth.AvailabilityUnknown},
		{name: "stale ready remains available", conditions: conditions(readyCondition("True", "Succeeded", "Old readiness", 2)), generation: 3, wantRec: kubehealth.ReconciliationInProgress, wantAvail: kubehealth.AvailabilityAvailable},
		{name: "stale failed remains unavailable", conditions: conditions(readyCondition("False", "Failed", "Old failure", 2)), generation: 3, wantRec: kubehealth.ReconciliationInProgress, wantAvail: kubehealth.AvailabilityUnavailable},
		{name: "terminating ready", conditions: conditions(readyCondition("True", "Succeeded", "Ready", 1)), generation: 1, terminating: true, wantRec: kubehealth.ReconciliationUnknown, wantAvail: kubehealth.AvailabilityAvailable, wantLifecycle: kubehealth.LifecycleTerminating},
	}

	assessor := newAssessor(t)
	for _, version := range []string{"v1", "v2beta1"} {
		for _, tt := range tests {
			t.Run(version+"/"+tt.name, func(t *testing.T) {
				obj := policy("kyverno.io/"+version, kind, tt.conditions)
				obj.SetGeneration(tt.generation)
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
}

func TestPolicyMalformedConditions(t *testing.T) {
	obj := policy("kyverno.io/v1", "Policy", nil)
	obj.Object["status"] = map[string]any{"conditions": "invalid"}
	if _, err := newAssessor(t).Assess(obj); err == nil {
		t.Fatal("expected malformed conditions error")
	}
}

func TestKyvernoExactGVKRegistration(t *testing.T) {
	assessor := newAssessor(t)
	for _, version := range []string{"v1", "v2beta1"} {
		for _, kind := range []string{"Policy", "ClusterPolicy"} {
			got := assess(t, assessor, policy("kyverno.io/"+version, kind, nil))
			if got.Reconciliation == kubehealth.ReconciliationUnknown {
				t.Errorf("kyverno.io/%s %s was not registered", version, kind)
			}
		}
	}
	got := assess(t, assessor, policy("kyverno.io/v1beta1", "Policy", nil))
	if got.Reconciliation != kubehealth.ReconciliationUnknown {
		t.Error("unsupported kyverno.io/v1beta1 Policy was registered")
	}
}

func newAssessor(t *testing.T) *kubehealth.Assessor {
	t.Helper()
	assessor := kubehealth.NewAssessor()
	if err := kyverno.Register(assessor); err != nil {
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

func policy(apiVersion, kind string, conditions []any) *unstructured.Unstructured {
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

func readyCondition(status, reason, message string, observedGeneration int64) map[string]any {
	return condition("Ready", status, reason, message, observedGeneration)
}

func condition(conditionType, status, reason, message string, observedGeneration int64) map[string]any {
	result := map[string]any{"type": conditionType, "status": status, "reason": reason, "message": message}
	if observedGeneration >= 0 {
		result["observedGeneration"] = observedGeneration
	}
	return result
}
