package externalsecrets_test

import (
	"strings"
	"testing"

	"github.com/kubehealth-project/kubehealth"
)

func TestExactGVKRegistration(t *testing.T) {
	tests := []struct {
		apiVersion string
		kind       string
	}{
		{apiVersion: "external-secrets.io/v1", kind: "ExternalSecret"},
		{apiVersion: "external-secrets.io/v1", kind: "ClusterExternalSecret"},
		{apiVersion: "external-secrets.io/v1", kind: "SecretStore"},
		{apiVersion: "external-secrets.io/v1", kind: "ClusterSecretStore"},
		{apiVersion: "external-secrets.io/v1alpha1", kind: "PushSecret"},
	}
	assessor := newAssessor(t)
	for _, tt := range tests {
		t.Run(tt.apiVersion+" "+tt.kind, func(t *testing.T) {
			got, err := assessor.Assess(resource(tt.apiVersion, tt.kind, nil))
			if err != nil {
				t.Fatal(err)
			}
			if got.Reconciliation != kubehealth.ReconciliationInProgress {
				t.Errorf("reconciliation = %q, want %q", got.Reconciliation, kubehealth.ReconciliationInProgress)
			}
		})
	}
}

func TestHistoricalAndSpeculativeGVKsAreNotRegistered(t *testing.T) {
	tests := []struct {
		apiVersion string
		kind       string
	}{
		{apiVersion: "external-secrets.io/v1alpha1", kind: "ExternalSecret"},
		{apiVersion: "external-secrets.io/v1beta1", kind: "ExternalSecret"},
		{apiVersion: "external-secrets.io/v1beta1", kind: "ClusterExternalSecret"},
		{apiVersion: "external-secrets.io/v1beta1", kind: "SecretStore"},
		{apiVersion: "external-secrets.io/v1beta1", kind: "ClusterSecretStore"},
		{apiVersion: "external-secrets.io/v1", kind: "PushSecret"},
		{apiVersion: "example.io/v1", kind: "ExternalSecret"},
	}
	assessor := newAssessor(t)
	for _, tt := range tests {
		t.Run(tt.apiVersion+" "+tt.kind, func(t *testing.T) {
			got, err := assessor.Assess(resource(tt.apiVersion, tt.kind, nil))
			if err != nil {
				t.Fatal(err)
			}
			if got.Reconciliation != kubehealth.ReconciliationUnknown {
				t.Errorf("reconciliation = %q, want %q", got.Reconciliation, kubehealth.ReconciliationUnknown)
			}
			if !strings.Contains(got.ReconciliationMessage, "No health check registered") {
				t.Errorf("message = %q, want unregistered check message", got.ReconciliationMessage)
			}
		})
	}
}
