package kubehealth

import (
	"fmt"
	"strconv"

	"github.com/kubehealth-project/kubehealth/api"
	conditionutil "github.com/kubehealth-project/kubehealth/internal/conditions"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Condition is the common Kubernetes condition shape used by KubeHealth.
type Condition = api.Condition

// assessStandardStatus evaluates generic lifecycle and reconciliation signals
// shared by many Kubernetes resources. Keeping these checks here avoids
// repeatedly implementing deletion, observed-generation, Reconciling, and
// Stalled handling in every resource-specific check. The returned boolean says
// whether a standard signal made an authoritative decision.
func assessStandardStatus(obj *unstructured.Unstructured) (Assessment, bool, error) {
	if obj.GetDeletionTimestamp() != nil {
		return Assessment{
			Reconciliation: Dimension[ReconciliationStatus]{Status: ReconciliationUnknown},
			Lifecycle:      Dimension[LifecycleStatus]{Status: LifecycleTerminating, Message: "Resource scheduled for deletion"},
		}, true, nil
	}

	observedGeneration, found := standardObservedGeneration(obj)
	if found && observedGeneration < obj.GetGeneration() {
		message := fmt.Sprintf(
			"%s generation is %d, but latest observed generation is %d",
			obj.GetKind(), obj.GetGeneration(), observedGeneration,
		)
		return Assessment{
			Reconciliation: Dimension[ReconciliationStatus]{Status: ReconciliationInProgress, Reason: "LatestGenerationNotObserved", Message: message},
			Lifecycle:      Dimension[LifecycleStatus]{Status: LifecycleActive},
		}, true, nil
	}

	conditions, err := readConditions(obj)
	if err != nil {
		return Assessment{}, false, err
	}
	for _, condition := range conditions {
		if condition.Type == "Reconciling" && condition.Status == corev1.ConditionTrue {
			return Assessment{Reconciliation: Dimension[ReconciliationStatus]{Status: ReconciliationInProgress, Reason: condition.Reason, Message: condition.Message}, Lifecycle: Dimension[LifecycleStatus]{Status: LifecycleActive}}, true, nil
		}
		if condition.Type == "Stalled" && condition.Status == corev1.ConditionTrue {
			return Assessment{Reconciliation: Dimension[ReconciliationStatus]{Status: ReconciliationFailed, Reason: condition.Reason, Message: condition.Message}, Lifecycle: Dimension[LifecycleStatus]{Status: LifecycleActive}}, true, nil
		}
	}

	return Assessment{}, false, nil
}

// standardObservedGeneration accepts numeric strings because some CRDs expose
// generation counters as strings. Nonnumeric legacy values are left to the
// resource-specific check instead of making the generic assessment fail.
func standardObservedGeneration(obj *unstructured.Unstructured) (int64, bool) {
	value, found, err := unstructured.NestedFieldNoCopy(obj.Object, "status", "observedGeneration")
	if err != nil || !found {
		return 0, false
	}
	switch typed := value.(type) {
	case int64:
		return typed, true
	case float64:
		return int64(typed), typed == float64(int64(typed))
	case string:
		parsed, err := strconv.ParseInt(typed, 10, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

// GetConditions returns status.conditions in the common Kubernetes condition shape.
func GetConditions(obj *unstructured.Unstructured) ([]Condition, error) {
	return conditionutil.Get(obj)
}

func readConditions(obj *unstructured.Unstructured) ([]Condition, error) {
	return GetConditions(obj)
}
