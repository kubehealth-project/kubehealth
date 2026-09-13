package externalsecrets_test

import (
	"testing"

	"github.com/kubehealth-project/kubehealth"
)

func TestExternalSecret(t *testing.T) {
	tests := []struct {
		name         string
		status       map[string]any
		reconcile    kubehealth.ReconciliationStatus
		availability kubehealth.AvailabilityStatus
	}{
		{name: "secret synced", status: conditions(condition("Ready", "True", "SecretSynced", "secret synced")), reconcile: kubehealth.ReconciliationReconciled, availability: kubehealth.AvailabilityAvailable},
		{name: "generic resource synced", status: conditions(condition("Ready", "True", "ResourceSynced", "resource synced")), reconcile: kubehealth.ReconciliationReconciled, availability: kubehealth.AvailabilityAvailable},
		{name: "provider failure", status: conditions(condition("Ready", "False", "SecretSyncedError", "could not get secret data")), reconcile: kubehealth.ReconciliationFailed, availability: kubehealth.AvailabilityUnknown},
		{name: "immutable target", status: conditions(condition("Ready", "False", "SecretImmutable", "target is immutable")), reconcile: kubehealth.ReconciliationFailed, availability: kubehealth.AvailabilityUnknown},
		{name: "target owned by another resource", status: conditions(condition("Ready", "False", "SecretOwnedByOther", "target has another owner")), reconcile: kubehealth.ReconciliationFailed, availability: kubehealth.AvailabilityUnknown},
		{name: "secret deliberately deleted", status: conditions(condition("Ready", "True", "SecretDeleted", "secret deleted")), reconcile: kubehealth.ReconciliationReconciled, availability: kubehealth.AvailabilityUnavailable},
		{name: "resource deliberately deleted", status: conditions(condition("Ready", "True", "ResourceDeleted", "resource deleted")), reconcile: kubehealth.ReconciliationReconciled, availability: kubehealth.AvailabilityUnavailable},
		{name: "merge target missing", status: conditions(condition("Ready", "True", "SecretMissing", "secret will not be created")), reconcile: kubehealth.ReconciliationReconciled, availability: kubehealth.AvailabilityUnavailable},
		{name: "generic merge target missing", status: conditions(condition("Ready", "True", "ResourceMissing", "resource will not be created")), reconcile: kubehealth.ReconciliationReconciled, availability: kubehealth.AvailabilityUnavailable},
		{name: "not reconciled yet", reconcile: kubehealth.ReconciliationInProgress, availability: kubehealth.AvailabilityUnknown},
		{name: "ready explicitly unknown", status: conditions(condition("Ready", "Unknown", "ProviderUncertain", "provider status unknown")), reconcile: kubehealth.ReconciliationUnknown, availability: kubehealth.AvailabilityUnknown},
		{name: "unrelated condition only", status: conditions(condition("Deleted", "False", "SecretExists", "target remains")), reconcile: kubehealth.ReconciliationInProgress, availability: kubehealth.AvailabilityUnknown},
		{name: "duplicate ready conditions", status: conditions(condition("Ready", "True", "SecretSynced", "synced"), condition("Ready", "False", "SecretSyncedError", "failed")), reconcile: kubehealth.ReconciliationUnknown, availability: kubehealth.AvailabilityUnknown},
	}

	assessor := newAssessor(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := assessor.Assess(resource("external-secrets.io/v1", "ExternalSecret", tt.status))
			if err != nil {
				t.Fatal(err)
			}
			assertAssessment(t, got, tt.reconcile, tt.availability)
		})
	}
}

func TestExternalSecretFailedRefreshDoesNotAssumePriorTargetAvailable(t *testing.T) {
	status := conditions(condition("Ready", "False", "SecretSyncedError", "refresh failed"))
	status["refreshTime"] = "2026-08-24T00:00:00Z"
	status["syncedResourceVersion"] = "17-abc"
	status["binding"] = map[string]any{"name": "example"}

	got, err := newAssessor(t).Assess(resource("external-secrets.io/v1", "ExternalSecret", status))
	if err != nil {
		t.Fatal(err)
	}
	assertAssessment(t, got, kubehealth.ReconciliationFailed, kubehealth.AvailabilityUnknown)
}

func TestExternalSecretTerminatingPreservesAvailability(t *testing.T) {
	obj := resource("external-secrets.io/v1", "ExternalSecret", conditions(condition("Ready", "True", "SecretSynced", "synced")))
	markTerminating(obj)
	got, err := newAssessor(t).Assess(obj)
	if err != nil {
		t.Fatal(err)
	}
	if got.Reconciliation.Status != kubehealth.ReconciliationUnknown || got.Availability.Status != kubehealth.AvailabilityAvailable || got.Lifecycle.Status != kubehealth.LifecycleTerminating {
		t.Fatalf("assessment = %#v", got)
	}
}

func TestExternalSecretMalformedConditions(t *testing.T) {
	_, err := newAssessor(t).Assess(resource("external-secrets.io/v1", "ExternalSecret", map[string]any{"conditions": "invalid"}))
	if err == nil {
		t.Fatal("expected malformed conditions error")
	}
}
