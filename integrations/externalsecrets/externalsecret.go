package externalsecrets

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kubehealth-project/kubehealth"
)

func externalSecret(obj *unstructured.Unstructured) (kubehealth.Assessment, error) {
	conditions, err := kubehealth.GetConditions(obj)
	if err != nil {
		return kubehealth.Assessment{}, err
	}
	state, ready := classifyReady(conditions)
	switch state {
	case readyTrue:
		availability := kubehealth.AvailabilityAvailable
		availabilityMessage := "ExternalSecret target is synchronized"
		switch ready.Reason {
		case "SecretDeleted", "ResourceDeleted":
			availability = kubehealth.AvailabilityUnavailable
			availabilityMessage = "ExternalSecret target was deleted"
		case "SecretMissing", "ResourceMissing":
			availability = kubehealth.AvailabilityUnavailable
			availabilityMessage = "ExternalSecret target is absent"
		}
		return assessment(kubehealth.ReconciliationReconciled, availability, ready.Message, availabilityMessage, conditions), nil
	case readyFalse:
		return assessment(
			kubehealth.ReconciliationFailed,
			kubehealth.AvailabilityUnknown,
			ready.Message,
			"Target availability cannot be determined after synchronization failure",
			conditions,
		), nil
	default:
		return readyUnknownAssessment("ExternalSecret", state, ready, conditions), nil
	}
}
