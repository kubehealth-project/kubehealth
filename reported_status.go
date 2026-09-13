package kubehealth

import (
	"encoding/json"
	"fmt"

	"github.com/kubehealth-project/kubehealth/api"
	healthv1alpha1 "github.com/kubehealth-project/kubehealth/api/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type reportedStatusState int

const (
	reportedStatusAbsent reportedStatusState = iota
	reportedStatusIncomplete
	reportedStatusStale
	reportedStatusCurrent
)

func readReportedStatus(obj *unstructured.Unstructured) (Assessment, reportedStatusState, error) {
	value, found, err := unstructured.NestedFieldNoCopy(obj.Object, "status", "kubeHealth")
	if err != nil {
		return Assessment{}, reportedStatusAbsent, fmt.Errorf("read status.kubeHealth: %w", err)
	}
	if !found {
		return Assessment{}, reportedStatusAbsent, nil
	}

	object, ok := value.(map[string]any)
	if !ok {
		return Assessment{}, reportedStatusAbsent, fmt.Errorf("read status.kubeHealth: expected an object, got %T", value)
	}
	complete, err := reportedStatusComplete(object)
	if err != nil {
		return Assessment{}, reportedStatusIncomplete, err
	}
	if !complete {
		return Assessment{}, reportedStatusIncomplete, nil
	}

	encoded, err := json.Marshal(object)
	if err != nil {
		return Assessment{}, reportedStatusIncomplete, fmt.Errorf("encode status.kubeHealth: %w", err)
	}
	var status healthv1alpha1.Status
	if err := json.Unmarshal(encoded, &status); err != nil {
		return Assessment{}, reportedStatusIncomplete, fmt.Errorf("decode status.kubeHealth: %w", err)
	}
	if err := status.Validate(); err != nil {
		return Assessment{}, reportedStatusIncomplete, fmt.Errorf("validate status.kubeHealth: %w", err)
	}

	assessment := status.ToAssessment()
	deleting := obj.GetDeletionTimestamp() != nil
	if !deleting && status.Lifecycle.Status == api.LifecycleTerminating {
		return Assessment{}, reportedStatusIncomplete, fmt.Errorf(
			"validate status.kubeHealth: lifecycle is %q but metadata.deletionTimestamp is not set",
			status.Lifecycle.Status,
		)
	}

	if status.ObservedGeneration > obj.GetGeneration() {
		return Assessment{}, reportedStatusIncomplete, fmt.Errorf(
			"validate status.kubeHealth: observedGeneration %d is newer than metadata.generation %d",
			status.ObservedGeneration, obj.GetGeneration(),
		)
	}
	if status.ObservedGeneration < obj.GetGeneration() {
		message := fmt.Sprintf(
			"%s generation is %d, but status.kubeHealth observed generation is %d",
			obj.GetKind(), obj.GetGeneration(), status.ObservedGeneration,
		)
		return Assessment{
			Reconciliation: Dimension[ReconciliationStatus]{Status: ReconciliationInProgress, Reason: "LatestGenerationNotObserved", Message: message},
			Availability:   assessment.Availability,
			Lifecycle:      Dimension[LifecycleStatus]{Status: lifecycleFromMetadata(obj), Message: lifecycleMessageFromMetadata(obj)},
		}, reportedStatusStale, nil
	}

	if deleting && status.Lifecycle.Status != api.LifecycleTerminating {
		return Assessment{}, reportedStatusIncomplete, nil
	}

	return assessment, reportedStatusCurrent, nil
}

func reportedStatusComplete(object map[string]any) (bool, error) {
	if _, found := object["contractVersion"]; !found {
		return false, nil
	}
	if _, found := object["observedGeneration"]; !found {
		return false, nil
	}
	for _, name := range []string{"reconciliation", "availability", "lifecycle"} {
		if _, found := object[name]; !found {
			return false, nil
		}
	}
	for _, name := range []string{"reconciliation", "availability", "lifecycle"} {
		value := object[name]
		dimension, ok := value.(map[string]any)
		if !ok {
			return false, fmt.Errorf("read status.kubeHealth.%s: expected an object, got %T", name, value)
		}
		if _, found := dimension["status"]; !found {
			return false, nil
		}
	}
	return true, nil
}

func lifecycleFromMetadata(obj *unstructured.Unstructured) LifecycleStatus {
	if obj.GetDeletionTimestamp() != nil {
		return LifecycleTerminating
	}
	return LifecycleActive
}

func lifecycleMessageFromMetadata(obj *unstructured.Unstructured) string {
	if obj.GetDeletionTimestamp() != nil {
		return "Resource scheduled for deletion"
	}
	return ""
}
