package builtins

import (
	"fmt"

	"github.com/kubehealth-project/kubehealth/api"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func assessDeployment(obj *unstructured.Unstructured) (api.Assessment, error) {
	var deployment appsv1.Deployment
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &deployment); err != nil {
		return api.Assessment{}, fmt.Errorf("convert Deployment: %w", err)
	}
	availability, availabilityMessage := deploymentAvailability(&deployment)
	result := func(status api.ReconciliationStatus, message string) api.Assessment {
		return api.Assessment{
			Reconciliation: status, Availability: availability, Lifecycle: api.LifecycleActive,
			ReconciliationMessage: message,
			AvailabilityMessage:   availabilityMessage,
		}
	}
	if deployment.Spec.Paused {
		return result(api.ReconciliationSuspended, "Deployment is paused"), nil
	}
	for _, condition := range deployment.Status.Conditions {
		if condition.Type == appsv1.DeploymentProgressing && condition.Reason == "ProgressDeadlineExceeded" {
			return result(api.ReconciliationFailed, fmt.Sprintf("Deployment %q exceeded its progress deadline", deployment.Name)), nil
		}
	}
	if deployment.Spec.Replicas != nil && deployment.Status.UpdatedReplicas < *deployment.Spec.Replicas {
		return result(api.ReconciliationInProgress, fmt.Sprintf("Updated: %d/%d", deployment.Status.UpdatedReplicas, *deployment.Spec.Replicas)), nil
	}
	if deployment.Status.Replicas > deployment.Status.UpdatedReplicas {
		return result(api.ReconciliationInProgress, fmt.Sprintf("Pending termination: %d", deployment.Status.Replicas-deployment.Status.UpdatedReplicas)), nil
	}
	if deployment.Status.AvailableReplicas < deployment.Status.UpdatedReplicas {
		return result(api.ReconciliationInProgress, fmt.Sprintf("Available: %d/%d", deployment.Status.AvailableReplicas, deployment.Status.UpdatedReplicas)), nil
	}
	return result(api.ReconciliationReconciled, "Deployment is reconciled"), nil
}

func deploymentAvailability(deployment *appsv1.Deployment) (api.AvailabilityStatus, string) {
	desired := int32(1)
	if deployment.Spec.Replicas != nil {
		desired = *deployment.Spec.Replicas
	}
	available := deployment.Status.AvailableReplicas
	switch {
	case desired == 0:
		return api.AvailabilityNotApplicable, "Deployment is scaled to zero"
	case available == 0:
		return api.AvailabilityUnavailable, "No Deployment replicas are available"
	case available < desired:
		return api.AvailabilityPartiallyAvailable, fmt.Sprintf("%d of %d desired replicas are available", available, desired)
	default:
		return api.AvailabilityAvailable, fmt.Sprintf("%d replicas are available", available)
	}
}
