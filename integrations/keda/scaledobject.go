package keda

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kubehealth-project/kubehealth"
)

func scaledObject(obj *unstructured.Unstructured) (kubehealth.Assessment, error) {
	conditions, err := kubehealth.GetConditions(obj)
	if err != nil {
		return kubehealth.Assessment{}, err
	}
	var ready, paused *kubehealth.Condition
	for i := range conditions {
		condition := &conditions[i]
		if condition.Type == "Fallback" && condition.Status == corev1.ConditionTrue {
			return scaledObjectAssessment(kubehealth.ReconciliationFailed, condition.Message, conditions), nil
		}
		if condition.Type == "Ready" {
			ready = condition
		}
		if condition.Type == "Paused" && condition.Status == corev1.ConditionTrue {
			paused = condition
		}
	}
	if ready != nil && ready.Status == corev1.ConditionFalse {
		return scaledObjectAssessment(kubehealth.ReconciliationFailed, ready.Message, conditions), nil
	}
	if ready != nil && ready.Status == corev1.ConditionTrue {
		message := ready.Message
		if paused != nil {
			message = paused.Message
		}
		return scaledObjectAssessment(kubehealth.ReconciliationReconciled, message, conditions), nil
	}
	return scaledObjectAssessment(kubehealth.ReconciliationInProgress, "Creating HorizontalPodAutoscaler Object", conditions), nil
}

func scaledObjectAssessment(reconciliation kubehealth.ReconciliationStatus, message string, conditions []kubehealth.Condition) kubehealth.Assessment {
	return kubehealth.Assessment{
		Reconciliation:        reconciliation,
		Availability:          kubehealth.AvailabilityNotApplicable,
		Lifecycle:             kubehealth.LifecycleActive,
		ReconciliationMessage: message,
		AvailabilityMessage:   "ScaledObject controls another resource and is not directly available",
		Conditions:            conditions,
	}
}
