package builtins_test

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kubehealth-project/kubehealth"
)

func TestDaemonSet(t *testing.T) {
	tests := []struct {
		name               string
		make               func() *appsv1.DaemonSet
		wantReconciliation kubehealth.ReconciliationStatus
		wantAvailability   kubehealth.AvailabilityStatus
	}{
		{name: "healthy", make: func() *appsv1.DaemonSet { return daemonSetScenario(3, 3, 3) }, wantReconciliation: kubehealth.ReconciliationReconciled, wantAvailability: kubehealth.AvailabilityAvailable},
		{name: "updating while available", make: func() *appsv1.DaemonSet { return daemonSetScenario(3, 1, 3) }, wantReconciliation: kubehealth.ReconciliationInProgress, wantAvailability: kubehealth.AvailabilityAvailable},
		{name: "updated but partially available", make: func() *appsv1.DaemonSet { return daemonSetScenario(3, 3, 2) }, wantReconciliation: kubehealth.ReconciliationInProgress, wantAvailability: kubehealth.AvailabilityPartiallyAvailable},
		{name: "on delete unavailable", make: func() *appsv1.DaemonSet {
			daemonSet := daemonSetScenario(3, 0, 0)
			daemonSet.Spec.UpdateStrategy.Type = appsv1.OnDeleteDaemonSetStrategyType
			return daemonSet
		}, wantReconciliation: kubehealth.ReconciliationReconciled, wantAvailability: kubehealth.AvailabilityUnavailable},
		{name: "no desired pods", make: func() *appsv1.DaemonSet { return daemonSetScenario(0, 0, 0) }, wantReconciliation: kubehealth.ReconciliationReconciled, wantAvailability: kubehealth.AvailabilityNotApplicable},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assessment, err := newBuiltinsAssessor(t).Assess(unstructuredFrom(t, test.make()))
			if err != nil {
				t.Fatal(err)
			}
			if assessment.Reconciliation.Status != test.wantReconciliation || assessment.Availability.Status != test.wantAvailability {
				t.Fatalf("assessment = %#v", assessment)
			}
		})
	}
}

func daemonSetScenario(desired, updated, available int32) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		TypeMeta:   metav1.TypeMeta{APIVersion: "apps/v1", Kind: "DaemonSet"},
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Generation: 1},
		Spec: appsv1.DaemonSetSpec{UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
			Type: appsv1.RollingUpdateDaemonSetStrategyType,
		}},
		Status: appsv1.DaemonSetStatus{
			ObservedGeneration:     1,
			DesiredNumberScheduled: desired,
			UpdatedNumberScheduled: updated,
			NumberAvailable:        available,
		},
	}
}
