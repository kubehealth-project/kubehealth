package externalsecrets_test

import (
	"testing"

	"github.com/kubehealth-project/kubehealth"
)

func TestSecretStore(t *testing.T) {
	testSecretStore(t, "SecretStore")
}

func testSecretStore(t *testing.T, kind string) {
	t.Helper()
	tests := []struct {
		name         string
		status       map[string]any
		reconcile    kubehealth.ReconciliationStatus
		availability kubehealth.AvailabilityStatus
	}{
		{name: "valid", status: conditions(condition("Ready", "True", "Valid", "store validated")), reconcile: kubehealth.ReconciliationReconciled, availability: kubehealth.AvailabilityAvailable},
		{name: "invalid configuration", status: conditions(condition("Ready", "False", "InvalidStoreConfiguration", "invalid store")), reconcile: kubehealth.ReconciliationFailed, availability: kubehealth.AvailabilityUnavailable},
		{name: "invalid provider", status: conditions(condition("Ready", "False", "InvalidProviderConfig", "invalid provider")), reconcile: kubehealth.ReconciliationFailed, availability: kubehealth.AvailabilityUnavailable},
		{name: "provider not found", status: conditions(condition("Ready", "False", "ProviderNotFound", "provider not found")), reconcile: kubehealth.ReconciliationFailed, availability: kubehealth.AvailabilityUnavailable},
		{name: "validation failed", status: conditions(condition("Ready", "False", "ValidationFailed", "unable to validate")), reconcile: kubehealth.ReconciliationFailed, availability: kubehealth.AvailabilityUnavailable},
		{name: "validation unknown", status: conditions(condition("Ready", "True", "ValidationUnknown", "could not determine validation status")), reconcile: kubehealth.ReconciliationReconciled, availability: kubehealth.AvailabilityUnknown},
		{name: "initializing", reconcile: kubehealth.ReconciliationInProgress, availability: kubehealth.AvailabilityUnknown},
		{name: "ready unknown", status: conditions(condition("Ready", "Unknown", "Checking", "validation pending")), reconcile: kubehealth.ReconciliationUnknown, availability: kubehealth.AvailabilityUnknown},
		{name: "duplicate ready", status: conditions(condition("Ready", "True", "Valid", "valid"), condition("Ready", "False", "ValidationFailed", "failed")), reconcile: kubehealth.ReconciliationUnknown, availability: kubehealth.AvailabilityUnknown},
	}
	assessor := newAssessor(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := assessor.Assess(resource("external-secrets.io/v1", kind, tt.status))
			if err != nil {
				t.Fatal(err)
			}
			assertAssessment(t, got, tt.reconcile, tt.availability)
		})
	}
}

func TestSecretStoreCapabilitiesDoNotDetermineHealth(t *testing.T) {
	for _, capability := range []string{"ReadOnly", "WriteOnly", "ReadWrite"} {
		t.Run(capability, func(t *testing.T) {
			status := conditions(condition("Ready", "True", "Valid", "store validated"))
			status["capabilities"] = capability
			got, err := newAssessor(t).Assess(resource("external-secrets.io/v1", "SecretStore", status))
			if err != nil {
				t.Fatal(err)
			}
			assertAssessment(t, got, kubehealth.ReconciliationReconciled, kubehealth.AvailabilityAvailable)
		})
	}
}
