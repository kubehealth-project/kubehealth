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
		{name: "reconciled", assessment: kubehealth.Assessment{Lifecycle: kubehealth.LifecycleActive, Reconciliation: kubehealth.ReconciliationReconciled}, want: status.CurrentStatus},
		{name: "in progress", assessment: kubehealth.Assessment{Lifecycle: kubehealth.LifecycleActive, Reconciliation: kubehealth.ReconciliationInProgress}, want: status.InProgressStatus},
		{name: "failed", assessment: kubehealth.Assessment{Lifecycle: kubehealth.LifecycleActive, Reconciliation: kubehealth.ReconciliationFailed}, want: status.FailedStatus},
		{name: "suspended", assessment: kubehealth.Assessment{Lifecycle: kubehealth.LifecycleActive, Reconciliation: kubehealth.ReconciliationSuspended}, want: status.InProgressStatus},
		{name: "terminating", assessment: kubehealth.Assessment{Lifecycle: kubehealth.LifecycleTerminating}, want: status.TerminatingStatus},
		{name: "not found", assessment: kubehealth.Assessment{Lifecycle: kubehealth.LifecycleNotFound}, want: status.NotFoundStatus},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := kstatus.Project(test.assessment); got.Status != test.want {
				t.Fatalf("status = %q, want %q", got.Status, test.want)
			}
		})
	}
}
