package prometheusoperator_test

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kubehealth-project/kubehealth"
	"github.com/kubehealth-project/kubehealth/integrations/prometheusoperator"
)

func TestWorkload(t *testing.T) {
	tests := []struct {
		name         string
		generation   int64
		paused       bool
		conditions   []any
		wantRec      kubehealth.ReconciliationStatus
		wantAvail    kubehealth.AvailabilityStatus
		wantRecMsg   string
		wantAvailMsg string
	}{
		{
			name: "reconciled and available", generation: 3,
			conditions: []any{
				condition("Reconciled", "True", "Success", "Spec successfully applied", 3),
				condition("Available", "True", "", "All instances are available", 3),
			},
			wantRec: kubehealth.ReconciliationReconciled, wantAvail: kubehealth.AvailabilityAvailable,
			wantRecMsg: "Spec successfully applied", wantAvailMsg: "All instances are available",
		},
		{
			name: "reconciled but partially available", generation: 3,
			conditions: []any{
				condition("Reconciled", "True", "Success", "Desired configuration applied", 3),
				condition("Available", "Degraded", "SomePodsNotReady", "Some instances are not ready", 3),
			},
			wantRec: kubehealth.ReconciliationReconciled, wantAvail: kubehealth.AvailabilityPartiallyAvailable,
			wantRecMsg: "Desired configuration applied", wantAvailMsg: "Some instances are not ready",
		},
		{
			name: "reconciliation failed but previous instances remain available", generation: 3,
			conditions: []any{
				condition("Reconciled", "False", "InvalidConfiguration", "Could not apply new configuration", 3),
				condition("Available", "True", "", "Previous instances remain available", 3),
			},
			wantRec: kubehealth.ReconciliationFailed, wantAvail: kubehealth.AvailabilityAvailable,
			wantRecMsg: "Could not apply new configuration", wantAvailMsg: "Previous instances remain available",
		},
		{
			name: "unavailable", generation: 3,
			conditions: []any{
				condition("Reconciled", "True", "Success", "Spec applied", 3),
				condition("Available", "False", "NoPodsReady", "No instances are ready", 3),
			},
			wantRec: kubehealth.ReconciliationReconciled, wantAvail: kubehealth.AvailabilityUnavailable,
			wantRecMsg: "Spec applied", wantAvailMsg: "No instances are ready",
		},
		{
			name: "paused while still available", generation: 3, paused: true,
			conditions: []any{
				condition("Reconciled", "True", "Success", "Spec applied", 3),
				condition("Available", "True", "", "Instances are available", 3),
			},
			wantRec: kubehealth.ReconciliationSuspended, wantAvail: kubehealth.AvailabilityAvailable,
			wantRecMsg: "Prometheus reconciliation is paused", wantAvailMsg: "Instances are available",
		},
		{
			name: "legacy partial availability reason", generation: 3,
			conditions: []any{
				condition("Available", "False", "SomePodsNotReady", "Some instances are not ready", 3),
			},
			wantRec: kubehealth.ReconciliationUnknown, wantAvail: kubehealth.AvailabilityPartiallyAvailable,
			wantRecMsg: "Prometheus status does not report reconciliation", wantAvailMsg: "Some instances are not ready",
		},
		{
			name:    "waiting for initialization",
			wantRec: kubehealth.ReconciliationInProgress, wantAvail: kubehealth.AvailabilityUnknown,
			wantRecMsg: "Waiting for Prometheus status", wantAvailMsg: "Prometheus availability is not reported",
		},
		{
			name: "conditions do not report reconciliation", generation: 1,
			conditions: []any{condition("Available", "True", "", "Available", 1)},
			wantRec:    kubehealth.ReconciliationUnknown, wantAvail: kubehealth.AvailabilityAvailable,
			wantRecMsg: "Prometheus status does not report reconciliation", wantAvailMsg: "Available",
		},
		{
			name: "stale conditions", generation: 4,
			conditions: []any{
				condition("Reconciled", "True", "Success", "Old reconciliation", 3),
				condition("Available", "True", "", "Old availability", 3),
			},
			wantRec: kubehealth.ReconciliationInProgress, wantAvail: kubehealth.AvailabilityUnknown,
			wantRecMsg:   "Waiting for Prometheus status to observe the latest generation",
			wantAvailMsg: "Waiting for Prometheus availability status to observe the latest generation",
		},
		{
			name: "unknown condition statuses", generation: 1,
			conditions: []any{
				condition("Reconciled", "Unknown", "Pending", "Operator cannot determine reconciliation", 1),
				condition("Available", "Unknown", "Pending", "Operator cannot determine availability", 1),
			},
			wantRec: kubehealth.ReconciliationUnknown, wantAvail: kubehealth.AvailabilityUnknown,
			wantRecMsg: "Operator cannot determine reconciliation", wantAvailMsg: "Operator cannot determine availability",
		},
		{
			name: "duplicate conditions are ambiguous", generation: 1,
			conditions: []any{
				condition("Reconciled", "True", "Success", "Reconciled", 1),
				condition("Reconciled", "False", "Failed", "Failed", 1),
				condition("Available", "True", "", "Available", 1),
				condition("Available", "False", "", "Unavailable", 1),
			},
			wantRec: kubehealth.ReconciliationUnknown, wantAvail: kubehealth.AvailabilityUnknown,
			wantRecMsg: "Prometheus has ambiguous Reconciled conditions", wantAvailMsg: "Prometheus has ambiguous Available conditions",
		},
	}

	assessor := newAssessor(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := resource("Prometheus", tt.conditions)
			obj.SetGeneration(tt.generation)
			if tt.paused {
				unstructured.SetNestedField(obj.Object, true, "spec", "paused")
			}
			got, err := assessor.Assess(obj)
			if err != nil {
				t.Fatal(err)
			}
			if got.Reconciliation.Status != tt.wantRec {
				t.Errorf("reconciliation = %q, want %q", got.Reconciliation.Status, tt.wantRec)
			}
			if got.Availability.Status != tt.wantAvail {
				t.Errorf("availability = %q, want %q", got.Availability.Status, tt.wantAvail)
			}
			if got.Reconciliation.Message != tt.wantRecMsg {
				t.Errorf("reconciliation message = %q, want %q", got.Reconciliation.Message, tt.wantRecMsg)
			}
			if got.Availability.Message != tt.wantAvailMsg {
				t.Errorf("availability message = %q, want %q", got.Availability.Message, tt.wantAvailMsg)
			}
			if got.Lifecycle.Status != kubehealth.LifecycleActive {
				t.Errorf("lifecycle = %q, want %q", got.Lifecycle.Status, kubehealth.LifecycleActive)
			}
		})
	}
}

func TestWorkloadExactGVKRegistration(t *testing.T) {
	assessor := newAssessor(t)
	for _, kind := range []string{"Prometheus", "Alertmanager", "ThanosRuler"} {
		got, err := assessor.Assess(resource(kind, nil))
		if err != nil {
			t.Errorf("Assess(%s): %v", kind, err)
			continue
		}
		if got.Reconciliation.Status == kubehealth.ReconciliationUnknown {
			t.Errorf("monitoring.coreos.com/v1 %s was not registered", kind)
		}
	}
	for _, apiVersion := range []string{"monitoring.coreos.com/v1alpha1", "monitoring.coreos.com/v1beta1"} {
		obj := resource("Prometheus", nil)
		obj.SetAPIVersion(apiVersion)
		got, err := assessor.Assess(obj)
		if err != nil {
			t.Errorf("Assess(%s): %v", apiVersion, err)
			continue
		}
		if got.Reconciliation.Status != kubehealth.ReconciliationUnknown {
			t.Errorf("unexpectedly registered %s Prometheus", apiVersion)
		}
	}
}

func TestWorkloadKinds(t *testing.T) {
	assessor := newAssessor(t)
	for _, kind := range []string{"Prometheus", "Alertmanager", "ThanosRuler"} {
		t.Run(kind, func(t *testing.T) {
			obj := resource(kind, []any{
				condition("Reconciled", "True", "Success", "Spec applied", 1),
				condition("Available", "True", "", "All instances are available", 1),
			})
			obj.SetGeneration(1)
			got, err := assessor.Assess(obj)
			if err != nil {
				t.Fatal(err)
			}
			if got.Reconciliation.Status != kubehealth.ReconciliationReconciled {
				t.Errorf("reconciliation = %q, want %q", got.Reconciliation.Status, kubehealth.ReconciliationReconciled)
			}
			if got.Availability.Status != kubehealth.AvailabilityAvailable {
				t.Errorf("availability = %q, want %q", got.Availability.Status, kubehealth.AvailabilityAvailable)
			}
		})
	}
}

func TestWorkloadInvalidConditions(t *testing.T) {
	assessor := newAssessor(t)
	obj := resource("Prometheus", "not-an-array")
	if _, err := assessor.Assess(obj); err == nil {
		t.Fatal("Assess() error = nil, want invalid status.conditions error")
	}
}

func newAssessor(t *testing.T) *kubehealth.Assessor {
	t.Helper()
	assessor := kubehealth.NewAssessor()
	if err := prometheusoperator.Register(assessor); err != nil {
		t.Fatal(err)
	}
	return assessor
}

func resource(kind string, conditions any) *unstructured.Unstructured {
	status := map[string]any{}
	if conditions != nil {
		status["conditions"] = conditions
	}
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "monitoring.coreos.com/v1", "kind": kind,
		"metadata": map[string]any{"name": "example"}, "status": status,
	}}
}

func condition(conditionType, status, reason, message string, observedGeneration int64) map[string]any {
	return map[string]any{
		"type": conditionType, "status": status, "reason": reason, "message": message,
		"observedGeneration": observedGeneration,
	}
}
