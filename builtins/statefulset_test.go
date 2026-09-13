package builtins_test

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kubehealth-project/kubehealth"
)

func TestStatefulSetLuaCheck(t *testing.T) {
	tests := []struct {
		name                 string
		make                 func() *appsv1.StatefulSet
		wantReconciliation   kubehealth.ReconciliationStatus
		wantAvailability     kubehealth.AvailabilityStatus
		wantReconcileMessage string
	}{
		{
			name: "healthy rolling update",
			make: func() *appsv1.StatefulSet {
				return statefulSetScenario(3, 3, 3, 3, "revision-2", "revision-2")
			},
			wantReconciliation:   kubehealth.ReconciliationReconciled,
			wantAvailability:     kubehealth.AvailabilityAvailable,
			wantReconcileMessage: "StatefulSet is reconciled",
		},
		{
			name: "initial rollout",
			make: func() *appsv1.StatefulSet {
				return statefulSetScenario(3, 1, 1, 1, "revision-1", "revision-1")
			},
			wantReconciliation:   kubehealth.ReconciliationInProgress,
			wantAvailability:     kubehealth.AvailabilityPartiallyAvailable,
			wantReconcileMessage: "Ready: 1/3",
		},
		{
			name: "update pending while old replicas serve",
			make: func() *appsv1.StatefulSet {
				return statefulSetScenario(3, 3, 3, 1, "revision-1", "revision-2")
			},
			wantReconciliation:   kubehealth.ReconciliationInProgress,
			wantAvailability:     kubehealth.AvailabilityAvailable,
			wantReconcileMessage: "Updated: 1/3",
		},
		{
			name: "partitioned update complete",
			make: func() *appsv1.StatefulSet {
				statefulSet := statefulSetScenario(3, 3, 3, 1, "revision-1", "revision-2")
				partition := int32(2)
				statefulSet.Spec.UpdateStrategy.RollingUpdate.Partition = &partition
				return statefulSet
			},
			wantReconciliation:   kubehealth.ReconciliationReconciled,
			wantAvailability:     kubehealth.AvailabilityAvailable,
			wantReconcileMessage: "StatefulSet is reconciled",
		},
		{
			name: "on delete ready",
			make: func() *appsv1.StatefulSet {
				statefulSet := statefulSetScenario(3, 3, 3, 0, "revision-1", "revision-2")
				statefulSet.Spec.UpdateStrategy = appsv1.StatefulSetUpdateStrategy{Type: appsv1.OnDeleteStatefulSetStrategyType}
				return statefulSet
			},
			wantReconciliation:   kubehealth.ReconciliationReconciled,
			wantAvailability:     kubehealth.AvailabilityAvailable,
			wantReconcileMessage: "StatefulSet is ready with OnDelete updates",
		},
		{
			name: "scaled to zero",
			make: func() *appsv1.StatefulSet {
				return statefulSetScenario(0, 0, 0, 0, "revision-1", "revision-1")
			},
			wantReconciliation:   kubehealth.ReconciliationReconciled,
			wantAvailability:     kubehealth.AvailabilityNotApplicable,
			wantReconcileMessage: "StatefulSet is reconciled",
		},
		{
			name: "observed generation ahead is accepted",
			make: func() *appsv1.StatefulSet {
				statefulSet := statefulSetScenario(3, 3, 3, 3, "revision-1", "revision-1")
				statefulSet.Status.ObservedGeneration = 2
				return statefulSet
			},
			wantReconciliation:   kubehealth.ReconciliationReconciled,
			wantAvailability:     kubehealth.AvailabilityAvailable,
			wantReconcileMessage: "StatefulSet is reconciled",
		},
		{
			name: "stale generation preserves Lua availability",
			make: func() *appsv1.StatefulSet {
				statefulSet := statefulSetScenario(3, 3, 3, 3, "revision-1", "revision-1")
				statefulSet.Generation = 2
				statefulSet.Status.ObservedGeneration = 1
				return statefulSet
			},
			wantReconciliation:   kubehealth.ReconciliationInProgress,
			wantAvailability:     kubehealth.AvailabilityAvailable,
			wantReconcileMessage: "StatefulSet generation is 2, but latest observed generation is 1",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assessment, err := newBuiltinsAssessor(t).Assess(unstructuredFrom(t, test.make()))
			if err != nil {
				t.Fatal(err)
			}
			if assessment.Reconciliation.Status != test.wantReconciliation {
				t.Fatalf("reconciliation = %q, want %q", assessment.Reconciliation, test.wantReconciliation)
			}
			if assessment.Availability.Status != test.wantAvailability {
				t.Fatalf("availability = %q, want %q", assessment.Availability, test.wantAvailability)
			}
			if assessment.Reconciliation.Message != test.wantReconcileMessage {
				t.Fatalf("reconciliation message = %q, want %q", assessment.Reconciliation.Message, test.wantReconcileMessage)
			}
		})
	}
}

func statefulSetScenario(desired, ready, available, updated int32, currentRevision, updateRevision string) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{APIVersion: "apps/v1", Kind: "StatefulSet"},
		ObjectMeta: metav1.ObjectMeta{
			Name: "database", Generation: 1,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &desired,
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type:          appsv1.RollingUpdateStatefulSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{},
			},
		},
		Status: appsv1.StatefulSetStatus{
			ObservedGeneration: 1,
			Replicas:           desired,
			ReadyReplicas:      ready,
			AvailableReplicas:  available,
			UpdatedReplicas:    updated,
			CurrentRevision:    currentRevision,
			UpdateRevision:     updateRevision,
		},
	}
}
