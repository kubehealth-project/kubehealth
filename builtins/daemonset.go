package builtins

import (
	"fmt"

	"github.com/kubehealth-project/kubehealth/api"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func assessDaemonSet(obj *unstructured.Unstructured) (api.Assessment, error) {
	var daemonSet appsv1.DaemonSet
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &daemonSet); err != nil {
		return api.Assessment{}, fmt.Errorf("convert DaemonSet: %w", err)
	}

	availability, availabilityMessage := daemonSetAvailability(&daemonSet)
	result := func(reconciliation api.ReconciliationStatus, message string) api.Assessment {
		return api.Assessment{
			Reconciliation: api.Dimension[api.ReconciliationStatus]{Status: reconciliation, Message: message},
			Availability:   api.Dimension[api.AvailabilityStatus]{Status: availability, Message: availabilityMessage},
			Lifecycle:      api.Dimension[api.LifecycleStatus]{Status: api.LifecycleActive},
		}
	}

	if daemonSet.Spec.UpdateStrategy.Type == appsv1.OnDeleteDaemonSetStrategyType {
		return result(api.ReconciliationReconciled, fmt.Sprintf("DaemonSet has updated %d of %d scheduled Pods", daemonSet.Status.UpdatedNumberScheduled, daemonSet.Status.DesiredNumberScheduled)), nil
	}
	if daemonSet.Status.UpdatedNumberScheduled < daemonSet.Status.DesiredNumberScheduled {
		return result(api.ReconciliationInProgress, fmt.Sprintf("Updated: %d/%d", daemonSet.Status.UpdatedNumberScheduled, daemonSet.Status.DesiredNumberScheduled)), nil
	}
	if daemonSet.Status.NumberAvailable < daemonSet.Status.DesiredNumberScheduled {
		return result(api.ReconciliationInProgress, fmt.Sprintf("Available: %d/%d", daemonSet.Status.NumberAvailable, daemonSet.Status.DesiredNumberScheduled)), nil
	}
	return result(api.ReconciliationReconciled, "DaemonSet is reconciled"), nil
}

func daemonSetAvailability(daemonSet *appsv1.DaemonSet) (api.AvailabilityStatus, string) {
	desired := daemonSet.Status.DesiredNumberScheduled
	available := daemonSet.Status.NumberAvailable
	switch {
	case desired == 0:
		return api.AvailabilityNotApplicable, "DaemonSet has no desired Pods"
	case available == 0:
		return api.AvailabilityUnavailable, "No DaemonSet Pods are available"
	case available < desired:
		return api.AvailabilityPartiallyAvailable, fmt.Sprintf("%d of %d desired Pods are available", available, desired)
	default:
		return api.AvailabilityAvailable, fmt.Sprintf("%d Pods are available", available)
	}
}
