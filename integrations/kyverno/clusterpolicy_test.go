package kyverno_test

import (
	"testing"

	"github.com/kubehealth-project/kubehealth"
	"github.com/kubehealth-project/kubehealth/integrations/kyverno"
)

func TestClusterPolicy(t *testing.T) {
	assessor := kubehealth.NewAssessor()
	if err := kyverno.Register(assessor); err != nil {
		t.Fatal(err)
	}

	got := assess(t, assessor, policy("ClusterPolicy", []any{readyCondition("True", "Succeeded", "Ready")}))
	if got.Reconciliation != kubehealth.ReconciliationReconciled || got.Availability != kubehealth.AvailabilityAvailable {
		t.Fatalf("assessment = %#v", got)
	}

	got = assess(t, assessor, policy("ClusterPolicy", nil))
	if got.Reconciliation != kubehealth.ReconciliationInProgress || got.Availability != kubehealth.AvailabilityUnavailable {
		t.Fatalf("assessment = %#v", got)
	}
}
