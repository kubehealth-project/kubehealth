package certmanager

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kubehealth-project/kubehealth"
)

func certificate(obj *unstructured.Unstructured) (kubehealth.Assessment, error) {
	statusConditions, conditions, conditionsPresent, err := readStatusConditions(obj)
	if err != nil {
		return kubehealth.Assessment{}, err
	}
	ready, ambiguousReady := conditionByType(statusConditions, "Ready")
	issuing, ambiguousIssuing := conditionByType(statusConditions, "Issuing")
	if ambiguousReady || ambiguousIssuing {
		return assessment(kubehealth.ReconciliationUnknown, kubehealth.AvailabilityUnknown, "Certificate has ambiguous status conditions", "Certificate availability is ambiguous", conditions), nil
	}

	if issuing != nil && issuing.Status == corev1.ConditionTrue {
		availability, message := certificateAvailability(ready)
		return assessment(kubehealth.ReconciliationInProgress, availability, issuing.Message, message, conditions), nil
	}
	if issuing != nil && issuing.Status == corev1.ConditionUnknown {
		availability, message := certificateAvailability(ready)
		return assessment(kubehealth.ReconciliationUnknown, availability, issuing.Message, message, conditions), nil
	}
	if ready != nil {
		if ready.stale(obj.GetGeneration()) {
			availability, message := certificateAvailability(ready)
			return assessment(kubehealth.ReconciliationInProgress, availability, "Waiting for certificate status to observe the latest generation", message, conditions), nil
		}
		switch ready.Status {
		case corev1.ConditionTrue:
			if certificateHasLatestFailure(obj) {
				return assessment(kubehealth.ReconciliationFailed, kubehealth.AvailabilityAvailable, "The latest certificate issuance failed", "The current certificate remains ready", conditions), nil
			}
			return assessment(kubehealth.ReconciliationReconciled, kubehealth.AvailabilityAvailable, ready.Message, "Certificate is ready", conditions), nil
		case corev1.ConditionFalse:
			return assessment(kubehealth.ReconciliationFailed, kubehealth.AvailabilityUnavailable, ready.Message, "Certificate is not ready", conditions), nil
		case corev1.ConditionUnknown:
			return assessment(kubehealth.ReconciliationUnknown, kubehealth.AvailabilityUnknown, ready.Message, "Certificate availability is unknown", conditions), nil
		}
	}
	if conditionsPresent && len(statusConditions) > 0 {
		return assessment(kubehealth.ReconciliationUnknown, kubehealth.AvailabilityUnknown, "Certificate status does not report readiness", "Certificate availability is not reported", conditions), nil
	}
	return assessment(kubehealth.ReconciliationInProgress, kubehealth.AvailabilityUnknown, "Waiting for certificate", "Certificate availability is not reported", conditions), nil
}

func certificateAvailability(ready *statusCondition) (kubehealth.AvailabilityStatus, string) {
	if ready == nil {
		return kubehealth.AvailabilityUnknown, "Certificate availability is not reported"
	}
	switch ready.Status {
	case corev1.ConditionTrue:
		return kubehealth.AvailabilityAvailable, "The current certificate remains ready"
	case corev1.ConditionFalse:
		return kubehealth.AvailabilityUnavailable, "Certificate is not ready"
	default:
		return kubehealth.AvailabilityUnknown, "Certificate availability is unknown"
	}
}

func certificateHasLatestFailure(obj *unstructured.Unstructured) bool {
	if _, found, _ := unstructured.NestedString(obj.Object, "status", "lastFailureTime"); found {
		return true
	}
	attempts, found, _ := unstructured.NestedInt64(obj.Object, "status", "failedIssuanceAttempts")
	return found && attempts > 0
}
