package certmanager_test

import (
	"testing"

	"github.com/kubehealth-project/kubehealth"
)

func TestIssuer(t *testing.T) {
	testIssuer(t, "Issuer")
}

func testIssuer(t *testing.T, kind string) {
	tests := []struct {
		name       string
		conditions []any
		generation int64
		wantRec    kubehealth.ReconciliationStatus
		wantAvail  kubehealth.AvailabilityStatus
	}{
		{name: "ready", conditions: conditions(condition("Ready", "True", "ACMEAccountRegistered", "Account registered", 1)), generation: 1, wantRec: kubehealth.ReconciliationReconciled, wantAvail: kubehealth.AvailabilityAvailable},
		{name: "failed", conditions: conditions(condition("Ready", "False", "ErrVerifyACMEAccount", "Account verification failed", 1)), generation: 1, wantRec: kubehealth.ReconciliationFailed, wantAvail: kubehealth.AvailabilityUnavailable},
		{name: "unknown", conditions: conditions(condition("Ready", "Unknown", "Pending", "State unknown", 1)), generation: 1, wantRec: kubehealth.ReconciliationUnknown, wantAvail: kubehealth.AvailabilityUnknown},
		{name: "initializing", wantRec: kubehealth.ReconciliationInProgress, wantAvail: kubehealth.AvailabilityUnknown},
		{name: "unrelated condition", conditions: conditions(condition("Other", "True", "Observed", "Observed", 1)), generation: 1, wantRec: kubehealth.ReconciliationUnknown, wantAvail: kubehealth.AvailabilityUnknown},
		{name: "stale ready", conditions: conditions(condition("Ready", "True", "Ready", "Old readiness", 2)), generation: 3, wantRec: kubehealth.ReconciliationInProgress, wantAvail: kubehealth.AvailabilityAvailable},
		{name: "stale failed", conditions: conditions(condition("Ready", "False", "Failed", "Old failure", 2)), generation: 3, wantRec: kubehealth.ReconciliationInProgress, wantAvail: kubehealth.AvailabilityUnavailable},
		{name: "ambiguous", conditions: conditions(condition("Ready", "True", "Ready", "Ready", 1), condition("Ready", "False", "Failed", "Failed", 1)), generation: 1, wantRec: kubehealth.ReconciliationUnknown, wantAvail: kubehealth.AvailabilityUnknown},
	}

	assessor := newAssessor(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := resource("cert-manager.io/v1", kind, tt.conditions)
			obj.SetGeneration(tt.generation)
			got := assess(t, assessor, obj)
			assertAssessment(t, got, tt.wantRec, tt.wantAvail, kubehealth.LifecycleActive)
		})
	}
}

func TestCertManagerExactGVKRegistration(t *testing.T) {
	assessor := newAssessor(t)
	for _, kind := range []string{"Certificate", "Issuer", "ClusterIssuer"} {
		got := assess(t, assessor, resource("cert-manager.io/v1", kind, nil))
		if got.Reconciliation == kubehealth.ReconciliationUnknown {
			t.Errorf("cert-manager.io/v1 %s was not registered", kind)
		}
	}
	for _, apiVersion := range []string{"cert-manager.io/v1alpha2", "certmanager.k8s.io/v1alpha1"} {
		got := assess(t, assessor, resource(apiVersion, "Certificate", nil))
		if got.Reconciliation != kubehealth.ReconciliationUnknown {
			t.Errorf("historical %s Certificate was registered", apiVersion)
		}
	}
}
