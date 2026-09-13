package externalsecrets

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kubehealth-project/kubehealth"
)

func pushSecret(obj *unstructured.Unstructured) (kubehealth.Assessment, error) {
	conditions, err := kubehealth.GetConditions(obj)
	if err != nil {
		return kubehealth.Assessment{}, err
	}
	state, ready := classifyReady(conditions)
	switch state {
	case readyTrue:
		return assessment(kubehealth.ReconciliationReconciled, kubehealth.AvailabilityAvailable, ready.Message, "Remote provider state is synchronized", conditions), nil
	case readyFalse:
		if ready.Reason == "SourceDeleted" {
			synced, found, err := unstructured.NestedMap(obj.Object, "status", "syncedPushSecrets")
			if err != nil {
				return kubehealth.Assessment{}, err
			}
			if found && len(synced) > 0 {
				return assessment(
					kubehealth.ReconciliationReconciled,
					kubehealth.AvailabilityUnknown,
					ready.Message,
					"Source deletion conflicts with recorded synchronized provider state",
					conditions,
				), nil
			}
			return assessment(kubehealth.ReconciliationReconciled, kubehealth.AvailabilityUnavailable, ready.Message, "Source was deleted and provider secrets were cleaned up", conditions), nil
		}
		return assessment(
			kubehealth.ReconciliationFailed,
			kubehealth.AvailabilityUnknown,
			ready.Message,
			"Remote provider state cannot be determined after push failure",
			conditions,
		), nil
	default:
		return readyUnknownAssessment("PushSecret", state, ready, conditions), nil
	}
}
