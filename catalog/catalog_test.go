package catalog_test

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kubehealth-project/kubehealth"
	"github.com/kubehealth-project/kubehealth/catalog"
)

func TestCatalogRegistersIntegrations(t *testing.T) {
	assessor, err := catalog.NewAssessor()
	if err != nil {
		t.Fatal(err)
	}
	objects := []*unstructured.Unstructured{
		resource("argoproj.io/v1alpha1", "Rollout"),
		resource("cert-manager.io/v1", "Certificate"),
		resource("cert-manager.io/v1", "Issuer"),
		resource("kyverno.io/v1", "Policy"),
		resource("kyverno.io/v1", "ClusterPolicy"),
		resource("keda.sh/v1alpha1", "ScaledObject"),
	}
	for _, obj := range objects {
		got, err := assessor.Assess(obj)
		if err != nil {
			t.Errorf("Assess(%s): %v", obj.GroupVersionKind(), err)
			continue
		}
		if got.Reconciliation == kubehealth.ReconciliationUnknown {
			t.Errorf("%s was not registered", obj.GroupVersionKind())
		}
	}
}

func resource(apiVersion, kind string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": apiVersion, "kind": kind,
		"metadata": map[string]any{"name": "example"},
	}}
}
