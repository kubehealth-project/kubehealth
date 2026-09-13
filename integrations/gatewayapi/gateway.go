package gatewayapi

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kubehealth-project/kubehealth"
)

func gateway(obj *unstructured.Unstructured) (kubehealth.Assessment, error) {
	topValue, _, err := unstructured.NestedFieldNoCopy(obj.Object, "status", "conditions")
	if err != nil {
		return kubehealth.Assessment{}, err
	}
	top, err := parseConditions(topValue)
	if err != nil {
		return kubehealth.Assessment{}, fmt.Errorf("read Gateway conditions: %w", err)
	}
	assessments := []attachmentAssessment{assessGatewayConditions(top, obj.GetGeneration(), "Gateway")}
	reported := publicConditions(top, "Gateway")
	listeners, _, err := unstructured.NestedSlice(obj.Object, "status", "listeners")
	if err != nil {
		return kubehealth.Assessment{}, fmt.Errorf("read status.listeners: %w", err)
	}
	listenerAssessments := make([]attachmentAssessment, 0, len(listeners))
	seen := make(map[string]bool)
	for _, item := range listeners {
		listener, ok := item.(map[string]any)
		if !ok {
			return kubehealth.Assessment{}, fmt.Errorf("expected listener status to be an object, got %T", item)
		}
		name := stringValue(listener["name"])
		seen[name] = true
		conditions, err := parseConditions(listener["conditions"])
		if err != nil {
			return kubehealth.Assessment{}, fmt.Errorf("read Listener %q conditions: %w", name, err)
		}
		label := "Listener " + name
		listenerAssessments = append(listenerAssessments, assessGatewayConditions(conditions, obj.GetGeneration(), label))
		reported = append(reported, publicConditions(conditions, label)...)
	}
	desired, _, err := unstructured.NestedSlice(obj.Object, "spec", "listeners")
	if err != nil {
		return kubehealth.Assessment{}, fmt.Errorf("read spec.listeners: %w", err)
	}
	for _, item := range desired {
		listener, _ := item.(map[string]any)
		name := stringValue(listener["name"])
		if !seen[name] {
			listenerAssessments = append(listenerAssessments, attachmentAssessment{
				reconciliation: kubehealth.ReconciliationInProgress, availability: kubehealth.AvailabilityUnknown,
				reconciliationMessage: "Waiting for status from Listener " + name,
				availabilityMessage:   "Listener " + name + " has not reported availability",
			})
		}
	}
	assessments = append(assessments, listenerAssessments...)
	reconciliation, _, reconciliationMessage, _ := aggregateAttachments(assessments)
	availabilityItems := listenerAssessments
	if len(availabilityItems) == 0 {
		availabilityItems = assessments[:1]
	}
	_, availability, _, availabilityMessage := aggregateAttachments(availabilityItems)
	return kubehealth.Assessment{
		Reconciliation: reconciliation, Availability: availability, Lifecycle: kubehealth.LifecycleActive,
		ReconciliationMessage: reconciliationMessage, AvailabilityMessage: availabilityMessage, Conditions: reported,
	}, nil
}

func assessGatewayConditions(conditions []condition, generation int64, label string) attachmentAssessment {
	accepted := findCondition(conditions, "Accepted")
	resolved := findCondition(conditions, "ResolvedRefs")
	programmed := findCondition(conditions, "Programmed")
	conflicted := findCondition(conditions, "Conflicted")
	result := attachmentAssessment{reconciliation: kubehealth.ReconciliationReconciled, availability: kubehealth.AvailabilityUnknown}

	for _, decisive := range []*condition{accepted, resolved, programmed, conflicted} {
		if isStale(decisive, generation) {
			result.reconciliation = kubehealth.ReconciliationInProgress
			result.reconciliationMessage = label + " has not observed the latest Gateway generation"
			break
		}
	}
	if isCurrent(accepted, generation) && accepted.Status == corev1.ConditionFalse {
		result.reconciliation = kubehealth.ReconciliationFailed
		result.reconciliationMessage = label + ": " + conditionMessage(accepted, "configuration is not accepted")
	} else if isCurrent(resolved, generation) && resolved.Status == corev1.ConditionFalse {
		result.reconciliation = kubehealth.ReconciliationFailed
		result.reconciliationMessage = label + ": " + conditionMessage(resolved, "references are not resolved")
	} else if isCurrent(conflicted, generation) && conflicted.Status == corev1.ConditionTrue {
		result.reconciliation = kubehealth.ReconciliationFailed
		result.reconciliationMessage = label + ": " + conditionMessage(conflicted, "configuration conflicts")
	} else if accepted == nil || programmed == nil || accepted.Status == corev1.ConditionUnknown || programmed.Status == corev1.ConditionUnknown {
		result.reconciliation = kubehealth.ReconciliationInProgress
		result.reconciliationMessage = "Waiting for " + label + " status"
	} else if isCurrent(programmed, generation) && programmed.Status == corev1.ConditionFalse {
		if programmed.Reason == "Invalid" {
			result.reconciliation = kubehealth.ReconciliationFailed
		} else {
			result.reconciliation = kubehealth.ReconciliationInProgress
		}
		result.reconciliationMessage = label + ": " + conditionMessage(programmed, "configuration is not programmed")
	}

	switch {
	case programmed == nil || programmed.Status == corev1.ConditionUnknown:
		result.availability = kubehealth.AvailabilityUnknown
		result.availabilityMessage = label + " has not reported whether its configuration is programmed"
	case programmed.Status == corev1.ConditionTrue:
		result.availability = kubehealth.AvailabilityAvailable
		result.availabilityMessage = label + " configuration is programmed; endpoint reachability is not verified"
	default:
		result.availability = kubehealth.AvailabilityUnavailable
		result.availabilityMessage = label + " configuration is not programmed"
	}
	return result
}
