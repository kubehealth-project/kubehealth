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
	switch assessment.Lifecycle.Status {
	case kubehealth.LifecycleTerminating:
		return result(assessment, status.TerminatingStatus, assessment.Lifecycle.Message)
	case kubehealth.LifecycleNotFound:
		return result(assessment, status.NotFoundStatus, assessment.Lifecycle.Message)
	case kubehealth.LifecycleUnknown:
		return result(assessment, status.UnknownStatus, assessment.Lifecycle.Message)
	}

	switch assessment.Reconciliation.Status {
	case kubehealth.ReconciliationReconciled:
		return result(assessment, status.CurrentStatus, assessment.Reconciliation.Message)
	case kubehealth.ReconciliationInProgress:
		return result(assessment, status.InProgressStatus, assessment.Reconciliation.Message)
	case kubehealth.ReconciliationFailed:
		return result(assessment, status.FailedStatus, assessment.Reconciliation.Message)
	case kubehealth.ReconciliationSuspended:
		return result(assessment, status.InProgressStatus, assessment.Reconciliation.Message)
	default:
		return result(assessment, status.UnknownStatus, assessment.Reconciliation.Message)
	}
}

func result(assessment kubehealth.Assessment, code status.Status, message string) *status.Result {
	return &status.Result{Status: code, Message: message}
}
