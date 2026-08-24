package keda_test

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kubehealth-project/kubehealth"
	"github.com/kubehealth-project/kubehealth/integrations/keda"
)

func TestScaledObject(t *testing.T) {
	tests := []struct {
		name       string
		conditions []any
		want       kubehealth.ReconciliationStatus
	}{
		{name: "ready", conditions: []any{condition("Ready", "True", "ScaledObjectReady", "Ready for scaling")}, want: kubehealth.ReconciliationReconciled},
		{name: "invalid", conditions: []any{condition("Ready", "False", "ScaledObjectCheckFailed", "Invalid trigger")}, want: kubehealth.ReconciliationFailed},
		{name: "fallback", conditions: []any{condition("Ready", "True", "ScaledObjectReady", "Ready"), condition("Fallback", "True", "FallbackExists", "Using fallback")}, want: kubehealth.ReconciliationFailed},
		{name: "waiting", want: kubehealth.ReconciliationInProgress},
	}

	assessor := kubehealth.NewAssessor()
	if err := keda.Register(assessor); err != nil {
		t.Fatal(err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "keda.sh/v1alpha1", "kind": "ScaledObject",
				"metadata": map[string]any{"name": "example"},
				"status":   map[string]any{"conditions": tt.conditions},
			}}
			got, err := assessor.Assess(obj)
			if err != nil {
				t.Fatal(err)
			}
			if got.Reconciliation != tt.want {
				t.Errorf("reconciliation = %q, want %q", got.Reconciliation, tt.want)
			}
			if got.Availability != kubehealth.AvailabilityNotApplicable {
				t.Errorf("availability = %q", got.Availability)
			}
		})
	}
}

func condition(conditionType, status, reason, message string) map[string]any {
	return map[string]any{"type": conditionType, "status": status, "reason": reason, "message": message}
}
