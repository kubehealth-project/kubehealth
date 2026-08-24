package certmanager

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kubehealth-project/kubehealth"
)

func issuer(obj *unstructured.Unstructured) (kubehealth.Assessment, error) {
	conditions, err := kubehealth.GetConditions(obj)
	if err != nil {
		return kubehealth.Assessment{}, err
	}
	ready := findCondition(conditions, "Ready")
	if ready != nil {
		switch ready.Status {
		case corev1.ConditionTrue:
			return assessment(kubehealth.ReconciliationReconciled, kubehealth.AvailabilityAvailable, ready.Message, "Issuer is ready to issue certificates", conditions), nil
		case corev1.ConditionFalse:
			return assessment(kubehealth.ReconciliationFailed, kubehealth.AvailabilityUnavailable, ready.Message, "Issuer is not ready", conditions), nil
		}
	}
	return assessment(kubehealth.ReconciliationInProgress, kubehealth.AvailabilityUnknown, "Initializing issuer", "Issuer availability is not reported", conditions), nil
}

func assessment(reconciliation kubehealth.ReconciliationStatus, availability kubehealth.AvailabilityStatus, reconciliationMessage, availabilityMessage string, conditions []kubehealth.Condition) kubehealth.Assessment {
	return kubehealth.Assessment{
		Reconciliation: reconciliation, Availability: availability,
		Lifecycle: kubehealth.LifecycleActive, ReconciliationMessage: reconciliationMessage,
		AvailabilityMessage: availabilityMessage, Conditions: conditions,
	}
}

func findCondition(conditions []kubehealth.Condition, conditionType string) *kubehealth.Condition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}
