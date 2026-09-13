package externalsecrets_test

import (
	"strings"
	"testing"

	"github.com/kubehealth-project/kubehealth"
)

func TestClusterExternalSecret(t *testing.T) {
	tests := []struct {
		name         string
		status       map[string]any
		reconcile    kubehealth.ReconciliationStatus
		availability kubehealth.AvailabilityStatus
	}{
		{name: "all selected namespaces provisioned", status: clusterStatus("True", []any{"one", "two"}, nil), reconcile: kubehealth.ReconciliationReconciled, availability: kubehealth.AvailabilityAvailable},
		{name: "no selected namespaces", status: clusterStatus("True", nil, nil), reconcile: kubehealth.ReconciliationReconciled, availability: kubehealth.AvailabilityAvailable},
		{name: "some namespaces failed", status: clusterStatus("False", []any{"one"}, []any{namespaceFailure("two")}), reconcile: kubehealth.ReconciliationFailed, availability: kubehealth.AvailabilityPartiallyAvailable},
		{name: "all namespaces failed", status: clusterStatus("False", nil, []any{namespaceFailure("one"), namespaceFailure("two")}), reconcile: kubehealth.ReconciliationFailed, availability: kubehealth.AvailabilityUnavailable},
		{name: "false condition lacks failure detail", status: clusterStatus("False", []any{"one"}, nil), reconcile: kubehealth.ReconciliationFailed, availability: kubehealth.AvailabilityUnknown},
		{name: "ready unknown", status: clusterStatus("Unknown", nil, nil), reconcile: kubehealth.ReconciliationUnknown, availability: kubehealth.AvailabilityUnknown},
		{name: "initializing", reconcile: kubehealth.ReconciliationInProgress, availability: kubehealth.AvailabilityUnknown},
		{name: "historical partially ready is ignored", status: conditions(condition("PartiallyReady", "True", "", "one or more namespaces failed")), reconcile: kubehealth.ReconciliationInProgress, availability: kubehealth.AvailabilityUnknown},
		{name: "historical not ready is ignored", status: conditions(condition("NotReady", "True", "", "one or more namespaces failed")), reconcile: kubehealth.ReconciliationInProgress, availability: kubehealth.AvailabilityUnknown},
		{name: "last historical condition does not override ready", status: conditions(condition("Ready", "True", "", ""), condition("PartiallyReady", "True", "", "old condition")), reconcile: kubehealth.ReconciliationReconciled, availability: kubehealth.AvailabilityAvailable},
		{name: "duplicate ready is ambiguous", status: conditions(condition("Ready", "True", "", ""), condition("Ready", "False", "", "failed")), reconcile: kubehealth.ReconciliationUnknown, availability: kubehealth.AvailabilityUnknown},
	}
	assessor := newAssessor(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := assessor.Assess(resource("external-secrets.io/v1", "ClusterExternalSecret", tt.status))
			if err != nil {
				t.Fatal(err)
			}
			assertAssessment(t, got, tt.reconcile, tt.availability)
			if tt.availability == kubehealth.AvailabilityAvailable || tt.availability == kubehealth.AvailabilityPartiallyAvailable || tt.availability == kubehealth.AvailabilityUnavailable {
				if !strings.Contains(got.Availability.Message, "child ExternalSecret health is not evaluated") {
					t.Errorf("availability message = %q, want child-health scope", got.Availability.Message)
				}
			}
		})
	}
}

func TestClusterExternalSecretMalformedNamespaceStatus(t *testing.T) {
	status := clusterStatus("False", nil, nil)
	status["failedNamespaces"] = "invalid"
	_, err := newAssessor(t).Assess(resource("external-secrets.io/v1", "ClusterExternalSecret", status))
	if err == nil {
		t.Fatal("expected malformed failedNamespaces error")
	}
}

func clusterStatus(ready string, provisioned, failed []any) map[string]any {
	status := conditions(condition("Ready", ready, "", "one or more namespaces failed"))
	if provisioned != nil {
		status["provisionedNamespaces"] = provisioned
	}
	if failed != nil {
		status["failedNamespaces"] = failed
	}
	return status
}

func namespaceFailure(namespace string) map[string]any {
	return map[string]any{"namespace": namespace, "reason": "failed to provision"}
}
