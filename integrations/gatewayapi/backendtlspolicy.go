package gatewayapi

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kubehealth-project/kubehealth"
)

func backendTLSPolicy(obj *unstructured.Unstructured) (kubehealth.Assessment, error) {
	ancestors, _, err := unstructured.NestedSlice(obj.Object, "status", "ancestors")
	if err != nil {
		return kubehealth.Assessment{}, fmt.Errorf("read status.ancestors: %w", err)
	}
	if len(ancestors) == 0 {
		return kubehealth.Assessment{
			Reconciliation: kubehealth.Dimension[kubehealth.ReconciliationStatus]{Status: kubehealth.ReconciliationInProgress, Message: "Waiting for BackendTLSPolicy status"},
			Availability:   kubehealth.Dimension[kubehealth.AvailabilityStatus]{Status: kubehealth.AvailabilityUnknown, Message: "BackendTLSPolicy has no reported ancestors"},
			Lifecycle:      kubehealth.Dimension[kubehealth.LifecycleStatus]{Status: kubehealth.LifecycleActive},
		}, nil
	}
	assessments := make([]attachmentAssessment, 0, len(ancestors))
	var reported []kubehealth.Condition
	for _, item := range ancestors {
		ancestor, ok := item.(map[string]any)
		if !ok {
			return kubehealth.Assessment{}, fmt.Errorf("expected policy ancestor status to be an object, got %T", item)
		}
		ref, _ := ancestor["ancestorRef"].(map[string]any)
		label := referenceLabel("Ancestor", ref, obj.GetNamespace())
		conditions, err := parseConditions(ancestor["conditions"])
		if err != nil {
			return kubehealth.Assessment{}, fmt.Errorf("read %s conditions: %w", label, err)
		}
		reported = append(reported, publicConditions(conditions, label)...)
		assessments = append(assessments, assessPolicyAncestor(conditions, obj.GetGeneration(), label))
	}
	reconciliation, availability, reconciliationMessage, availabilityMessage := aggregateAttachments(assessments)
	return kubehealth.Assessment{
		Reconciliation: kubehealth.Dimension[kubehealth.ReconciliationStatus]{Status: reconciliation, Message: reconciliationMessage},
		Availability:   kubehealth.Dimension[kubehealth.AvailabilityStatus]{Status: availability, Message: availabilityMessage},
		Lifecycle:      kubehealth.Dimension[kubehealth.LifecycleStatus]{Status: kubehealth.LifecycleActive},
	}, nil
}

func assessPolicyAncestor(conditions []condition, generation int64, label string) attachmentAssessment {
	accepted := findCondition(conditions, "Accepted")
	resolved := findCondition(conditions, "ResolvedRefs")
	result := attachmentAssessment{reconciliation: kubehealth.ReconciliationReconciled, availability: kubehealth.AvailabilityUnknown}
	if isStale(accepted, generation) || isStale(resolved, generation) {
		result.reconciliation = kubehealth.ReconciliationInProgress
		result.reconciliationMessage = label + " has not observed the latest BackendTLSPolicy generation"
	}
	if isCurrent(accepted, generation) && accepted.Status == corev1.ConditionFalse {
		result.reconciliation = kubehealth.ReconciliationFailed
		result.reconciliationMessage = label + ": " + conditionMessage(accepted, "BackendTLSPolicy is not accepted")
	} else if isCurrent(resolved, generation) && resolved.Status == corev1.ConditionFalse {
		result.reconciliation = kubehealth.ReconciliationFailed
		result.reconciliationMessage = label + ": " + conditionMessage(resolved, "BackendTLSPolicy references are not resolved")
	} else if accepted == nil || accepted.Status == corev1.ConditionUnknown {
		result.reconciliation = kubehealth.ReconciliationInProgress
		result.reconciliationMessage = "Waiting for BackendTLSPolicy acceptance from " + label
	}

	switch {
	case accepted == nil || accepted.Status == corev1.ConditionUnknown:
		result.availability = kubehealth.AvailabilityUnknown
		result.availabilityMessage = label + " has not reported BackendTLSPolicy availability"
	case accepted.Status == corev1.ConditionFalse || resolved != nil && resolved.Status == corev1.ConditionFalse:
		result.availability = kubehealth.AvailabilityUnavailable
		result.availabilityMessage = label + " cannot use the BackendTLSPolicy"
	default:
		result.availability = kubehealth.AvailabilityAvailable
		result.availabilityMessage = label + " can use the BackendTLSPolicy; TLS handshakes and backend reachability are not verified"
	}
	return result
}
