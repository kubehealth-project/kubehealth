package externalsecrets_test

import (
	"testing"

	"github.com/kubehealth-project/kubehealth"
)

func TestClusterSecretStore(t *testing.T) {
	testSecretStore(t, "ClusterSecretStore")
}

func TestClusterSecretStoreTerminatingCanRemainAvailable(t *testing.T) {
	obj := resource("external-secrets.io/v1", "ClusterSecretStore", conditions(condition("Ready", "True", "Valid", "store validated")))
	markTerminating(obj)
	got, err := newAssessor(t).Assess(obj)
	if err != nil {
		t.Fatal(err)
	}
	if got.Reconciliation != kubehealth.ReconciliationUnknown || got.Availability != kubehealth.AvailabilityAvailable || got.Lifecycle != kubehealth.LifecycleTerminating {
		t.Fatalf("assessment = %#v", got)
	}
}
