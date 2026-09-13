package externalsecrets_test

import (
	"testing"

	"github.com/kubehealth-project/kubehealth"
)

func TestPushSecret(t *testing.T) {
	tests := []struct {
		name         string
		status       map[string]any
		reconcile    kubehealth.ReconciliationStatus
		availability kubehealth.AvailabilityStatus
	}{
		{name: "synced", status: conditions(condition("Ready", "True", "Synced", "PushSecret synced successfully")), reconcile: kubehealth.ReconciliationReconciled, availability: kubehealth.AvailabilityAvailable},
		{name: "push failed", status: conditions(condition("Ready", "False", "Errored", "set secret failed")), reconcile: kubehealth.ReconciliationFailed, availability: kubehealth.AvailabilityUnknown},
		{name: "failed after previous synchronization", status: pushStatus("Errored", map[string]any{"SecretStore/store": map[string]any{"remote": map[string]any{}}}), reconcile: kubehealth.ReconciliationFailed, availability: kubehealth.AvailabilityUnknown},
		{name: "source deleted and provider cleaned", status: pushStatus("SourceDeleted", map[string]any{}), reconcile: kubehealth.ReconciliationReconciled, availability: kubehealth.AvailabilityUnavailable},
		{name: "source deleted conflicts with synchronized state", status: pushStatus("SourceDeleted", map[string]any{"SecretStore/store": map[string]any{"remote": map[string]any{}}}), reconcile: kubehealth.ReconciliationReconciled, availability: kubehealth.AvailabilityUnknown},
		{name: "initializing", reconcile: kubehealth.ReconciliationInProgress, availability: kubehealth.AvailabilityUnknown},
		{name: "ready unknown", status: conditions(condition("Ready", "Unknown", "Checking", "push status unknown")), reconcile: kubehealth.ReconciliationUnknown, availability: kubehealth.AvailabilityUnknown},
		{name: "duplicate ready", status: conditions(condition("Ready", "True", "Synced", "synced"), condition("Ready", "False", "Errored", "failed")), reconcile: kubehealth.ReconciliationUnknown, availability: kubehealth.AvailabilityUnknown},
	}
	assessor := newAssessor(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := assessor.Assess(resource("external-secrets.io/v1alpha1", "PushSecret", tt.status))
			if err != nil {
				t.Fatal(err)
			}
			assertAssessment(t, got, tt.reconcile, tt.availability)
		})
	}
}

func TestPushSecretTerminatingRetainsReportedAvailability(t *testing.T) {
	obj := resource("external-secrets.io/v1alpha1", "PushSecret", conditions(condition("Ready", "True", "Synced", "synced")))
	markTerminating(obj)
	got, err := newAssessor(t).Assess(obj)
	if err != nil {
		t.Fatal(err)
	}
	if got.Reconciliation.Status != kubehealth.ReconciliationUnknown || got.Availability.Status != kubehealth.AvailabilityAvailable || got.Lifecycle.Status != kubehealth.LifecycleTerminating {
		t.Fatalf("assessment = %#v", got)
	}
}

func pushStatus(reason string, synced map[string]any) map[string]any {
	status := conditions(condition("Ready", "False", reason, "push did not complete normally"))
	status["syncedPushSecrets"] = synced
	return status
}
