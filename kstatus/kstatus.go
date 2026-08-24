// Package kstatus projects KubeHealth assessments into upstream kstatus types.
package kstatus

import (
	"github.com/kubehealth-project/kubehealth"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	status "sigs.k8s.io/cli-utils/pkg/kstatus/status"
)

// Assess assesses a resource with KubeHealth and projects the result into a
// single kstatus-compatible value.
func Assess(obj *unstructured.Unstructured) (*status.Result, error) {
	assessment, err := kubehealth.Assess(obj)
	return Project(assessment), err
}

// Project converts a KubeHealth assessment into one kstatus result.
func Project(assessment kubehealth.Assessment) *status.Result {
	switch assessment.Lifecycle {
	case kubehealth.LifecycleTerminating:
		return result(assessment, status.TerminatingStatus, assessment.LifecycleMessage)
	case kubehealth.LifecycleNotFound:
		return result(assessment, status.NotFoundStatus, assessment.LifecycleMessage)
	case kubehealth.LifecycleUnknown:
		return result(assessment, status.UnknownStatus, assessment.LifecycleMessage)
	}

	switch assessment.Reconciliation {
	case kubehealth.ReconciliationReconciled:
		return result(assessment, status.CurrentStatus, assessment.ReconciliationMessage)
	case kubehealth.ReconciliationInProgress:
		return result(assessment, status.InProgressStatus, assessment.ReconciliationMessage)
	case kubehealth.ReconciliationFailed:
		return result(assessment, status.FailedStatus, assessment.ReconciliationMessage)
	case kubehealth.ReconciliationSuspended:
		return result(assessment, status.InProgressStatus, assessment.ReconciliationMessage)
	default:
		return result(assessment, status.UnknownStatus, assessment.ReconciliationMessage)
	}
}

func result(assessment kubehealth.Assessment, code status.Status, message string) *status.Result {
	conditions := make([]status.Condition, 0, len(assessment.Conditions))
	for _, condition := range assessment.Conditions {
		conditions = append(conditions, status.Condition{
			Type: status.ConditionType(condition.Type), Status: condition.Status,
			Reason: condition.Reason, Message: condition.Message,
		})
	}
	return &status.Result{Status: code, Message: message, Conditions: conditions}
}
