package gatewayapi_test

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kubehealth-project/kubehealth"
	"github.com/kubehealth-project/kubehealth/integrations/gatewayapi"
)

func assess(t *testing.T, obj *unstructured.Unstructured) kubehealth.Assessment {
	t.Helper()
	assessor := kubehealth.NewAssessor()
	if err := gatewayapi.Register(assessor); err != nil {
		t.Fatal(err)
	}
	got, err := assessor.Assess(obj)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func resource(kind string, generation int64, spec, status map[string]any) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gateway.networking.k8s.io/v1",
		"kind":       kind,
		"metadata": map[string]any{
			"name": "example", "namespace": "default", "generation": generation,
		},
		"spec": spec,
	}}
	if status != nil {
		obj.Object["status"] = status
	}
	return obj
}

func condition(conditionType, status, reason string, generation int64) map[string]any {
	return map[string]any{
		"type": conditionType, "status": status, "reason": reason,
		"message": reason + " message", "observedGeneration": generation,
	}
}

func assertAssessment(t *testing.T, got kubehealth.Assessment, reconciliation kubehealth.ReconciliationStatus, availability kubehealth.AvailabilityStatus) {
	t.Helper()
	if got.Reconciliation.Status != reconciliation {
		t.Errorf("reconciliation = %q, want %q (%s)", got.Reconciliation, reconciliation, got.Reconciliation.Message)
	}
	if got.Availability.Status != availability {
		t.Errorf("availability = %q, want %q (%s)", got.Availability, availability, got.Availability.Message)
	}
	if got.Lifecycle.Status != kubehealth.LifecycleActive {
		t.Errorf("lifecycle = %q, want %q", got.Lifecycle, kubehealth.LifecycleActive)
	}
}

func TestRegistersOnlyFiveV1GVKs(t *testing.T) {
	assessor := kubehealth.NewAssessor()
	if err := gatewayapi.Register(assessor); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"GatewayClass", "Gateway", "HTTPRoute", "GRPCRoute", "BackendTLSPolicy"} {
		obj := resource(kind, 1, map[string]any{}, nil)
		if _, err := assessor.Assess(obj); err != nil {
			t.Errorf("assess %s: %v", kind, err)
		}
	}
	unsupported := resource("GatewayClass", 1, map[string]any{}, nil)
	unsupported.SetGroupVersionKind(schema.GroupVersionKind{Group: "gateway.networking.k8s.io", Version: "v1beta1", Kind: "GatewayClass"})
	got, err := assessor.Assess(unsupported)
	if err != nil {
		t.Fatal(err)
	}
	if got.Reconciliation.Status != kubehealth.ReconciliationUnknown {
		t.Fatalf("v1beta1 reconciliation = %q, want Unknown", got.Reconciliation)
	}
}

func TestGatewayAPILifecycleDeletion(t *testing.T) {
	obj := resource("GatewayClass", 1, map[string]any{"controllerName": "example.io/controller"}, map[string]any{
		"conditions": []any{condition("Accepted", "True", "Accepted", 1)},
	})
	now := metav1.Now()
	obj.SetDeletionTimestamp(&now)
	got := assess(t, obj)
	if got.Lifecycle.Status != kubehealth.LifecycleTerminating {
		t.Fatalf("lifecycle = %q, want Terminating", got.Lifecycle)
	}
	if got.Availability.Status != kubehealth.AvailabilityAvailable {
		t.Fatalf("availability = %q, want Available", got.Availability)
	}
}
