package certmanager

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kubehealth-project/kubehealth"
)

func certificate(obj *unstructured.Unstructured) (kubehealth.Assessment, error) {
	conditions, err := kubehealth.GetConditions(obj)
	if err != nil {
		return kubehealth.Assessment{}, err
	}
	ready := findCondition(conditions, "Ready")
	for _, condition := range conditions {
		if condition.Type == "Issuing" && condition.Status == corev1.ConditionTrue {
			availability := kubehealth.AvailabilityUnknown
			availabilityMessage := "Certificate availability is not reported"
			if ready != nil && ready.Status == corev1.ConditionTrue {
				availability = kubehealth.AvailabilityAvailable
				availabilityMessage = "The current certificate remains ready while renewal is in progress"
			}
			return assessment(kubehealth.ReconciliationInProgress, availability, condition.Message, availabilityMessage, conditions), nil
		}
	}
	if ready != nil {
		switch ready.Status {
		case corev1.ConditionTrue:
			return assessment(kubehealth.ReconciliationReconciled, kubehealth.AvailabilityAvailable, ready.Message, "Certificate is ready", conditions), nil
		case corev1.ConditionFalse:
			return assessment(kubehealth.ReconciliationFailed, kubehealth.AvailabilityUnavailable, ready.Message, "Certificate is not ready", conditions), nil
		}
	}
	return assessment(kubehealth.ReconciliationInProgress, kubehealth.AvailabilityUnknown, "Waiting for certificate", "Certificate availability is not reported", conditions), nil
}
