package gatewayapi

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/kubehealth-project/kubehealth"
)

type condition struct {
	Type               string
	Status             corev1.ConditionStatus
	Reason             string
	Message            string
	ObservedGeneration *int64
}

type attachmentAssessment struct {
	reconciliation        kubehealth.ReconciliationStatus
	availability          kubehealth.AvailabilityStatus
	reconciliationMessage string
	availabilityMessage   string
}

func parseConditions(value any) ([]condition, error) {
	if value == nil {
		return nil, nil
	}
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("expected conditions to be an array, got %T", value)
	}
	result := make([]condition, 0, len(items))
	for _, item := range items {
		object, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("expected condition to be an object, got %T", item)
		}
		parsed := condition{
			Type: stringValue(object["type"]), Status: corev1.ConditionStatus(stringValue(object["status"])),
			Reason: stringValue(object["reason"]), Message: stringValue(object["message"]),
		}
		if value, found := int64Value(object["observedGeneration"]); found {
			parsed.ObservedGeneration = &value
		}
		result = append(result, parsed)
	}
	return result, nil
}

func findCondition(conditions []condition, conditionType string) *condition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}

func isCurrent(condition *condition, generation int64) bool {
	return condition != nil && (generation == 0 || condition.ObservedGeneration == nil || *condition.ObservedGeneration >= generation)
}

func isStale(condition *condition, generation int64) bool {
	return condition != nil && generation > 0 && condition.ObservedGeneration != nil && *condition.ObservedGeneration < generation
}

func publicConditions(conditions []condition, context string) []kubehealth.Condition {
	result := make([]kubehealth.Condition, 0, len(conditions))
	for _, condition := range conditions {
		message := condition.Message
		if context != "" {
			if message == "" {
				message = context
			} else {
				message = context + ": " + message
			}
		}
		result = append(result, kubehealth.Condition{
			Type: condition.Type, Status: condition.Status, Reason: condition.Reason, Message: message,
		})
	}
	return result
}

func aggregateAttachments(items []attachmentAssessment) (kubehealth.ReconciliationStatus, kubehealth.AvailabilityStatus, string, string) {
	reconciliation := kubehealth.ReconciliationReconciled
	availability := kubehealth.AvailabilityNotApplicable
	var reconciliationMessage, availabilityMessage string
	available, unavailable, partial, unknown := false, false, false, false
	for _, item := range items {
		switch item.reconciliation {
		case kubehealth.ReconciliationFailed:
			reconciliation = kubehealth.ReconciliationFailed
			reconciliationMessage = item.reconciliationMessage
		case kubehealth.ReconciliationInProgress:
			if reconciliation != kubehealth.ReconciliationFailed {
				reconciliation = kubehealth.ReconciliationInProgress
				reconciliationMessage = item.reconciliationMessage
			}
		case kubehealth.ReconciliationUnknown:
			if reconciliation == kubehealth.ReconciliationReconciled {
				reconciliation = kubehealth.ReconciliationUnknown
				reconciliationMessage = item.reconciliationMessage
			}
		}
		switch item.availability {
		case kubehealth.AvailabilityAvailable:
			available = true
		case kubehealth.AvailabilityUnavailable:
			unavailable = true
		case kubehealth.AvailabilityPartiallyAvailable:
			partial = true
		case kubehealth.AvailabilityUnknown:
			unknown = true
		}
		if item.availabilityMessage != "" {
			availabilityMessage = item.availabilityMessage
		}
	}
	switch {
	case partial || available && unavailable:
		availability = kubehealth.AvailabilityPartiallyAvailable
	case unknown:
		availability = kubehealth.AvailabilityUnknown
	case available:
		availability = kubehealth.AvailabilityAvailable
	case unavailable:
		availability = kubehealth.AvailabilityUnavailable
	}
	return reconciliation, availability, reconciliationMessage, availabilityMessage
}

func conditionMessage(condition *condition, fallback string) string {
	if condition != nil && condition.Message != "" {
		return condition.Message
	}
	return fallback
}

func stringValue(value any) string {
	result, _ := value.(string)
	return result
}

func int64Value(value any) (int64, bool) {
	switch typed := value.(type) {
	case int64:
		return typed, true
	case int32:
		return int64(typed), true
	case int:
		return int64(typed), true
	case float64:
		return int64(typed), typed == float64(int64(typed))
	default:
		return 0, false
	}
}
