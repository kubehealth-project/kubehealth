package externalsecrets

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kubehealth-project/kubehealth"
)

func clusterExternalSecret(obj *unstructured.Unstructured) (kubehealth.Assessment, error) {
	conditions, err := kubehealth.GetConditions(obj)
	if err != nil {
		return kubehealth.Assessment{}, err
	}
	state, ready := classifyReady(conditions)
	switch state {
	case readyTrue:
		return assessment(
			kubehealth.ReconciliationReconciled,
			kubehealth.AvailabilityAvailable,
			ready.Message,
			"All selected namespaces were provisioned; child ExternalSecret health is not evaluated",
			conditions,
		), nil
	case readyFalse:
		failed, err := stringSliceLength(obj, "failedNamespaces")
		if err != nil {
			return kubehealth.Assessment{}, err
		}
		provisioned, err := stringSliceLength(obj, "provisionedNamespaces")
		if err != nil {
			return kubehealth.Assessment{}, err
		}
		switch {
		case failed == 0:
			return assessment(
				kubehealth.ReconciliationFailed,
				kubehealth.AvailabilityUnknown,
				ready.Message,
				"Ready is false but no failed namespaces are reported",
				conditions,
			), nil
		case provisioned > 0:
			return assessment(
				kubehealth.ReconciliationFailed,
				kubehealth.AvailabilityPartiallyAvailable,
				ready.Message,
				fmt.Sprintf("Provisioned %d namespace(s), while %d failed; child ExternalSecret health is not evaluated", provisioned, failed),
				conditions,
			), nil
		default:
			return assessment(
				kubehealth.ReconciliationFailed,
				kubehealth.AvailabilityUnavailable,
				ready.Message,
				fmt.Sprintf("Failed to provision %d namespace(s); child ExternalSecret health is not evaluated", failed),
				conditions,
			), nil
		}
	default:
		return readyUnknownAssessment("ClusterExternalSecret", state, ready, conditions), nil
	}
}

func stringSliceLength(obj *unstructured.Unstructured, field string) (int, error) {
	value, found, err := unstructured.NestedSlice(obj.Object, "status", field)
	if err != nil {
		return 0, fmt.Errorf("read status.%s: %w", field, err)
	}
	if !found {
		return 0, nil
	}
	return len(value), nil
}
