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
		Reconciliation: kubehealth.ReconciliationInProgress, Availability: kubehealth.AvailabilityUnknown,
		Lifecycle: kubehealth.LifecycleActive, ReconciliationMessage: "Waiting for GatewayClass status",
		AvailabilityMessage: "GatewayClass availability is not reported", Conditions: publicConditions(conditions, ""),
	}
	if accepted == nil {
		return result, nil
	}
	if isStale(accepted, obj.GetGeneration()) {
		result.ReconciliationMessage = "Waiting for GatewayClass to observe the latest generation"
		if accepted.Status == corev1.ConditionTrue {
			result.Availability = kubehealth.AvailabilityAvailable
			result.AvailabilityMessage = "The previously observed GatewayClass configuration is accepted"
		}
		return result, nil
	}
	switch accepted.Status {
	case corev1.ConditionTrue:
		result.Reconciliation = kubehealth.ReconciliationReconciled
		result.Availability = kubehealth.AvailabilityAvailable
		result.ReconciliationMessage = conditionMessage(accepted, "GatewayClass is accepted")
		result.AvailabilityMessage = "GatewayClass is available for provisioning Gateways"
	case corev1.ConditionFalse:
		result.Reconciliation = kubehealth.ReconciliationFailed
		result.Availability = kubehealth.AvailabilityUnavailable
		result.ReconciliationMessage = conditionMessage(accepted, "GatewayClass is not accepted")
		result.AvailabilityMessage = "GatewayClass is not available for provisioning Gateways"
	default:
		result.ReconciliationMessage = conditionMessage(accepted, "Waiting for GatewayClass acceptance")
	}
	return result, nil
}
