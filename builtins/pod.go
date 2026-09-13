package builtins

import (
	"fmt"

	"github.com/kubehealth-project/kubehealth/api"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func assessPod(obj *unstructured.Unstructured) (api.Assessment, error) {
	var pod corev1.Pod
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &pod); err != nil {
		return api.Assessment{}, fmt.Errorf("convert Pod: %w", err)
	}

	switch pod.Status.Phase {
	case corev1.PodPending:
		return podAssessment(api.ReconciliationInProgress, api.AvailabilityUnavailable, messageOrDefault(pod.Status.Message, "Pod is pending"), "Pod is not ready"), nil
	case corev1.PodSucceeded:
		return podAssessment(api.ReconciliationReconciled, api.AvailabilityNotApplicable, messageOrDefault(pod.Status.Message, "Pod completed successfully"), "Pod completed successfully"), nil
	case corev1.PodFailed:
		return podAssessment(api.ReconciliationFailed, api.AvailabilityUnavailable, podFailureMessage(&pod), "Pod has failed"), nil
	case corev1.PodRunning:
		ready := podReady(&pod)
		switch pod.Spec.RestartPolicy {
		case corev1.RestartPolicyAlways:
			if ready {
				return podAssessment(api.ReconciliationReconciled, api.AvailabilityAvailable, messageOrDefault(pod.Status.Message, "Pod is ready"), "Pod Ready condition is true"), nil
			}
			return podAssessment(api.ReconciliationInProgress, api.AvailabilityUnavailable, messageOrDefault(pod.Status.Message, "Pod is running but not ready"), "Pod Ready condition is not true"), nil
		case corev1.RestartPolicyNever, corev1.RestartPolicyOnFailure:
			availability := api.AvailabilityUnavailable
			availabilityMessage := "Pod Ready condition is not true"
			if ready {
				availability = api.AvailabilityAvailable
				availabilityMessage = "Pod Ready condition is true"
			}
			return podAssessment(api.ReconciliationInProgress, availability, messageOrDefault(pod.Status.Message, "Pod is running"), availabilityMessage), nil
		default:
			return podAssessment(api.ReconciliationUnknown, api.AvailabilityUnknown, pod.Status.Message, "Pod restart policy is unknown"), nil
		}
	default:
		return podAssessment(api.ReconciliationUnknown, api.AvailabilityUnknown, pod.Status.Message, fmt.Sprintf("Unknown Pod phase %q", pod.Status.Phase)), nil
	}
}

func podAssessment(reconciliation api.ReconciliationStatus, availability api.AvailabilityStatus, reconciliationMessage, availabilityMessage string) api.Assessment {
	return api.Assessment{
		Reconciliation: api.Dimension[api.ReconciliationStatus]{Status: reconciliation, Message: reconciliationMessage},
		Availability:   api.Dimension[api.AvailabilityStatus]{Status: availability, Message: availabilityMessage},
		Lifecycle:      api.Dimension[api.LifecycleStatus]{Status: api.LifecycleActive},
	}
}

func podReady(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

func messageOrDefault(message, fallback string) string {
	if message != "" {
		return message
	}
	return fallback
}

func podFailureMessage(pod *corev1.Pod) string {
	if pod.Status.Message != "" {
		return pod.Status.Message
	}
	statuses := append(append([]corev1.ContainerStatus(nil), pod.Status.InitContainerStatuses...), pod.Status.ContainerStatuses...)
	for _, status := range statuses {
		terminated := status.State.Terminated
		if terminated == nil {
			continue
		}
		if terminated.Message != "" {
			return terminated.Message
		}
		if terminated.Reason == "OOMKilled" {
			return terminated.Reason
		}
		if terminated.ExitCode != 0 {
			return fmt.Sprintf("container %q failed with exit code %d", status.Name, terminated.ExitCode)
		}
	}
	return ""
}
