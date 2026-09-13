package builtins

import (
	"fmt"

	"github.com/kubehealth-project/kubehealth/api"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func assessPersistentVolumeClaim(obj *unstructured.Unstructured) (api.Assessment, error) {
	var claim corev1.PersistentVolumeClaim
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &claim); err != nil {
		return api.Assessment{}, fmt.Errorf("convert PersistentVolumeClaim: %w", err)
	}
	switch claim.Status.Phase {
	case corev1.ClaimBound:
		return api.Assessment{
			Reconciliation: api.Dimension[api.ReconciliationStatus]{Status: api.ReconciliationReconciled, Message: "PVC is bound"},
			Availability:   api.Dimension[api.AvailabilityStatus]{Status: api.AvailabilityUnknown, Message: "Bound does not prove the volume can be mounted or used"},
			Lifecycle:      api.Dimension[api.LifecycleStatus]{Status: api.LifecycleActive},
		}, nil
	case corev1.ClaimLost:
		return api.Assessment{
			Reconciliation: api.Dimension[api.ReconciliationStatus]{Status: api.ReconciliationFailed, Message: "PVC is lost"},
			Availability:   api.Dimension[api.AvailabilityStatus]{Status: api.AvailabilityUnavailable, Message: "The bound volume is lost"},
			Lifecycle:      api.Dimension[api.LifecycleStatus]{Status: api.LifecycleActive},
		}, nil
	default:
		return api.Assessment{
			Reconciliation: api.Dimension[api.ReconciliationStatus]{Status: api.ReconciliationInProgress, Message: fmt.Sprintf("PVC is not bound. Phase: %s", claim.Status.Phase)},
			Availability:   api.Dimension[api.AvailabilityStatus]{Status: api.AvailabilityUnavailable, Message: "PVC is not bound"},
			Lifecycle:      api.Dimension[api.LifecycleStatus]{Status: api.LifecycleActive},
		}, nil
	}
}
