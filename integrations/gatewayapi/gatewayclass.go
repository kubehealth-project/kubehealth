package gatewayapi

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kubehealth-project/kubehealth"
)

func gatewayClass(obj *unstructured.Unstructured) (kubehealth.Assessment, error) {
	value, _, err := unstructured.NestedFieldNoCopy(obj.Object, "status", "conditions")
	if err != nil {
		return kubehealth.Assessment{}, err
	}
	conditions, err := parseConditions(value)
	if err != nil {
		return kubehealth.Assessment{}, err
	}
	accepted := findCondition(conditions, "Accepted")
	result := kubehealth.Assessment{
		Reconciliation: kubehealth.Dimension[kubehealth.ReconciliationStatus]{Status: kubehealth.ReconciliationInProgress, Message: "Waiting for GatewayClass status"},
		Availability:   kubehealth.Dimension[kubehealth.AvailabilityStatus]{Status: kubehealth.AvailabilityUnknown, Message: "GatewayClass availability is not reported"},
		Lifecycle:      kubehealth.Dimension[kubehealth.LifecycleStatus]{Status: kubehealth.LifecycleActive},
	}
	if accepted == nil {
		return result, nil
	}
	if isStale(accepted, obj.GetGeneration()) {
		result.Reconciliation.Message = "Waiting for GatewayClass to observe the latest generation"
		if accepted.Status == corev1.ConditionTrue {
			result.Availability.Status = kubehealth.AvailabilityAvailable
			result.Availability.Message = "The previously observed GatewayClass configuration is accepted"
		}
		return result, nil
	}
	switch accepted.Status {
	case corev1.ConditionTrue:
		result.Reconciliation.Status = kubehealth.ReconciliationReconciled
		result.Availability.Status = kubehealth.AvailabilityAvailable
		result.Reconciliation.Message = conditionMessage(accepted, "GatewayClass is accepted")
		result.Availability.Message = "GatewayClass is available for provisioning Gateways"
	case corev1.ConditionFalse:
		result.Reconciliation.Status = kubehealth.ReconciliationFailed
		result.Availability.Status = kubehealth.AvailabilityUnavailable
		result.Reconciliation.Message = conditionMessage(accepted, "GatewayClass is not accepted")
		result.Availability.Message = "GatewayClass is not available for provisioning Gateways"
	default:
		result.Reconciliation.Message = conditionMessage(accepted, "Waiting for GatewayClass acceptance")
	}
	return result, nil
}
