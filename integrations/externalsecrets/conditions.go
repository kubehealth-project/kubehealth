package externalsecrets

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/kubehealth-project/kubehealth"
)

type readyState int

const (
	readyMissing readyState = iota
	readyTrue
	readyFalse
	readyUnknown
	readyAmbiguous
)

func classifyReady(conditions []kubehealth.Condition) (readyState, *kubehealth.Condition) {
	var ready *kubehealth.Condition
	for i := range conditions {
		if conditions[i].Type != "Ready" {
			continue
		}
		if ready != nil {
			return readyAmbiguous, nil
		}
		ready = &conditions[i]
	}
	if ready == nil {
		return readyMissing, nil
	}
	switch ready.Status {
	case corev1.ConditionTrue:
		return readyTrue, ready
	case corev1.ConditionFalse:
		return readyFalse, ready
	default:
		return readyUnknown, ready
	}
}

func assessment(reconciliation kubehealth.ReconciliationStatus, availability kubehealth.AvailabilityStatus, reconciliationMessage, availabilityMessage string, conditions []kubehealth.Condition) kubehealth.Assessment {
	return kubehealth.Assessment{
		Reconciliation: kubehealth.Dimension[kubehealth.ReconciliationStatus]{Status: reconciliation, Message: reconciliationMessage},
		Availability:   kubehealth.Dimension[kubehealth.AvailabilityStatus]{Status: availability, Message: availabilityMessage},
		Lifecycle:      kubehealth.Dimension[kubehealth.LifecycleStatus]{Status: kubehealth.LifecycleActive},
	}
}

func readyUnknownAssessment(kind string, state readyState, condition *kubehealth.Condition, conditions []kubehealth.Condition) kubehealth.Assessment {
	if state == readyMissing {
		return assessment(
			kubehealth.ReconciliationInProgress,
			kubehealth.AvailabilityUnknown,
			"Waiting for "+kind+" status",
			kind+" availability is not reported",
			conditions,
		)
	}
	message := "The Ready condition is unknown"
	if state == readyAmbiguous {
		message = "Multiple Ready conditions make " + kind + " status ambiguous"
	} else if condition != nil && condition.Message != "" {
		message = condition.Message
	}
	return assessment(
		kubehealth.ReconciliationUnknown,
		kubehealth.AvailabilityUnknown,
		message,
		kind+" availability is unknown",
		conditions,
	)
}
