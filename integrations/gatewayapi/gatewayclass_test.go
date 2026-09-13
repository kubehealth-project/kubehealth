package gatewayapi_test

import (
	"testing"

	"github.com/kubehealth-project/kubehealth"
	"github.com/kubehealth-project/kubehealth/integrations/gatewayapi"
)

func TestGatewayClass(t *testing.T) {
	tests := []struct {
		name           string
		generation     int64
		conditions     []any
		reconciliation kubehealth.ReconciliationStatus
		availability   kubehealth.AvailabilityStatus
	}{
		{"accepted", 2, []any{condition("Accepted", "True", "Accepted", 2)}, kubehealth.ReconciliationReconciled, kubehealth.AvailabilityAvailable},
		{"rejected", 2, []any{condition("Accepted", "False", "InvalidParameters", 2)}, kubehealth.ReconciliationFailed, kubehealth.AvailabilityUnavailable},
		{"pending", 2, []any{condition("Accepted", "Unknown", "Pending", 2)}, kubehealth.ReconciliationInProgress, kubehealth.AvailabilityUnknown},
		{"stale accepted remains available", 2, []any{condition("Accepted", "True", "Accepted", 1)}, kubehealth.ReconciliationInProgress, kubehealth.AvailabilityAvailable},
		{"future generation is current", 2, []any{condition("Accepted", "True", "Accepted", 3)}, kubehealth.ReconciliationReconciled, kubehealth.AvailabilityAvailable},
		{"missing status", 2, nil, kubehealth.ReconciliationInProgress, kubehealth.AvailabilityUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var status map[string]any
			if tt.conditions != nil {
				status = map[string]any{"conditions": tt.conditions}
			}
			got := assess(t, resource("GatewayClass", tt.generation, map[string]any{"controllerName": "example.io/controller"}, status))
			assertAssessment(t, got, tt.reconciliation, tt.availability)
		})
	}
}

func TestGatewayClassRejectsMalformedConditions(t *testing.T) {
	assessor := kubehealth.NewAssessor()
	if err := gatewayapi.Register(assessor); err != nil {
		t.Fatal(err)
	}
	_, err := assessor.Assess(resource("GatewayClass", 1, map[string]any{"controllerName": "example.io/controller"}, map[string]any{
		"conditions": map[string]any{"type": "Accepted"},
	}))
	if err == nil {
		t.Fatal("expected malformed conditions to return an error")
	}
}
