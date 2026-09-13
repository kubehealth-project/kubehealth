package kubehealth_test

import (
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	kubehealth "github.com/kubehealth-project/kubehealth"
)

var widgetGVK = schema.GroupVersionKind{Group: "example.io", Version: "v1", Kind: "Widget"}

func TestReportedStatusSkipsEntireAssessment(t *testing.T) {
	assessor := kubehealth.NewAssessor()
	checkCalls := 0
	if err := assessor.Register(widgetGVK, func(*unstructured.Unstructured) (kubehealth.Assessment, error) {
		checkCalls++
		return kubehealth.Assessment{
			Reconciliation: kubehealth.Dimension[kubehealth.ReconciliationStatus]{Status: kubehealth.ReconciliationReconciled}, Availability: kubehealth.Dimension[kubehealth.AvailabilityStatus]{Status: kubehealth.AvailabilityUnavailable}, Lifecycle: kubehealth.Dimension[kubehealth.LifecycleStatus]{Status: kubehealth.LifecycleActive},
		}, nil
	}); err != nil {
		t.Fatal(err)
	}

	obj := widget(4, reportedStatus(4, "Failed", "Available", "Active"))
	obj.Object["status"].(map[string]any)["conditions"] = "malformed but not evaluated"
	got, err := assessor.Assess(obj)
	if err != nil {
		t.Fatal(err)
	}
	if checkCalls != 0 {
		t.Fatalf("registered check called %d times, want 0", checkCalls)
	}
	if got.Reconciliation.Status != kubehealth.ReconciliationFailed || got.Reconciliation.Message != "desired revision failed" {
		t.Fatalf("reconciliation result = %#v", got)
	}
	if got.Availability.Status != kubehealth.AvailabilityAvailable || got.Availability.Message != "previous revision serves" {
		t.Fatalf("availability result = %#v", got)
	}
	if got.Lifecycle.Status != kubehealth.LifecycleActive {
		t.Fatalf("lifecycle = %q, want %q", got.Lifecycle, kubehealth.LifecycleActive)
	}
}

func TestReportedStatusAcceptsExplicitUnknown(t *testing.T) {
	obj := widget(1, reportedStatus(1, "Unknown", "Unknown", "Active"))
	got, err := kubehealth.Assess(obj)
	if err != nil {
		t.Fatal(err)
	}
	if got.Reconciliation.Status != kubehealth.ReconciliationUnknown || got.Availability.Status != kubehealth.AvailabilityUnknown {
		t.Fatalf("result = %#v", got)
	}
}

func TestPartialReportedStatusFallsBackToRegisteredCheck(t *testing.T) {
	assessor := kubehealth.NewAssessor()
	checkCalls := 0
	if err := assessor.Register(widgetGVK, func(*unstructured.Unstructured) (kubehealth.Assessment, error) {
		checkCalls++
		return kubehealth.Assessment{
			Reconciliation: kubehealth.Dimension[kubehealth.ReconciliationStatus]{Status: kubehealth.ReconciliationReconciled}, Availability: kubehealth.Dimension[kubehealth.AvailabilityStatus]{Status: kubehealth.AvailabilityAvailable}, Lifecycle: kubehealth.Dimension[kubehealth.LifecycleStatus]{Status: kubehealth.LifecycleActive},
		}, nil
	}); err != nil {
		t.Fatal(err)
	}

	status := reportedStatus(1, "Failed", "Unavailable", "Active")
	delete(status, "lifecycle")
	got, err := assessor.Assess(widget(1, status))
	if err != nil {
		t.Fatal(err)
	}
	if checkCalls != 1 {
		t.Fatalf("registered check called %d times, want 1", checkCalls)
	}
	if got.Reconciliation.Status != kubehealth.ReconciliationReconciled || got.Availability.Status != kubehealth.AvailabilityAvailable {
		t.Fatalf("result = %#v", got)
	}
}

func TestMalformedPartialReportedStatusFallsBackToRegisteredCheck(t *testing.T) {
	assessor := kubehealth.NewAssessor()
	checkCalls := 0
	if err := assessor.Register(widgetGVK, func(*unstructured.Unstructured) (kubehealth.Assessment, error) {
		checkCalls++
		return kubehealth.Assessment{
			Reconciliation: kubehealth.Dimension[kubehealth.ReconciliationStatus]{Status: kubehealth.ReconciliationReconciled}, Availability: kubehealth.Dimension[kubehealth.AvailabilityStatus]{Status: kubehealth.AvailabilityAvailable}, Lifecycle: kubehealth.Dimension[kubehealth.LifecycleStatus]{Status: kubehealth.LifecycleActive},
		}, nil
	}); err != nil {
		t.Fatal(err)
	}

	status := reportedStatus(1, "Failed", "Unavailable", "Active")
	status["reconciliation"] = "malformed"
	delete(status, "lifecycle")
	got, err := assessor.Assess(widget(1, status))
	if err != nil {
		t.Fatal(err)
	}
	if checkCalls != 1 {
		t.Fatalf("registered check called %d times, want 1", checkCalls)
	}
	if got.Reconciliation.Status != kubehealth.ReconciliationReconciled || got.Availability.Status != kubehealth.AvailabilityAvailable {
		t.Fatalf("result = %#v", got)
	}
}

func TestStaleReportedStatusRunsRegisteredCheck(t *testing.T) {
	assessor := kubehealth.NewAssessor()
	checkCalls := 0
	if err := assessor.Register(widgetGVK, func(*unstructured.Unstructured) (kubehealth.Assessment, error) {
		checkCalls++
		return kubehealth.Assessment{
			Reconciliation: kubehealth.Dimension[kubehealth.ReconciliationStatus]{Status: kubehealth.ReconciliationReconciled}, Availability: kubehealth.Dimension[kubehealth.AvailabilityStatus]{Status: kubehealth.AvailabilityPartiallyAvailable}, Lifecycle: kubehealth.Dimension[kubehealth.LifecycleStatus]{Status: kubehealth.LifecycleActive},
		}, nil
	}); err != nil {
		t.Fatal(err)
	}

	got, err := assessor.Assess(widget(2, reportedStatus(1, "Reconciled", "Available", "Active")))
	if err != nil {
		t.Fatal(err)
	}
	if checkCalls != 1 {
		t.Fatalf("registered check called %d times, want 1", checkCalls)
	}
	if got.Reconciliation.Status != kubehealth.ReconciliationInProgress {
		t.Fatalf("reconciliation = %q, want %q", got.Reconciliation, kubehealth.ReconciliationInProgress)
	}
	if got.Availability.Status != kubehealth.AvailabilityPartiallyAvailable {
		t.Fatalf("availability = %q, want %q", got.Availability, kubehealth.AvailabilityPartiallyAvailable)
	}
	if !strings.Contains(got.Reconciliation.Message, "observed generation is 1") {
		t.Fatalf("reconciliation message = %q", got.Reconciliation.Message)
	}
}

func TestStaleReportedStatusSuppliesAvailabilityWithoutRegisteredCheck(t *testing.T) {
	got, err := kubehealth.Assess(widget(2, reportedStatus(1, "Reconciled", "Available", "Active")))
	if err != nil {
		t.Fatal(err)
	}
	if got.Reconciliation.Status != kubehealth.ReconciliationInProgress || got.Availability.Status != kubehealth.AvailabilityAvailable {
		t.Fatalf("result = %#v", got)
	}
}

func TestDeletingReportedStatusCanReturnImmediately(t *testing.T) {
	assessor := kubehealth.NewAssessor()
	checkCalls := 0
	if err := assessor.Register(widgetGVK, func(*unstructured.Unstructured) (kubehealth.Assessment, error) {
		checkCalls++
		return kubehealth.Assessment{}, nil
	}); err != nil {
		t.Fatal(err)
	}
	obj := widget(2, reportedStatus(2, "InProgress", "Available", "Terminating"))
	now := metav1.Now()
	obj.SetDeletionTimestamp(&now)

	got, err := assessor.Assess(obj)
	if err != nil {
		t.Fatal(err)
	}
	if checkCalls != 0 {
		t.Fatalf("registered check called %d times, want 0", checkCalls)
	}
	if got.Lifecycle.Status != kubehealth.LifecycleTerminating || got.Availability.Status != kubehealth.AvailabilityAvailable {
		t.Fatalf("result = %#v", got)
	}
}

func TestDeletionTimestampMakesActiveReportedStatusIneligible(t *testing.T) {
	assessor := kubehealth.NewAssessor()
	checkCalls := 0
	if err := assessor.Register(widgetGVK, func(*unstructured.Unstructured) (kubehealth.Assessment, error) {
		checkCalls++
		return kubehealth.Assessment{
			Reconciliation: kubehealth.Dimension[kubehealth.ReconciliationStatus]{Status: kubehealth.ReconciliationFailed}, Availability: kubehealth.Dimension[kubehealth.AvailabilityStatus]{Status: kubehealth.AvailabilityAvailable}, Lifecycle: kubehealth.Dimension[kubehealth.LifecycleStatus]{Status: kubehealth.LifecycleActive},
		}, nil
	}); err != nil {
		t.Fatal(err)
	}
	obj := widget(2, reportedStatus(2, "Reconciled", "Unavailable", "Active"))
	now := metav1.Now()
	obj.SetDeletionTimestamp(&now)

	got, err := assessor.Assess(obj)
	if err != nil {
		t.Fatal(err)
	}
	if checkCalls != 1 {
		t.Fatalf("registered check called %d times, want 1", checkCalls)
	}
	if got.Reconciliation.Status != kubehealth.ReconciliationUnknown || got.Availability.Status != kubehealth.AvailabilityAvailable || got.Lifecycle.Status != kubehealth.LifecycleTerminating {
		t.Fatalf("result = %#v", got)
	}
}

func TestReportedStatusRejectsInvalidCompleteReports(t *testing.T) {
	tests := []struct {
		name   string
		status map[string]any
		want   string
	}{
		{"unsupported contract", reportedStatusWithVersion("v2", 1, "Reconciled", "Available", "Active"), "unsupported KubeHealth contract version"},
		{"future generation", reportedStatus(2, "Reconciled", "Available", "Active"), "newer than metadata.generation"},
		{"invalid status", reportedStatus(1, "Healthy", "Available", "Active"), "invalid reconciliation status"},
		{"terminating without deletion", reportedStatus(1, "Reconciled", "Available", "Terminating"), "metadata.deletionTimestamp is not set"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := kubehealth.Assess(widget(1, test.status))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func widget(generation int64, kubeHealth map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "example.io/v1",
		"kind":       "Widget",
		"metadata": map[string]any{
			"name":       "example",
			"generation": generation,
		},
		"status": map[string]any{"kubeHealth": kubeHealth},
	}}
}

func reportedStatus(generation int64, reconciliation, availability, lifecycle string) map[string]any {
	return reportedStatusWithVersion("v1alpha1", generation, reconciliation, availability, lifecycle)
}

func reportedStatusWithVersion(version string, generation int64, reconciliation, availability, lifecycle string) map[string]any {
	return map[string]any{
		"contractVersion":    version,
		"observedGeneration": generation,
		"reconciliation": map[string]any{
			"status":  reconciliation,
			"reason":  "ReconciliationReported",
			"message": "desired revision failed",
		},
		"availability": map[string]any{
			"status":  availability,
			"reason":  "AvailabilityReported",
			"message": "previous revision serves",
		},
		"lifecycle": map[string]any{
			"status": lifecycle,
			"reason": "LifecycleReported",
		},
	}
}
