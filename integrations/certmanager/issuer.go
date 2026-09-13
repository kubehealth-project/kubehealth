package certmanager

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kubehealth-project/kubehealth"
)

func issuer(obj *unstructured.Unstructured) (kubehealth.Assessment, error) {
	statusConditions, conditions, conditionsPresent, err := readStatusConditions(obj)
	if err != nil {
		return kubehealth.Assessment{}, err
	}
	kind := obj.GetKind()
	ready, ambiguous := conditionByType(statusConditions, "Ready")
	if ambiguous {
		return assessment(kubehealth.ReconciliationUnknown, kubehealth.AvailabilityUnknown, kind+" has ambiguous Ready conditions", kind+" availability is ambiguous", conditions), nil
	}
	if ready != nil {
		if ready.stale(obj.GetGeneration()) {
			availability, message := issuerAvailability(kind, ready.Status)
			return assessment(kubehealth.ReconciliationInProgress, availability, "Waiting for "+kind+" status to observe the latest generation", message, conditions), nil
		}
		switch ready.Status {
		case corev1.ConditionTrue:
			return assessment(kubehealth.ReconciliationReconciled, kubehealth.AvailabilityAvailable, ready.Message, kind+" is ready to issue certificates", conditions), nil
		case corev1.ConditionFalse:
			return assessment(kubehealth.ReconciliationFailed, kubehealth.AvailabilityUnavailable, ready.Message, kind+" is not ready", conditions), nil
		case corev1.ConditionUnknown:
			return assessment(kubehealth.ReconciliationUnknown, kubehealth.AvailabilityUnknown, ready.Message, kind+" availability is unknown", conditions), nil
		}
	}
	if conditionsPresent && len(statusConditions) > 0 {
		return assessment(kubehealth.ReconciliationUnknown, kubehealth.AvailabilityUnknown, kind+" status does not report readiness", kind+" availability is not reported", conditions), nil
	}
	return assessment(kubehealth.ReconciliationInProgress, kubehealth.AvailabilityUnknown, "Initializing "+kind, kind+" availability is not reported", conditions), nil
}

func assessment(reconciliation kubehealth.ReconciliationStatus, availability kubehealth.AvailabilityStatus, reconciliationMessage, availabilityMessage string, conditions []kubehealth.Condition) kubehealth.Assessment {
	return kubehealth.Assessment{
		Reconciliation: reconciliation, Availability: availability,
		Lifecycle: kubehealth.LifecycleActive, ReconciliationMessage: reconciliationMessage,
		AvailabilityMessage: availabilityMessage, Conditions: conditions,
	}
}

func issuerAvailability(kind string, status corev1.ConditionStatus) (kubehealth.AvailabilityStatus, string) {
	switch status {
	case corev1.ConditionTrue:
		return kubehealth.AvailabilityAvailable, kind + " is ready to issue certificates"
	case corev1.ConditionFalse:
		return kubehealth.AvailabilityUnavailable, kind + " is not ready"
	default:
		return kubehealth.AvailabilityUnknown, kind + " availability is unknown"
	}
}
