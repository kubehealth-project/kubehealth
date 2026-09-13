package gatewayapi

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kubehealth-project/kubehealth"
)

func route(obj *unstructured.Unstructured) (kubehealth.Assessment, error) {
	desired, _, err := unstructured.NestedSlice(obj.Object, "spec", "parentRefs")
	if err != nil {
		return kubehealth.Assessment{}, fmt.Errorf("read spec.parentRefs: %w", err)
	}
	if len(desired) == 0 {
		return kubehealth.Assessment{
			Reconciliation: kubehealth.ReconciliationReconciled, Availability: kubehealth.AvailabilityNotApplicable,
			Lifecycle: kubehealth.LifecycleActive, ReconciliationMessage: "Route does not request attachment to a parent",
			AvailabilityMessage: "Route availability is not applicable without a parent attachment",
		}, nil
	}
	parents, _, err := unstructured.NestedSlice(obj.Object, "status", "parents")
	if err != nil {
		return kubehealth.Assessment{}, fmt.Errorf("read status.parents: %w", err)
	}
	assessments := make([]attachmentAssessment, 0, len(parents)+len(desired))
	seen := make(map[string]bool)
	var reported []kubehealth.Condition
	for _, item := range parents {
		parent, ok := item.(map[string]any)
		if !ok {
			return kubehealth.Assessment{}, fmt.Errorf("expected route parent status to be an object, got %T", item)
		}
		ref, _ := parent["parentRef"].(map[string]any)
		key := referenceKey(ref, obj.GetNamespace())
		if !desiredReference(desired, key, obj.GetNamespace()) {
			continue
		}
		seen[key] = true
		label := referenceLabel("Parent", ref, obj.GetNamespace())
		conditions, err := parseConditions(parent["conditions"])
		if err != nil {
			return kubehealth.Assessment{}, fmt.Errorf("read %s conditions: %w", label, err)
		}
		reported = append(reported, publicConditions(conditions, label)...)
		assessments = append(assessments, assessRouteAttachment(conditions, obj.GetGeneration(), label))
	}
	for _, item := range desired {
		ref, ok := item.(map[string]any)
		if !ok {
			return kubehealth.Assessment{}, fmt.Errorf("expected parentRef to be an object, got %T", item)
		}
		key := referenceKey(ref, obj.GetNamespace())
		if !seen[key] {
			label := referenceLabel("Parent", ref, obj.GetNamespace())
			assessments = append(assessments, attachmentAssessment{
				reconciliation: kubehealth.ReconciliationInProgress, availability: kubehealth.AvailabilityUnknown,
				reconciliationMessage: "Waiting for status from " + label,
				availabilityMessage:   label + " has not reported Route availability",
			})
		}
	}
	reconciliation, availability, reconciliationMessage, availabilityMessage := aggregateAttachments(assessments)
	return kubehealth.Assessment{
		Reconciliation: reconciliation, Availability: availability, Lifecycle: kubehealth.LifecycleActive,
		ReconciliationMessage: reconciliationMessage, AvailabilityMessage: availabilityMessage, Conditions: reported,
	}, nil
}

func assessRouteAttachment(conditions []condition, generation int64, label string) attachmentAssessment {
	accepted := findCondition(conditions, "Accepted")
	resolved := findCondition(conditions, "ResolvedRefs")
	partial := findCondition(conditions, "PartiallyInvalid")
	programmed := findCondition(conditions, "Programmed")
	result := attachmentAssessment{reconciliation: kubehealth.ReconciliationReconciled, availability: kubehealth.AvailabilityUnknown}

	for _, decisive := range []*condition{accepted, resolved, partial, programmed} {
		if isStale(decisive, generation) {
			result.reconciliation = kubehealth.ReconciliationInProgress
			result.reconciliationMessage = label + " has not observed the latest Route generation"
			break
		}
	}
	if isCurrent(accepted, generation) && accepted.Status == corev1.ConditionFalse {
		result.reconciliation = kubehealth.ReconciliationFailed
		result.reconciliationMessage = label + ": " + conditionMessage(accepted, "Route is not accepted")
	} else if isCurrent(resolved, generation) && resolved.Status == corev1.ConditionFalse {
		result.reconciliation = kubehealth.ReconciliationFailed
		result.reconciliationMessage = label + ": " + conditionMessage(resolved, "Route references are not resolved")
	} else if isCurrent(partial, generation) && partial.Status == corev1.ConditionTrue {
		result.reconciliation = kubehealth.ReconciliationFailed
		result.reconciliationMessage = label + ": " + conditionMessage(partial, "Route contains invalid rules")
	} else if accepted == nil || accepted.Status == corev1.ConditionUnknown {
		result.reconciliation = kubehealth.ReconciliationInProgress
		result.reconciliationMessage = "Waiting for Route acceptance from " + label
	} else if isCurrent(programmed, generation) && programmed.Status != corev1.ConditionTrue {
		result.reconciliation = kubehealth.ReconciliationInProgress
		result.reconciliationMessage = label + ": " + conditionMessage(programmed, "Route is still being programmed")
	}

	switch {
	case accepted == nil || accepted.Status == corev1.ConditionUnknown:
		result.availability = kubehealth.AvailabilityUnknown
		result.availabilityMessage = label + " has not reported Route availability"
	case accepted.Status == corev1.ConditionFalse:
		result.availability = kubehealth.AvailabilityUnavailable
		result.availabilityMessage = label + " has not accepted the Route"
	case accepted.Status == corev1.ConditionTrue && resolved != nil && resolved.Status == corev1.ConditionFalse:
		result.availability = kubehealth.AvailabilityPartiallyAvailable
		result.availabilityMessage = label + " accepted the Route, but some references are unresolved"
	case accepted.Status == corev1.ConditionTrue && partial != nil && partial.Status == corev1.ConditionTrue:
		result.availability = kubehealth.AvailabilityPartiallyAvailable
		result.availabilityMessage = label + " implements only the valid Route rules"
	case programmed != nil && programmed.Status != corev1.ConditionTrue:
		result.availability = kubehealth.AvailabilityUnavailable
		result.availabilityMessage = label + " has not programmed the Route"
	default:
		result.availability = kubehealth.AvailabilityAvailable
		result.availabilityMessage = label + " has accepted the Route; Gateway and backend reachability are not verified"
	}
	return result
}

func desiredReference(desired []any, key, namespace string) bool {
	for _, item := range desired {
		if ref, ok := item.(map[string]any); ok && referenceKey(ref, namespace) == key {
			return true
		}
	}
	return false
}

func referenceKey(ref map[string]any, namespace string) string {
	groupName := stringValue(ref["group"])
	if _, found := ref["group"]; !found {
		groupName = group
	}
	kind := stringValue(ref["kind"])
	if kind == "" {
		kind = "Gateway"
	}
	refNamespace := stringValue(ref["namespace"])
	if refNamespace == "" {
		refNamespace = namespace
	}
	port, _ := int64Value(ref["port"])
	return strings.Join([]string{groupName, kind, refNamespace, stringValue(ref["name"]), stringValue(ref["sectionName"]), fmt.Sprint(port)}, "\x00")
}

func referenceLabel(prefix string, ref map[string]any, namespace string) string {
	name := stringValue(ref["name"])
	if name == "" {
		name = "<unknown>"
	}
	refNamespace := stringValue(ref["namespace"])
	if refNamespace == "" {
		refNamespace = namespace
	}
	label := prefix + " " + refNamespace + "/" + name
	if section := stringValue(ref["sectionName"]); section != "" {
		label += " section " + section
	}
	return label
}
