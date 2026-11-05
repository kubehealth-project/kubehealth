package kstatus_test

import (
	"testing"

	"github.com/kubehealth-project/kubehealth"
	"github.com/kubehealth-project/kubehealth/kstatus"
	status "sigs.k8s.io/cli-utils/pkg/kstatus/status"
)

func TestProject(t *testing.T) {
	tests := []struct {
		name       string
		assessment kubehealth.Assessment
		want       status.Status
	}{
		{name: "reconciled", assessment: assessment(kubehealth.ReconciliationReconciled, kubehealth.LifecycleActive), want: status.CurrentStatus},
		{name: "in progress", assessment: assessment(kubehealth.ReconciliationInProgress, kubehealth.LifecycleActive), want: status.InProgressStatus},
		{name: "failed", assessment: assessment(kubehealth.ReconciliationFailed, kubehealth.LifecycleActive), want: status.FailedStatus},
		{name: "suspended", assessment: assessment(kubehealth.ReconciliationSuspended, kubehealth.LifecycleActive), want: status.InProgressStatus},
		{name: "initializing", assessment: assessment("", kubehealth.LifecycleInitializing), want: status.InProgressStatus},
		{name: "terminating", assessment: assessment("", kubehealth.LifecycleTerminating), want: status.TerminatingStatus},
		{name: "not found", assessment: assessment("", kubehealth.LifecycleNotFound), want: status.NotFoundStatus},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := kstatus.Project(test.assessment); got.Status != test.want {
				t.Fatalf("status = %q, want %q", got.Status, test.want)
			}
		})
	}
}

func assessment(reconciliation kubehealth.ReconciliationStatus, lifecycle kubehealth.LifecycleStatus) kubehealth.Assessment {
	return kubehealth.Assessment{
		Reconciliation: kubehealth.Dimension[kubehealth.ReconciliationStatus]{Status: reconciliation},
		Lifecycle:      kubehealth.Dimension[kubehealth.LifecycleStatus]{Status: lifecycle},
	}
}
