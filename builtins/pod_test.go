package builtins_test

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kubehealth-project/kubehealth"
)

func TestPod(t *testing.T) {
	tests := []struct {
		name               string
		make               func() *corev1.Pod
		wantReconciliation kubehealth.ReconciliationStatus
		wantAvailability   kubehealth.AvailabilityStatus
		wantMessage        string
	}{
		{name: "always ready", make: func() *corev1.Pod { return podScenario(corev1.PodRunning, corev1.RestartPolicyAlways, true) }, wantReconciliation: kubehealth.ReconciliationReconciled, wantAvailability: kubehealth.AvailabilityAvailable},
		{name: "finite ready", make: func() *corev1.Pod { return podScenario(corev1.PodRunning, corev1.RestartPolicyNever, true) }, wantReconciliation: kubehealth.ReconciliationInProgress, wantAvailability: kubehealth.AvailabilityAvailable},
		{name: "finite waiting error remains in progress", make: func() *corev1.Pod {
			pod := podScenario(corev1.PodRunning, corev1.RestartPolicyOnFailure, false)
			pod.Status.ContainerStatuses = []corev1.ContainerStatus{{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff", Message: "cannot pull"}}}}
			return pod
		}, wantReconciliation: kubehealth.ReconciliationInProgress, wantAvailability: kubehealth.AvailabilityUnavailable},
		{name: "waiting error remains in progress", make: func() *corev1.Pod {
			pod := podScenario(corev1.PodPending, corev1.RestartPolicyAlways, false)
			pod.Status.ContainerStatuses = []corev1.ContainerStatus{{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff", Message: "cannot pull"}}}}
			return pod
		}, wantReconciliation: kubehealth.ReconciliationInProgress, wantAvailability: kubehealth.AvailabilityUnavailable, wantMessage: "Pod is pending"},
		{name: "previous termination does not make current state failed", make: func() *corev1.Pod {
			pod := podScenario(corev1.PodRunning, corev1.RestartPolicyAlways, false)
			pod.Status.ContainerStatuses = []corev1.ContainerStatus{{LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 1}}}}
			return pod
		}, wantReconciliation: kubehealth.ReconciliationInProgress, wantAvailability: kubehealth.AvailabilityUnavailable, wantMessage: "Pod is running but not ready"},
		{name: "failed extracts exit code", make: func() *corev1.Pod {
			pod := podScenario(corev1.PodFailed, corev1.RestartPolicyNever, false)
			pod.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "main", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 12}}}}
			return pod
		}, wantReconciliation: kubehealth.ReconciliationFailed, wantAvailability: kubehealth.AvailabilityUnavailable, wantMessage: `container "main" failed with exit code 12`},
		{name: "succeeded", make: func() *corev1.Pod { return podScenario(corev1.PodSucceeded, corev1.RestartPolicyNever, false) }, wantReconciliation: kubehealth.ReconciliationReconciled, wantAvailability: kubehealth.AvailabilityNotApplicable},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assessment, err := newBuiltinsAssessor(t).Assess(unstructuredFrom(t, test.make()))
			if err != nil {
				t.Fatal(err)
			}
			if assessment.Reconciliation.Status != test.wantReconciliation || assessment.Availability.Status != test.wantAvailability {
				t.Fatalf("assessment = %#v", assessment)
			}
			if test.wantMessage != "" && assessment.Reconciliation.Message != test.wantMessage {
				t.Fatalf("message = %q, want %q", assessment.Reconciliation.Message, test.wantMessage)
			}
		})
	}
}

func podScenario(phase corev1.PodPhase, restartPolicy corev1.RestartPolicy, ready bool) *corev1.Pod {
	readyStatus := corev1.ConditionFalse
	if ready {
		readyStatus = corev1.ConditionTrue
	}
	return &corev1.Pod{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
		ObjectMeta: metav1.ObjectMeta{Name: "pod"},
		Spec:       corev1.PodSpec{RestartPolicy: restartPolicy},
		Status:     corev1.PodStatus{Phase: phase, Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: readyStatus}}},
	}
}
