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
			Reconciliation: api.ReconciliationReconciled, Availability: api.AvailabilityUnknown, Lifecycle: api.LifecycleActive,
			ReconciliationMessage: "PVC is bound", AvailabilityMessage: "Bound does not prove the volume can be mounted or used",
		}, nil
	case corev1.ClaimLost:
		return api.Assessment{
			Reconciliation: api.ReconciliationFailed, Availability: api.AvailabilityUnavailable, Lifecycle: api.LifecycleActive,
			ReconciliationMessage: "PVC is lost", AvailabilityMessage: "The bound volume is lost",
		}, nil
	default:
		return api.Assessment{
			Reconciliation: api.ReconciliationInProgress, Availability: api.AvailabilityUnavailable, Lifecycle: api.LifecycleActive,
			ReconciliationMessage: fmt.Sprintf("PVC is not bound. Phase: %s", claim.Status.Phase), AvailabilityMessage: "PVC is not bound",
		}, nil
	}
}
