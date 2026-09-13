package v1alpha1_test

import (
	"testing"
	"time"

	"github.com/kubehealth-project/kubehealth/api"
	healthv1alpha1 "github.com/kubehealth-project/kubehealth/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestStatusHelpersMaintainTransitionTimes(t *testing.T) {
	first := metav1.NewTime(time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC))
	second := metav1.NewTime(first.Add(time.Minute))
	third := metav1.NewTime(second.Add(time.Minute))
	status := healthv1alpha1.NewStatus(3)

	status.SetReconciliationAt(api.ReconciliationInProgress, "Reconciling", "starting", first)
	status.SetReconciliationAt(api.ReconciliationInProgress, "WaitingForDependency", "still waiting", second)
	if got := status.Reconciliation.LastTransitionTime; got == nil || !got.Equal(&first) {
		t.Fatalf("unchanged status transition time = %v, want %v", got, first)
	}

	status.SetReconciliationAt(api.ReconciliationReconciled, "ReconciliationSucceeded", "ready", third)
	if got := status.Reconciliation.LastTransitionTime; got == nil || !got.Equal(&third) {
		t.Fatalf("changed status transition time = %v, want %v", got, third)
	}
}

func TestStatusValidate(t *testing.T) {
	valid := validStatus()
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid status: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*healthv1alpha1.Status)
	}{
		{"unsupported contract", func(status *healthv1alpha1.Status) { status.ContractVersion = "v2" }},
		{"negative generation", func(status *healthv1alpha1.Status) { status.ObservedGeneration = -1 }},
		{"invalid reconciliation", func(status *healthv1alpha1.Status) { status.Reconciliation.Status = "Healthy" }},
		{"invalid availability", func(status *healthv1alpha1.Status) { status.Availability.Status = "Degraded" }},
		{"invalid lifecycle", func(status *healthv1alpha1.Status) { status.Lifecycle.Status = api.LifecycleNotFound }},
		{"invalid reason", func(status *healthv1alpha1.Status) { status.Availability.Reason = "not camel case" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status := validStatus()
			test.mutate(&status)
			if err := status.Validate(); err == nil {
				t.Fatal("Validate() succeeded, want an error")
			}
		})
	}
}

func TestStatusAssessment(t *testing.T) {
	status := validStatus()
	got := status.ToAssessment()
	if got.Reconciliation.Status != api.ReconciliationFailed || got.Reconciliation.Message != "rollout failed" {
		t.Fatalf("reconciliation = %q, message = %q", got.Reconciliation.Status, got.Reconciliation.Message)
	}
	if got.Availability.Status != api.AvailabilityAvailable || got.Availability.Message != "old revision serves" {
		t.Fatalf("availability = %q, message = %q", got.Availability.Status, got.Availability.Message)
	}
	if got.Lifecycle.Status != api.LifecycleActive || got.Lifecycle.Message != "resource is active" {
		t.Fatalf("lifecycle = %q, message = %q", got.Lifecycle.Status, got.Lifecycle.Message)
	}
}

func TestStatusDeepCopyDoesNotShareTransitionTimes(t *testing.T) {
	status := validStatus()
	now := metav1.Now()
	status.Reconciliation.LastTransitionTime = &now

	copy := status.DeepCopy()
	copy.Reconciliation.LastTransitionTime.Time = copy.Reconciliation.LastTransitionTime.Add(time.Minute)
	if status.Reconciliation.LastTransitionTime.Equal(copy.Reconciliation.LastTransitionTime) {
		t.Fatal("DeepCopy() shared reconciliation transition time")
	}
}

func validStatus() healthv1alpha1.Status {
	return healthv1alpha1.Status{
		ContractVersion:    healthv1alpha1.ContractVersion,
		ObservedGeneration: 3,
		Reconciliation: api.Dimension[api.ReconciliationStatus]{
			Status: api.ReconciliationFailed, Reason: "ProgressDeadlineExceeded", Message: "rollout failed",
		},
		Availability: api.Dimension[api.AvailabilityStatus]{
			Status: api.AvailabilityAvailable, Reason: "PreviousRevisionServing", Message: "old revision serves",
		},
		Lifecycle: api.Dimension[api.LifecycleStatus]{
			Status: api.LifecycleActive, Reason: "DeletionNotRequested", Message: "resource is active",
		},
	}
}
