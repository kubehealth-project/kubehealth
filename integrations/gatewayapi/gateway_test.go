package gatewayapi_test

import (
	"testing"

	"github.com/kubehealth-project/kubehealth"
)

func TestGateway(t *testing.T) {
	tests := []struct {
		name           string
		generation     int64
		top            []any
		listeners      []any
		reconciliation kubehealth.ReconciliationStatus
		availability   kubehealth.AvailabilityStatus
	}{
		{
			name: "all listeners programmed", generation: 2,
			top:            []any{condition("Accepted", "True", "Accepted", 2), condition("Programmed", "True", "Programmed", 2)},
			listeners:      []any{listener("http", 2, "True", "True"), listener("https", 2, "True", "True")},
			reconciliation: kubehealth.ReconciliationReconciled, availability: kubehealth.AvailabilityAvailable,
		},
		{
			name: "one invalid listener is partially available", generation: 2,
			top:            []any{condition("Accepted", "True", "ListenersNotValid", 2), condition("Programmed", "True", "Programmed", 2)},
			listeners:      []any{listener("http", 2, "True", "True"), listener("https", 2, "False", "False")},
			reconciliation: kubehealth.ReconciliationFailed, availability: kubehealth.AvailabilityPartiallyAvailable,
		},
		{
			name: "pending programming", generation: 2,
			top:            []any{condition("Accepted", "True", "Accepted", 2), condition("Programmed", "False", "Pending", 2)},
			listeners:      []any{listener("http", 2, "True", "False"), listener("https", 2, "True", "False")},
			reconciliation: kubehealth.ReconciliationInProgress, availability: kubehealth.AvailabilityUnavailable,
		},
		{
			name: "stale programmed configuration remains available", generation: 2,
			top:            []any{condition("Accepted", "True", "Accepted", 1), condition("Programmed", "True", "Programmed", 1)},
			listeners:      []any{listener("http", 1, "True", "True"), listener("https", 1, "True", "True")},
			reconciliation: kubehealth.ReconciliationInProgress, availability: kubehealth.AvailabilityAvailable,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			specListeners := []any{
				map[string]any{"name": "http", "protocol": "HTTP", "port": int64(80)},
				map[string]any{"name": "https", "protocol": "HTTPS", "port": int64(443)},
			}
			got := assess(t, resource("Gateway", tt.generation, map[string]any{"gatewayClassName": "example", "listeners": specListeners}, map[string]any{
				"conditions": tt.top, "listeners": tt.listeners,
			}))
			assertAssessment(t, got, tt.reconciliation, tt.availability)
		})
	}
}

func TestGatewayMissingDesiredListenerStatus(t *testing.T) {
	got := assess(t, resource("Gateway", 1, map[string]any{
		"gatewayClassName": "example",
		"listeners":        []any{map[string]any{"name": "http"}, map[string]any{"name": "https"}},
	}, map[string]any{
		"conditions": []any{condition("Accepted", "True", "Accepted", 1), condition("Programmed", "True", "Programmed", 1)},
		"listeners":  []any{listener("http", 1, "True", "True")},
	}))
	assertAssessment(t, got, kubehealth.ReconciliationInProgress, kubehealth.AvailabilityUnknown)
}

func listener(name string, generation int64, accepted, programmed string) map[string]any {
	return map[string]any{
		"name": name,
		"conditions": []any{
			condition("Accepted", accepted, map[bool]string{true: "Accepted", false: "Invalid"}[accepted == "True"], generation),
			condition("ResolvedRefs", "True", "ResolvedRefs", generation),
			condition("Programmed", programmed, map[bool]string{true: "Programmed", false: "Pending"}[programmed == "True"], generation),
		},
	}
}
