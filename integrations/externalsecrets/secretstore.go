package externalsecrets

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kubehealth-project/kubehealth"
)

func secretStore(obj *unstructured.Unstructured) (kubehealth.Assessment, error) {
	conditions, err := kubehealth.GetConditions(obj)
	if err != nil {
		return kubehealth.Assessment{}, err
	}
	kind := obj.GetKind()
	state, ready := classifyReady(conditions)
	switch state {
	case readyTrue:
		if ready.Reason == "ValidationUnknown" {
			return assessment(
				kubehealth.ReconciliationReconciled,
				kubehealth.AvailabilityUnknown,
				ready.Message,
				"Store availability could not be validated",
				conditions,
			), nil
		}
		return assessment(kubehealth.ReconciliationReconciled, kubehealth.AvailabilityAvailable, ready.Message, "Store is ready to serve requests", conditions), nil
	case readyFalse:
		return assessment(kubehealth.ReconciliationFailed, kubehealth.AvailabilityUnavailable, ready.Message, "Store is not ready to serve requests", conditions), nil
	default:
		return readyUnknownAssessment(kind, state, ready, conditions), nil
	}
}
