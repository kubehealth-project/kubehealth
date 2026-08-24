package certmanager_test

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kubehealth-project/kubehealth"
	"github.com/kubehealth-project/kubehealth/integrations/certmanager"
)

func TestCertificate(t *testing.T) {
	tests := []struct {
		name, kind         string
		conditions         []any
		wantReconciliation kubehealth.ReconciliationStatus
		wantAvailability   kubehealth.AvailabilityStatus
	}{
		{
			name: "certificate ready", kind: "Certificate",
			conditions:         []any{condition("Ready", "True", "Issued", "Certificate is up to date")},
			wantReconciliation: kubehealth.ReconciliationReconciled,
			wantAvailability:   kubehealth.AvailabilityAvailable,
		},
		{
			name: "certificate issuing", kind: "Certificate",
			conditions:         []any{condition("Issuing", "True", "Renewing", "Renewing certificate"), condition("Ready", "True", "Issued", "Current certificate is valid")},
			wantReconciliation: kubehealth.ReconciliationInProgress,
			wantAvailability:   kubehealth.AvailabilityAvailable,
		},
		{
			name: "certificate failed", kind: "Certificate",
			conditions:         []any{condition("Ready", "False", "DoesNotExist", "Issuing certificate failed")},
			wantReconciliation: kubehealth.ReconciliationFailed,
			wantAvailability:   kubehealth.AvailabilityUnavailable,
		},
	}

	assessor := kubehealth.NewAssessor()
	if err := certmanager.Register(assessor); err != nil {
		t.Fatal(err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := assessor.Assess(resource("cert-manager.io/v1", tt.kind, tt.conditions))
			if err != nil {
				t.Fatal(err)
			}
			if got.Reconciliation != tt.wantReconciliation {
				t.Errorf("reconciliation = %q, want %q", got.Reconciliation, tt.wantReconciliation)
			}
			if got.Availability != tt.wantAvailability {
				t.Errorf("availability = %q, want %q", got.Availability, tt.wantAvailability)
			}
		})
	}
}

func resource(apiVersion, kind string, conditions []any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": apiVersion,
		"kind":       kind,
		"metadata":   map[string]any{"name": "example"},
		"status":     map[string]any{"conditions": conditions},
	}}
}

func condition(conditionType, status, reason, message string) map[string]any {
	return map[string]any{"type": conditionType, "status": status, "reason": reason, "message": message}
}
