package gatewayapi_test

import (
	"testing"

	"github.com/kubehealth-project/kubehealth"
)

func TestBackendTLSPolicy(t *testing.T) {
	tests := []struct {
		name           string
		generation     int64
		ancestors      []any
		reconciliation kubehealth.ReconciliationStatus
		availability   kubehealth.AvailabilityStatus
	}{
		{
			name: "accepted", generation: 3,
			ancestors:      []any{ancestor("gateway-a", []any{condition("Accepted", "True", "Accepted", 3), condition("ResolvedRefs", "True", "ResolvedRefs", 3)})},
			reconciliation: kubehealth.ReconciliationReconciled, availability: kubehealth.AvailabilityAvailable,
		},
		{
			name: "mixed ancestors", generation: 3,
			ancestors: []any{
				ancestor("gateway-a", []any{condition("Accepted", "True", "Accepted", 3), condition("ResolvedRefs", "True", "ResolvedRefs", 3)}),
				ancestor("gateway-b", []any{condition("Accepted", "False", "Conflicted", 3), condition("ResolvedRefs", "True", "ResolvedRefs", 3)}),
			},
			reconciliation: kubehealth.ReconciliationFailed, availability: kubehealth.AvailabilityPartiallyAvailable,
		},
		{
			name: "unresolved certificate", generation: 3,
			ancestors:      []any{ancestor("gateway-a", []any{condition("Accepted", "True", "Accepted", 3), condition("ResolvedRefs", "False", "InvalidCACertificateRef", 3)})},
			reconciliation: kubehealth.ReconciliationFailed, availability: kubehealth.AvailabilityUnavailable,
		},
		{
			name: "healthy and stale ancestors", generation: 3,
			ancestors: []any{
				ancestor("gateway-a", []any{condition("Accepted", "True", "Accepted", 3), condition("ResolvedRefs", "True", "ResolvedRefs", 3)}),
				ancestor("gateway-b", []any{condition("Accepted", "True", "Accepted", 2), condition("ResolvedRefs", "True", "ResolvedRefs", 2)}),
			},
			reconciliation: kubehealth.ReconciliationInProgress, availability: kubehealth.AvailabilityAvailable,
		},
		{
			name: "no ancestors", generation: 3, ancestors: nil,
			reconciliation: kubehealth.ReconciliationInProgress, availability: kubehealth.AvailabilityUnknown,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := assess(t, resource("BackendTLSPolicy", tt.generation, map[string]any{
				"targetRefs": []any{map[string]any{"group": "", "kind": "Service", "name": "backend"}},
				"validation": map[string]any{"hostname": "backend.example.com", "wellKnownCACertificates": "System"},
			}, map[string]any{"ancestors": tt.ancestors}))
			assertAssessment(t, got, tt.reconciliation, tt.availability)
		})
	}
}

func TestBackendTLSPolicyUnknownAcceptance(t *testing.T) {
	got := assess(t, resource("BackendTLSPolicy", 1, map[string]any{
		"targetRefs": []any{map[string]any{"group": "", "kind": "Service", "name": "backend"}},
		"validation": map[string]any{"hostname": "backend.example.com", "wellKnownCACertificates": "System"},
	}, map[string]any{"ancestors": []any{
		ancestor("gateway-a", []any{condition("Accepted", "Unknown", "Pending", 1)}),
	}}))
	assertAssessment(t, got, kubehealth.ReconciliationInProgress, kubehealth.AvailabilityUnknown)
}

func ancestor(name string, conditions []any) map[string]any {
	return map[string]any{
		"ancestorRef":    map[string]any{"name": name},
		"controllerName": "example.io/controller", "conditions": conditions,
	}
}
