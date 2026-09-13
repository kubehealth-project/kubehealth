package builtins

import (
	"fmt"

	"github.com/kubehealth-project/kubehealth/api"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func assessService(obj *unstructured.Unstructured) (api.Assessment, error) {
	var service corev1.Service
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &service); err != nil {
		return api.Assessment{}, fmt.Errorf("convert Service: %w", err)
	}
	if service.Spec.Type == corev1.ServiceTypeLoadBalancer && len(service.Status.LoadBalancer.Ingress) == 0 {
		return api.Assessment{
			Reconciliation: api.Dimension[api.ReconciliationStatus]{Status: api.ReconciliationInProgress, Message: "Waiting for load balancer ingress"},
			Availability:   api.Dimension[api.AvailabilityStatus]{Status: api.AvailabilityUnavailable, Message: "Load balancer ingress is not assigned"},
			Lifecycle:      api.Dimension[api.LifecycleStatus]{Status: api.LifecycleActive},
		}, nil
	}
	availability := api.AvailabilityUnknown
	availabilityMessage := "Service availability requires EndpointSlice information"
	if service.Spec.Type == corev1.ServiceTypeLoadBalancer {
		availability = api.AvailabilityAvailable
		availabilityMessage = "Load balancer ingress is assigned"
	}
	return api.Assessment{
		Reconciliation: api.Dimension[api.ReconciliationStatus]{Status: api.ReconciliationReconciled, Message: "Service is reconciled"},
		Availability:   api.Dimension[api.AvailabilityStatus]{Status: availability, Message: availabilityMessage},
		Lifecycle:      api.Dimension[api.LifecycleStatus]{Status: api.LifecycleActive},
	}, nil
}
