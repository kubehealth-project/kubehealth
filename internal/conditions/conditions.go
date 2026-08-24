// Package conditions reads common Kubernetes conditions from unstructured resources.
package conditions

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kubehealth-project/kubehealth/api"
)

// Get returns status.conditions in the KubeHealth condition shape.
func Get(obj *unstructured.Unstructured) ([]api.Condition, error) {
	value, found, err := unstructured.NestedFieldNoCopy(obj.Object, "status", "conditions")
	if err != nil {
		return nil, fmt.Errorf("read status.conditions: %w", err)
	}
	if !found {
		return nil, nil
	}
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("read status.conditions: expected an array, got %T", value)
	}
	conditions := make([]api.Condition, 0, len(items))
	for _, item := range items {
		value, ok := item.(map[string]any)
		if !ok {
			continue
		}
		conditions = append(conditions, api.Condition{
			Type: stringField(value, "type"), Status: corev1.ConditionStatus(stringField(value, "status")),
			Reason: stringField(value, "reason"), Message: stringField(value, "message"),
		})
	}
	return conditions, nil
}

func stringField(object map[string]any, field string) string {
	value, _ := object[field].(string)
	return value
}
