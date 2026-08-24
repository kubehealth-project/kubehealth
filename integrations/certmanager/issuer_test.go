package certmanager_test

import (
	"testing"

	"github.com/kubehealth-project/kubehealth"
	"github.com/kubehealth-project/kubehealth/integrations/certmanager"
)

func TestIssuer(t *testing.T) {
	tests := []struct {
		name               string
		conditions         []any
		wantReconciliation kubehealth.ReconciliationStatus
		wantAvailability   kubehealth.AvailabilityStatus
	}{
		{name: "ready", conditions: []any{condition("Ready", "True", "ACMEAccountRegistered", "Account registered")}, wantReconciliation: kubehealth.ReconciliationReconciled, wantAvailability: kubehealth.AvailabilityAvailable},
		{name: "failed", conditions: []any{condition("Ready", "False", "ErrVerifyACMEAccount", "Account verification failed")}, wantReconciliation: kubehealth.ReconciliationFailed, wantAvailability: kubehealth.AvailabilityUnavailable},
		{name: "initializing", wantReconciliation: kubehealth.ReconciliationInProgress, wantAvailability: kubehealth.AvailabilityUnknown},
	}

	assessor := kubehealth.NewAssessor()
	if err := certmanager.Register(assessor); err != nil {
		t.Fatal(err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := assessor.Assess(resource("cert-manager.io/v1", "Issuer", tt.conditions))
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
