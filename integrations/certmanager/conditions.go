package certmanager

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kubehealth-project/kubehealth"
)

type statusCondition struct {
	kubehealth.Condition
	observedGeneration    int64
	hasObservedGeneration bool
}

func readStatusConditions(obj *unstructured.Unstructured) ([]statusCondition, []kubehealth.Condition, bool, error) {
	value, found, err := unstructured.NestedFieldNoCopy(obj.Object, "status", "conditions")
	if err != nil {
		return nil, nil, false, fmt.Errorf("read status.conditions: %w", err)
	}
	if !found {
		return nil, nil, false, nil
	}
	items, ok := value.([]any)
	if !ok {
		return nil, nil, true, fmt.Errorf("read status.conditions: expected an array, got %T", value)
	}

	conditions := make([]statusCondition, 0, len(items))
	public := make([]kubehealth.Condition, 0, len(items))
	for _, item := range items {
		fields, ok := item.(map[string]any)
		if !ok {
			continue
		}
		condition := statusCondition{Condition: kubehealth.Condition{
			Type: stringField(fields, "type"), Status: corev1.ConditionStatus(stringField(fields, "status")),
			Reason: stringField(fields, "reason"), Message: stringField(fields, "message"),
		}}
		condition.observedGeneration, condition.hasObservedGeneration, err = unstructured.NestedInt64(fields, "observedGeneration")
		if err != nil {
			return nil, nil, true, fmt.Errorf("read status.conditions observedGeneration: %w", err)
		}
		conditions = append(conditions, condition)
		public = append(public, condition.Condition)
	}
	return conditions, public, true, nil
}

func conditionByType(conditions []statusCondition, conditionType string) (*statusCondition, bool) {
	var found *statusCondition
	for i := range conditions {
		if conditions[i].Type != conditionType {
			continue
		}
		if found != nil {
			return nil, true
		}
		found = &conditions[i]
	}
	return found, false
}

func (condition *statusCondition) stale(generation int64) bool {
	return condition != nil && condition.hasObservedGeneration && condition.observedGeneration < generation
}

func stringField(fields map[string]any, name string) string {
	value, _ := fields[name].(string)
	return value
}
