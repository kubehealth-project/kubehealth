package conditions_test

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kubehealth-project/kubehealth/internal/conditions"
)

func TestGet(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"status": map[string]any{
			"conditions": []any{
				map[string]any{
					"type": "Ready", "status": "True",
					"reason": "Succeeded", "message": "Resource is ready",
				},
			},
		},
	}}

	got, err := conditions.Get(obj)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("conditions = %#v", got)
	}
	if got[0].Type != "Ready" || got[0].Status != corev1.ConditionTrue {
		t.Fatalf("condition = %#v", got[0])
	}
}

func TestGetWithoutConditions(t *testing.T) {
	got, err := conditions.Get(&unstructured.Unstructured{})
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("conditions = %#v, want nil", got)
	}
}
