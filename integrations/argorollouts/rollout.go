package argorollouts

import (
	"fmt"
	"strconv"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kubehealth-project/kubehealth"
)

const workloadGenerationAnnotation = "rollout.argoproj.io/workload-generation"

// rollout implements the current status semantics documented at
// https://argo-rollouts.readthedocs.io/en/stable/features/specification/.
func rollout(obj *unstructured.Unstructured) (kubehealth.Assessment, error) {
	conditions, err := kubehealth.GetConditions(obj)
	if err != nil {
		return kubehealth.Assessment{}, err
	}
	availability, availabilityMessage := rolloutAvailability(obj)
	result := func(status kubehealth.ReconciliationStatus, message string) kubehealth.Assessment {
		return kubehealth.Assessment{
			Reconciliation: status, Availability: availability, Lifecycle: kubehealth.LifecycleActive,
			ReconciliationMessage: message, AvailabilityMessage: availabilityMessage,
			Conditions: conditions,
		}
	}

	if !generationObserved(obj) || !workloadGenerationObserved(obj) {
		return result(kubehealth.ReconciliationInProgress, "Waiting for rollout spec update to be observed"), nil
	}

	if phase, found, _ := unstructured.NestedString(obj.Object, "status", "phase"); found {
		message, _, _ := unstructured.NestedString(obj.Object, "status", "message")
		switch phase {
		case "Healthy":
			return result(kubehealth.ReconciliationReconciled, message), nil
		case "Progressing":
			return result(kubehealth.ReconciliationInProgress, message), nil
		case "Degraded":
			return result(kubehealth.ReconciliationFailed, message), nil
		case "Paused":
			return result(kubehealth.ReconciliationSuspended, message), nil
		default:
			return result(kubehealth.ReconciliationUnknown, message), nil
		}
	}

	for _, condition := range conditions {
		if condition.Type == "InvalidSpec" ||
			(condition.Type == "Progressing" && (condition.Reason == "RolloutAborted" || condition.Reason == "ProgressDeadlineExceeded")) {
			return result(kubehealth.ReconciliationFailed, condition.Message), nil
		}
	}

	if paused(obj) {
		return result(kubehealth.ReconciliationSuspended, "Rollout is paused"), nil
	}

	currentPodHash, found, _ := unstructured.NestedString(obj.Object, "status", "currentPodHash")
	if !found {
		return result(kubehealth.ReconciliationInProgress, "Waiting for rollout to finish: status has not been reconciled."), nil
	}

	desired := nestedInt64(obj.Object, 1, "spec", "replicas")
	replicas := nestedInt64(obj.Object, 0, "status", "replicas")
	updated := nestedInt64(obj.Object, 0, "status", "updatedReplicas")
	available := nestedInt64(obj.Object, 0, "status", "availableReplicas")
	if updated < desired {
		return result(kubehealth.ReconciliationInProgress, "Waiting for roll out to finish: More replicas need to be updated"), nil
	}
	if available < updated {
		return result(kubehealth.ReconciliationInProgress, "Waiting for roll out to finish: updated replicas are still becoming available"), nil
	}

	stableRS, _, _ := unstructured.NestedString(obj.Object, "status", "stableRS")
	if _, blueGreen, _ := unstructured.NestedMap(obj.Object, "spec", "strategy", "blueGreen"); blueGreen {
		activeSelector, _, _ := unstructured.NestedString(obj.Object, "status", "blueGreen", "activeSelector")
		if activeSelector != currentPodHash {
			return result(kubehealth.ReconciliationInProgress, "active service cutover pending"), nil
		}
		if stableRS != "" && stableRS != currentPodHash {
			return result(kubehealth.ReconciliationInProgress, "waiting for analysis to complete"), nil
		}
	} else if _, canary, _ := unstructured.NestedMap(obj.Object, "spec", "strategy", "canary"); canary {
		if replicas > updated {
			return result(kubehealth.ReconciliationInProgress, "Waiting for roll out to finish: old replicas are pending termination"), nil
		}
		if stableRS == "" || stableRS != currentPodHash {
			return result(kubehealth.ReconciliationInProgress, "Waiting for rollout to finish steps"), nil
		}
	}

	return result(kubehealth.ReconciliationReconciled, "Rollout is reconciled"), nil
}

func rolloutAvailability(obj *unstructured.Unstructured) (kubehealth.AvailabilityStatus, string) {
	desired := nestedInt64(obj.Object, 1, "spec", "replicas")
	available := nestedInt64(obj.Object, 0, "status", "availableReplicas")
	switch {
	case desired == 0:
		return kubehealth.AvailabilityNotApplicable, "Rollout is scaled to zero"
	case available == 0:
		return kubehealth.AvailabilityUnavailable, "No Rollout replicas are available"
	case available < desired:
		return kubehealth.AvailabilityPartiallyAvailable, fmt.Sprintf("%d of %d desired replicas are available", available, desired)
	default:
		return kubehealth.AvailabilityAvailable, fmt.Sprintf("%d replicas are available", available)
	}
}

func generationObserved(obj *unstructured.Unstructured) bool {
	if _, found, _ := unstructured.NestedMap(obj.Object, "status"); !found {
		return false
	}
	observed, found := numberAt(obj.Object, "status", "observedGeneration")
	if !found {
		return false
	}
	return observed > obj.GetGeneration() || observed == obj.GetGeneration()
}

func workloadGenerationObserved(obj *unstructured.Unstructured) bool {
	if _, found, _ := unstructured.NestedMap(obj.Object, "spec", "workloadRef"); !found || obj.GetAnnotations() == nil {
		return true
	}
	desiredText, desiredFound := obj.GetAnnotations()[workloadGenerationAnnotation]
	desired, desiredValid := parseInt64(desiredText)
	observed, observedFound := numberAt(obj.Object, "status", "workloadObservedGeneration")
	if !desiredFound && !observedFound {
		return true
	}
	return desiredFound && desiredValid && observedFound && desired == observed
}

func paused(obj *unstructured.Unstructured) bool {
	if value, found, _ := unstructured.NestedBool(obj.Object, "spec", "paused"); found && value {
		return true
	}
	items, found, _ := unstructured.NestedSlice(obj.Object, "status", "pauseConditions")
	return found && len(items) > 0
}

func nestedInt64(object map[string]any, fallback int64, fields ...string) int64 {
	if value, found := numberAt(object, fields...); found {
		return value
	}
	return fallback
}

func numberAt(object map[string]any, fields ...string) (int64, bool) {
	value, found, err := unstructured.NestedFieldNoCopy(object, fields...)
	if err != nil || !found {
		return 0, false
	}
	switch typed := value.(type) {
	case int64:
		return typed, true
	case int32:
		return int64(typed), true
	case int:
		return int64(typed), true
	case float64:
		return int64(typed), typed == float64(int64(typed))
	case string:
		return parseInt64(typed)
	default:
		return 0, false
	}
}

func parseInt64(value string) (int64, bool) {
	parsed, err := strconv.ParseInt(value, 10, 64)
	return parsed, err == nil
}
