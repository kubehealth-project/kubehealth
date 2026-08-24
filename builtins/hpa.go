package builtins

import (
	"fmt"

	"github.com/kubehealth-project/kubehealth/api"
	"github.com/kubehealth-project/kubehealth/internal/conditions"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var degradedHPAConditions = map[string]map[string]bool{
	"AbleToScale": {
		"FailedGetScale":    true,
		"FailedUpdateScale": true,
	},
	"ScalingActive": {
		"FailedGetResourceMetric":      true,
		"FailedGetObjectMetric":        true,
		"FailedGetPodsMetric":          true,
		"FailedGetExternalMetric":      true,
		"FailedComputeMetricsReplicas": true,
		"InvalidSelector":              true,
	},
}

func assessHPAV1(_ *unstructured.Unstructured) (api.Assessment, error) {
	return api.Assessment{
		Reconciliation: api.ReconciliationUnknown, Availability: api.AvailabilityNotApplicable,
		Lifecycle:             api.LifecycleActive,
		ReconciliationMessage: "autoscaling/v1 does not expose standardized HPA conditions",
	}, nil
}

func assessHPAV2(obj *unstructured.Unstructured) (api.Assessment, error) {
	parsed, err := conditions.Get(obj)
	if err != nil {
		return api.Assessment{}, fmt.Errorf("read HPA conditions: %w", err)
	}
	return assessHPAConditions(parsed), nil
}

func assessHPAConditions(conditions []api.Condition) api.Assessment {
	for _, condition := range conditions {
		if degradedHPAConditions[condition.Type][condition.Reason] {
			return hpaAssessment(api.ReconciliationFailed, condition)
		}
	}
	for _, condition := range conditions {
		if (condition.Type == "AbleToScale" || condition.Type == "ScalingLimited") && condition.Status == "True" {
			return hpaAssessment(api.ReconciliationReconciled, condition)
		}
	}
	return progressingHPA()
}

func hpaAssessment(status api.ReconciliationStatus, condition api.Condition) api.Assessment {
	return api.Assessment{
		Reconciliation: status, Availability: api.AvailabilityNotApplicable, Lifecycle: api.LifecycleActive,
		ReconciliationMessage: condition.Message,
		Conditions:            []api.Condition{condition},
	}
}

func progressingHPA() api.Assessment {
	return api.Assessment{
		Reconciliation: api.ReconciliationInProgress, Availability: api.AvailabilityNotApplicable,
		Lifecycle: api.LifecycleActive, ReconciliationMessage: "Waiting to Autoscale",
	}
}
