package prometheusoperator

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kubehealth-project/kubehealth"
)

// workload follows the Prometheus Operator status conditions documented at
// https://prometheus-operator.dev/docs/proposals/implemented/status-subresource/.
func workload(obj *unstructured.Unstructured) (kubehealth.Assessment, error) {
	conditions, conditionsPresent, err := readStatusConditions(obj)
	if err != nil {
		return kubehealth.Assessment{}, err
	}

	kind := obj.GetKind()
	reconciliation, reconciliationMessage := workloadReconciliation(obj, conditions, conditionsPresent, kind)
	availability, availabilityMessage := workloadAvailability(obj, conditions, kind)
	return kubehealth.Assessment{
		Reconciliation: kubehealth.Dimension[kubehealth.ReconciliationStatus]{Status: reconciliation.status, Reason: reconciliation.reason, Message: reconciliationMessage},
		Availability:   kubehealth.Dimension[kubehealth.AvailabilityStatus]{Status: availability.status, Reason: availability.reason, Message: availabilityMessage},
		Lifecycle:      kubehealth.Dimension[kubehealth.LifecycleStatus]{Status: kubehealth.LifecycleActive},
	}, nil
}

type reconciliationResult struct {
	status kubehealth.ReconciliationStatus
	reason string
}

type availabilityResult struct {
	status kubehealth.AvailabilityStatus
	reason string
}

func workloadReconciliation(obj *unstructured.Unstructured, conditions []statusCondition, conditionsPresent bool, kind string) (reconciliationResult, string) {
	if paused(obj) {
		return reconciliationResult{status: kubehealth.ReconciliationSuspended, reason: "Paused"}, kind + " reconciliation is paused"
	}

	condition, ambiguous := conditionByType(conditions, "Reconciled")
	if ambiguous {
		return reconciliationResult{status: kubehealth.ReconciliationUnknown}, kind + " has ambiguous Reconciled conditions"
	}
	if condition == nil {
		if !conditionsPresent || len(conditions) == 0 {
			return reconciliationResult{status: kubehealth.ReconciliationInProgress}, "Waiting for " + kind + " status"
		}
		return reconciliationResult{status: kubehealth.ReconciliationUnknown}, kind + " status does not report reconciliation"
	}
	if condition.stale(obj.GetGeneration()) {
		return reconciliationResult{status: kubehealth.ReconciliationInProgress}, "Waiting for " + kind + " status to observe the latest generation"
	}

	switch condition.Status {
	case "True":
		return reconciliationResult{status: kubehealth.ReconciliationReconciled, reason: condition.Reason}, conditionMessage(condition, kind+" reconciliation succeeded")
	case "False":
		return reconciliationResult{status: kubehealth.ReconciliationFailed, reason: condition.Reason}, conditionMessage(condition, kind+" reconciliation failed")
	default:
		return reconciliationResult{status: kubehealth.ReconciliationUnknown, reason: condition.Reason}, conditionMessage(condition, kind+" reconciliation status is unknown")
	}
}

func paused(obj *unstructured.Unstructured) bool {
	value, found, err := unstructured.NestedBool(obj.Object, "spec", "paused")
	return err == nil && found && value
}

func workloadAvailability(obj *unstructured.Unstructured, conditions []statusCondition, kind string) (availabilityResult, string) {
	condition, ambiguous := conditionByType(conditions, "Available")
	if ambiguous {
		return availabilityResult{status: kubehealth.AvailabilityUnknown}, kind + " has ambiguous Available conditions"
	}
	if condition == nil {
		return availabilityResult{status: kubehealth.AvailabilityUnknown}, kind + " availability is not reported"
	}
	if condition.stale(obj.GetGeneration()) {
		return availabilityResult{status: kubehealth.AvailabilityUnknown}, "Waiting for " + kind + " availability status to observe the latest generation"
	}

	switch condition.Status {
	case "True":
		return availabilityResult{status: kubehealth.AvailabilityAvailable, reason: condition.Reason}, conditionMessage(condition, "All "+kind+" instances are available")
	case "Degraded":
		return availabilityResult{status: kubehealth.AvailabilityPartiallyAvailable, reason: condition.Reason}, conditionMessage(condition, "Some "+kind+" instances are not ready")
	case "False":
		// Older Prometheus Operator versions used False with this reason for
		// partial availability; newer versions use the Degraded condition status.
		if condition.Reason == "SomePodsNotReady" {
			return availabilityResult{status: kubehealth.AvailabilityPartiallyAvailable, reason: condition.Reason}, conditionMessage(condition, "Some "+kind+" instances are not ready")
		}
		return availabilityResult{status: kubehealth.AvailabilityUnavailable, reason: condition.Reason}, conditionMessage(condition, "No "+kind+" instances are available")
	default:
		return availabilityResult{status: kubehealth.AvailabilityUnknown, reason: condition.Reason}, conditionMessage(condition, kind+" availability status is unknown")
	}
}

func conditionMessage(condition *statusCondition, fallback string) string {
	if condition.Message != "" {
		return condition.Message
	}
	if condition.Reason != "" {
		return condition.Reason
	}
	return fallback
}
