package kyverno

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kubehealth-project/kubehealth"
)

func policy(obj *unstructured.Unstructured) (kubehealth.Assessment, error) {
	statusConditions, conditions, conditionsPresent, err := readStatusConditions(obj)
	if err != nil {
		return kubehealth.Assessment{}, err
	}
	kind := obj.GetKind()
	ready, ambiguous := readyCondition(statusConditions)
	if ambiguous {
		return policyAssessment(kubehealth.ReconciliationUnknown, kubehealth.AvailabilityUnknown, kind+" has ambiguous Ready conditions", kind+" availability is ambiguous", conditions), nil
	}
	if ready == nil {
		if conditionsPresent && len(statusConditions) > 0 {
			return policyAssessment(kubehealth.ReconciliationUnknown, kubehealth.AvailabilityUnknown, kind+" status does not report readiness", kind+" availability is not reported", conditions), nil
		}
		return policyAssessment(kubehealth.ReconciliationInProgress, kubehealth.AvailabilityUnknown, "Waiting for "+kind+" to be ready", kind+" availability is not reported", conditions), nil
	}

	if (ready.Status == corev1.ConditionTrue && ready.Reason == "Failed") ||
		(ready.Status == corev1.ConditionFalse && ready.Reason == "Succeeded") {
		return policyAssessment(kubehealth.ReconciliationUnknown, kubehealth.AvailabilityUnknown, "Ready condition has contradictory status and reason", kind+" availability is ambiguous", conditions), nil
	}
	if ready.stale(obj.GetGeneration()) {
		availability, message := policyAvailability(kind, ready.Status)
		return policyAssessment(kubehealth.ReconciliationInProgress, availability, "Waiting for "+kind+" status to observe the latest generation", message, conditions), nil
	}

	switch ready.Status {
	case corev1.ConditionTrue:
		return policyAssessment(kubehealth.ReconciliationReconciled, kubehealth.AvailabilityAvailable, ready.Message, kind+" is ready to serve admission requests", conditions), nil
	case corev1.ConditionFalse:
		return policyAssessment(kubehealth.ReconciliationFailed, kubehealth.AvailabilityUnavailable, ready.Message, kind+" is not ready to serve admission requests", conditions), nil
	default:
		return policyAssessment(kubehealth.ReconciliationUnknown, kubehealth.AvailabilityUnknown, ready.Message, kind+" availability is unknown", conditions), nil
	}
}

func policyAssessment(reconciliation kubehealth.ReconciliationStatus, availability kubehealth.AvailabilityStatus, reconciliationMessage, availabilityMessage string, conditions []kubehealth.Condition) kubehealth.Assessment {
	return kubehealth.Assessment{
		Reconciliation: kubehealth.Dimension[kubehealth.ReconciliationStatus]{Status: reconciliation, Message: reconciliationMessage},
		Availability:   kubehealth.Dimension[kubehealth.AvailabilityStatus]{Status: availability, Message: availabilityMessage},
		Lifecycle:      kubehealth.Dimension[kubehealth.LifecycleStatus]{Status: kubehealth.LifecycleActive},
	}
}

func policyAvailability(kind string, status corev1.ConditionStatus) (kubehealth.AvailabilityStatus, string) {
	switch status {
	case corev1.ConditionTrue:
		return kubehealth.AvailabilityAvailable, kind + " is ready to serve admission requests"
	case corev1.ConditionFalse:
		return kubehealth.AvailabilityUnavailable, kind + " is not ready to serve admission requests"
	default:
		return kubehealth.AvailabilityUnknown, kind + " availability is unknown"
	}
}
