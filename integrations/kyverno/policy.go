package kyverno

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kubehealth-project/kubehealth"
)

func policy(obj *unstructured.Unstructured) (kubehealth.Assessment, error) {
	conditions, err := kubehealth.GetConditions(obj)
	if err != nil {
		return kubehealth.Assessment{}, err
	}
	for _, condition := range conditions {
		if condition.Type == "Ready" && condition.Status == corev1.ConditionTrue && condition.Reason == "Succeeded" {
			return kubehealth.Assessment{
				Reconciliation:        kubehealth.ReconciliationReconciled,
				Availability:          kubehealth.AvailabilityAvailable,
				Lifecycle:             kubehealth.LifecycleActive,
				ReconciliationMessage: "Policy is ready",
				AvailabilityMessage:   "Policy is active",
				Conditions:            conditions,
			}, nil
		}
	}
	return kubehealth.Assessment{
		Reconciliation:        kubehealth.ReconciliationInProgress,
		Availability:          kubehealth.AvailabilityUnavailable,
		Lifecycle:             kubehealth.LifecycleActive,
		ReconciliationMessage: "Waiting for Policy to be ready",
		AvailabilityMessage:   "Policy is not ready",
		Conditions:            conditions,
	}, nil
}
