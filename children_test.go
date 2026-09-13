package kubehealth_test

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kubehealth-project/kubehealth"
)

func TestAggregateReconciliation(t *testing.T) {
	tests := []struct {
		name     string
		statuses []kubehealth.ReconciliationStatus
		want     kubehealth.ReconciliationStatus
	}{
		{name: "empty", want: kubehealth.ReconciliationNotApplicable},
		{name: "all reconciled", statuses: []kubehealth.ReconciliationStatus{kubehealth.ReconciliationReconciled, kubehealth.ReconciliationReconciled}, want: kubehealth.ReconciliationReconciled},
		{name: "failure takes precedence", statuses: []kubehealth.ReconciliationStatus{kubehealth.ReconciliationInProgress, kubehealth.ReconciliationFailed, kubehealth.ReconciliationUnknown}, want: kubehealth.ReconciliationFailed},
		{name: "progress takes precedence over unknown", statuses: []kubehealth.ReconciliationStatus{kubehealth.ReconciliationUnknown, kubehealth.ReconciliationInProgress}, want: kubehealth.ReconciliationInProgress},
		{name: "progress takes precedence over suspended", statuses: []kubehealth.ReconciliationStatus{kubehealth.ReconciliationSuspended, kubehealth.ReconciliationInProgress}, want: kubehealth.ReconciliationInProgress},
		{name: "suspended takes precedence over unknown", statuses: []kubehealth.ReconciliationStatus{kubehealth.ReconciliationUnknown, kubehealth.ReconciliationSuspended}, want: kubehealth.ReconciliationSuspended},
		{name: "unknown prevents reconciled", statuses: []kubehealth.ReconciliationStatus{kubehealth.ReconciliationReconciled, kubehealth.ReconciliationUnknown}, want: kubehealth.ReconciliationUnknown},
		{name: "not applicable is ignored", statuses: []kubehealth.ReconciliationStatus{kubehealth.ReconciliationNotApplicable, kubehealth.ReconciliationReconciled}, want: kubehealth.ReconciliationReconciled},
		{name: "all not applicable", statuses: []kubehealth.ReconciliationStatus{kubehealth.ReconciliationNotApplicable, kubehealth.ReconciliationNotApplicable}, want: kubehealth.ReconciliationNotApplicable},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assessments := make([]kubehealth.Assessment, 0, len(test.statuses))
			for _, status := range test.statuses {
				assessments = append(assessments, kubehealth.Assessment{Reconciliation: kubehealth.Dimension[kubehealth.ReconciliationStatus]{Status: status}})
			}
			if got := kubehealth.AggregateReconciliation(assessments); got != test.want {
				t.Fatalf("AggregateReconciliation() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestAggregateAvailability(t *testing.T) {
	tests := []struct {
		name     string
		statuses []kubehealth.AvailabilityStatus
		want     kubehealth.AvailabilityStatus
	}{
		{name: "empty", want: kubehealth.AvailabilityNotApplicable},
		{name: "all available", statuses: []kubehealth.AvailabilityStatus{kubehealth.AvailabilityAvailable, kubehealth.AvailabilityAvailable}, want: kubehealth.AvailabilityAvailable},
		{name: "all unavailable", statuses: []kubehealth.AvailabilityStatus{kubehealth.AvailabilityUnavailable, kubehealth.AvailabilityUnavailable}, want: kubehealth.AvailabilityUnavailable},
		{name: "known mixture is partial", statuses: []kubehealth.AvailabilityStatus{kubehealth.AvailabilityAvailable, kubehealth.AvailabilityUnavailable}, want: kubehealth.AvailabilityPartiallyAvailable},
		{name: "explicit partial", statuses: []kubehealth.AvailabilityStatus{kubehealth.AvailabilityPartiallyAvailable, kubehealth.AvailabilityAvailable}, want: kubehealth.AvailabilityPartiallyAvailable},
		{name: "available and unknown is unknown", statuses: []kubehealth.AvailabilityStatus{kubehealth.AvailabilityAvailable, kubehealth.AvailabilityUnknown}, want: kubehealth.AvailabilityUnknown},
		{name: "unavailable and unknown is unknown", statuses: []kubehealth.AvailabilityStatus{kubehealth.AvailabilityUnavailable, kubehealth.AvailabilityUnknown}, want: kubehealth.AvailabilityUnknown},
		{name: "partial evidence survives unknown", statuses: []kubehealth.AvailabilityStatus{kubehealth.AvailabilityPartiallyAvailable, kubehealth.AvailabilityUnknown}, want: kubehealth.AvailabilityPartiallyAvailable},
		{name: "known mixture survives unknown", statuses: []kubehealth.AvailabilityStatus{kubehealth.AvailabilityAvailable, kubehealth.AvailabilityUnavailable, kubehealth.AvailabilityUnknown}, want: kubehealth.AvailabilityPartiallyAvailable},
		{name: "not applicable is ignored", statuses: []kubehealth.AvailabilityStatus{kubehealth.AvailabilityNotApplicable, kubehealth.AvailabilityAvailable}, want: kubehealth.AvailabilityAvailable},
		{name: "all not applicable", statuses: []kubehealth.AvailabilityStatus{kubehealth.AvailabilityNotApplicable, kubehealth.AvailabilityNotApplicable}, want: kubehealth.AvailabilityNotApplicable},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assessments := make([]kubehealth.Assessment, 0, len(test.statuses))
			for _, status := range test.statuses {
				assessments = append(assessments, kubehealth.Assessment{Availability: kubehealth.Dimension[kubehealth.AvailabilityStatus]{Status: status}})
			}
			if got := kubehealth.AggregateAvailability(assessments); got != test.want {
				t.Fatalf("AggregateAvailability() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestAggregateChildrenKeepsDimensionsIndependent(t *testing.T) {
	got := kubehealth.AggregateChildren([]kubehealth.Assessment{
		{Reconciliation: kubehealth.Dimension[kubehealth.ReconciliationStatus]{Status: kubehealth.ReconciliationFailed}, Availability: kubehealth.Dimension[kubehealth.AvailabilityStatus]{Status: kubehealth.AvailabilityAvailable}},
		{Reconciliation: kubehealth.Dimension[kubehealth.ReconciliationStatus]{Status: kubehealth.ReconciliationReconciled}, Availability: kubehealth.Dimension[kubehealth.AvailabilityStatus]{Status: kubehealth.AvailabilityAvailable}},
	})

	if got.Reconciliation != kubehealth.ReconciliationFailed {
		t.Fatalf("reconciliation = %q, want %q", got.Reconciliation, kubehealth.ReconciliationFailed)
	}
	if got.Availability != kubehealth.AvailabilityAvailable {
		t.Fatalf("availability = %q, want %q", got.Availability, kubehealth.AvailabilityAvailable)
	}
}

func TestAssessChildrenReconciliation(t *testing.T) {
	reconciled := healthyDeployment()
	inProgress := healthyDeployment()
	inProgress.Name = "updating"
	inProgress.Status.UpdatedReplicas = 0

	status, err := kubehealth.AssessChildrenReconciliation([]*unstructured.Unstructured{
		unstructuredFrom(t, reconciled),
		unstructuredFrom(t, inProgress),
	})
	if err != nil {
		t.Fatal(err)
	}
	if status != kubehealth.ReconciliationInProgress {
		t.Fatalf("status = %q, want %q", status, kubehealth.ReconciliationInProgress)
	}
}

func TestAssessChildren(t *testing.T) {
	reconciled := healthyDeployment()
	inProgress := healthyDeployment()
	inProgress.Name = "updating"
	inProgress.Status.UpdatedReplicas = 0
	inProgress.Status.AvailableReplicas = 0

	got, err := kubehealth.AssessChildren([]*unstructured.Unstructured{
		unstructuredFrom(t, reconciled),
		unstructuredFrom(t, inProgress),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Reconciliation != kubehealth.ReconciliationInProgress {
		t.Fatalf("reconciliation = %q, want %q", got.Reconciliation, kubehealth.ReconciliationInProgress)
	}
	if got.Availability != kubehealth.AvailabilityPartiallyAvailable {
		t.Fatalf("availability = %q, want %q", got.Availability, kubehealth.AvailabilityPartiallyAvailable)
	}
}

func TestAssessChildrenReconciliationReturnsChildError(t *testing.T) {
	invalid := &appsv1.Deployment{
		TypeMeta:   metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{Name: "invalid", Generation: 1},
	}
	child := unstructuredFrom(t, invalid)
	child.Object["status"] = map[string]any{"observedGeneration": "invalid"}

	status, err := kubehealth.AssessChildrenReconciliation([]*unstructured.Unstructured{child})
	if err == nil {
		t.Fatal("expected child assessment error")
	}
	if status != kubehealth.ReconciliationUnknown {
		t.Fatalf("status = %q, want %q", status, kubehealth.ReconciliationUnknown)
	}
}
