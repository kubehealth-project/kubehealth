package builtins_test

import (
	"testing"

	"github.com/kubehealth-project/kubehealth"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestHPA(t *testing.T) {
	tests := []struct {
		name       string
		apiVersion string
		conditions []any
		want       kubehealth.ReconciliationStatus
	}{
		{name: "v1 lacks standardized health conditions", apiVersion: "autoscaling/v1", want: kubehealth.ReconciliationUnknown},
		{name: "v2 healthy", apiVersion: "autoscaling/v2", conditions: []any{hpaCondition("AbleToScale", "True", "SucceededGetScale", "ready")}, want: kubehealth.ReconciliationReconciled},
		{name: "v2 failed", apiVersion: "autoscaling/v2", conditions: []any{hpaCondition("ScalingActive", "False", "FailedGetExternalMetric", "failed")}, want: kubehealth.ReconciliationFailed},
		{name: "v2 unknown failure type", apiVersion: "autoscaling/v2", conditions: []any{hpaCondition("Other", "False", "FailedGetScale", "ignored")}, want: kubehealth.ReconciliationInProgress},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			obj := &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": test.apiVersion,
				"kind":       "HorizontalPodAutoscaler",
				"metadata":   map[string]any{"name": "autoscaler"},
				"status":     map[string]any{"conditions": test.conditions},
			}}
			assessment, err := newBuiltinsAssessor(t).Assess(obj)
			if err != nil {
				t.Fatal(err)
			}
			if assessment.Reconciliation.Status != test.want {
				t.Fatalf("reconciliation = %q, want %q", assessment.Reconciliation, test.want)
			}
			if assessment.Availability.Status != kubehealth.AvailabilityNotApplicable {
				t.Fatalf("availability = %q, want %q", assessment.Availability, kubehealth.AvailabilityNotApplicable)
			}
		})
	}
}

func hpaCondition(conditionType, status, reason, message string) map[string]any {
	return map[string]any{"type": conditionType, "status": status, "reason": reason, "message": message}
}
