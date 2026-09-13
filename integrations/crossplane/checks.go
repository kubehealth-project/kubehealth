package crossplane

import (
	"fmt"

	"github.com/kubehealth-project/kubehealth"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func managedResourceCheck(obj *unstructured.Unstructured) (kubehealth.Assessment, error) {
	conditions, err := kubehealth.GetConditions(obj)
	if err != nil {
		return kubehealth.Assessment{}, err
	}

	ready := findCondition(conditions, "Ready")
	synced := findCondition(conditions, "Synced")
	lastAsync := findCondition(conditions, "LastAsyncOperation")

	reconciliation, reconciliationMessage := managedReconciliation(synced, lastAsync)
	availability, availabilityMessage := conditionAvailability(ready, obj.GetKind())
	return assessment(reconciliation, availability, reconciliationMessage, availabilityMessage, conditions), nil
}

func managedReconciliation(synced, lastAsync *kubehealth.Condition) (kubehealth.ReconciliationStatus, string) {
	if synced != nil && synced.Status == corev1.ConditionFalse && synced.Reason == "ReconcilePaused" {
		return kubehealth.ReconciliationSuspended, conditionMessage(synced, "Reconciliation is paused")
	}
	if lastAsync != nil && lastAsync.Status == corev1.ConditionFalse {
		return kubehealth.ReconciliationFailed, conditionMessage(lastAsync, "The last asynchronous operation failed")
	}
	if synced == nil {
		return kubehealth.ReconciliationInProgress, "Waiting for the first successful reconciliation"
	}
	switch synced.Status {
	case corev1.ConditionTrue:
		return kubehealth.ReconciliationReconciled, conditionMessage(synced, "Resource is synced")
	case corev1.ConditionFalse:
		return kubehealth.ReconciliationFailed, conditionMessage(synced, "Resource reconciliation failed")
	default:
		return kubehealth.ReconciliationUnknown, conditionMessage(synced, "Resource reconciliation status is unknown")
	}
}

func packageCheck(obj *unstructured.Unstructured) (kubehealth.Assessment, error) {
	conditions, err := kubehealth.GetConditions(obj)
	if err != nil {
		return kubehealth.Assessment{}, err
	}
	installed := findCondition(conditions, "Installed")
	healthy := findCondition(conditions, "Healthy")

	reconciliation := kubehealth.ReconciliationInProgress
	reconciliationMessage := "Waiting for package installation"
	if installed != nil {
		switch installed.Status {
		case corev1.ConditionTrue:
			reconciliation = kubehealth.ReconciliationReconciled
			reconciliationMessage = conditionMessage(installed, "Package is installed")
		case corev1.ConditionFalse:
			reconciliationMessage = conditionMessage(installed, "Package installation is in progress")
		default:
			reconciliation = kubehealth.ReconciliationUnknown
			reconciliationMessage = conditionMessage(installed, "Package installation status is unknown")
		}
	}

	availability, availabilityMessage := conditionAvailability(healthy, "Package")
	return assessment(reconciliation, availability, reconciliationMessage, availabilityMessage, conditions), nil
}

func packageRevisionCheck(obj *unstructured.Unstructured) (kubehealth.Assessment, error) {
	conditions, err := kubehealth.GetConditions(obj)
	if err != nil {
		return kubehealth.Assessment{}, err
	}
	if desiredState, _, _ := unstructured.NestedString(obj.Object, "spec", "desiredState"); desiredState == "Inactive" {
		return assessment(
			kubehealth.ReconciliationSuspended,
			kubehealth.AvailabilityNotApplicable,
			"Package revision is intentionally inactive",
			"An inactive package revision is not expected to serve",
			conditions,
		), nil
	}

	revisionHealthy := findCondition(conditions, "RevisionHealthy")
	reconciliation, reconciliationMessage := booleanConditionReconciliation(
		revisionHealthy,
		"Waiting for package revision health",
		"Package revision is healthy",
		"Package revision is unhealthy",
	)

	availabilityCondition := revisionHealthy
	capability := "Package revision"
	if obj.GetKind() == "ProviderRevision" || obj.GetKind() == "FunctionRevision" {
		if runtimeHealthy := findCondition(conditions, "RuntimeHealthy"); runtimeHealthy != nil {
			availabilityCondition = runtimeHealthy
			capability = "Package runtime"
		}
	}
	availability, availabilityMessage := conditionAvailability(availabilityCondition, capability)
	return assessment(reconciliation, availability, reconciliationMessage, availabilityMessage, conditions), nil
}

func declarativeCheck(obj *unstructured.Unstructured) (kubehealth.Assessment, error) {
	conditions, err := kubehealth.GetConditions(obj)
	if err != nil {
		return kubehealth.Assessment{}, err
	}
	return assessment(
		kubehealth.ReconciliationNotApplicable,
		kubehealth.AvailabilityNotApplicable,
		fmt.Sprintf("%s is declarative configuration", obj.GetKind()),
		fmt.Sprintf("%s does not report an object-local serving capability", obj.GetKind()),
		conditions,
	), nil
}

func compositionRevisionCheck(obj *unstructured.Unstructured) (kubehealth.Assessment, error) {
	conditions, err := kubehealth.GetConditions(obj)
	if err != nil {
		return kubehealth.Assessment{}, err
	}
	reconciliation, message := booleanConditionReconciliation(
		findCondition(conditions, "ValidPipeline"),
		"Waiting for composition pipeline validation",
		"Composition pipeline is valid",
		"Composition pipeline is invalid",
	)
	return assessment(
		reconciliation,
		kubehealth.AvailabilityNotApplicable,
		message,
		"A composition revision does not serve independently",
		conditions,
	), nil
}

func xrdCheck(obj *unstructured.Unstructured) (kubehealth.Assessment, error) {
	conditions, err := kubehealth.GetConditions(obj)
	if err != nil {
		return kubehealth.Assessment{}, err
	}
	established := findCondition(conditions, "Established")
	if established == nil {
		return assessment(
			kubehealth.ReconciliationInProgress,
			kubehealth.AvailabilityUnknown,
			"Waiting for the composite resource API to be established",
			"Composite resource API availability is not reported",
			conditions,
		), nil
	}
	switch established.Status {
	case corev1.ConditionTrue:
		return assessment(
			kubehealth.ReconciliationReconciled,
			kubehealth.AvailabilityAvailable,
			conditionMessage(established, "Composite resource API is established"),
			"Composite resource API is established",
			conditions,
		), nil
	case corev1.ConditionFalse:
		return assessment(
			kubehealth.ReconciliationInProgress,
			kubehealth.AvailabilityUnavailable,
			conditionMessage(established, "Composite resource API is not established"),
			"Composite resource API is not established",
			conditions,
		), nil
	default:
		return assessment(
			kubehealth.ReconciliationUnknown,
			kubehealth.AvailabilityUnknown,
			conditionMessage(established, "Composite resource API establishment is unknown"),
			"Composite resource API availability is unknown",
			conditions,
		), nil
	}
}

func booleanConditionReconciliation(condition *kubehealth.Condition, waiting, success, failure string) (kubehealth.ReconciliationStatus, string) {
	if condition == nil {
		return kubehealth.ReconciliationInProgress, waiting
	}
	switch condition.Status {
	case corev1.ConditionTrue:
		return kubehealth.ReconciliationReconciled, conditionMessage(condition, success)
	case corev1.ConditionFalse:
		return kubehealth.ReconciliationFailed, conditionMessage(condition, failure)
	default:
		return kubehealth.ReconciliationUnknown, conditionMessage(condition, waiting)
	}
}

func conditionAvailability(condition *kubehealth.Condition, capability string) (kubehealth.AvailabilityStatus, string) {
	if condition == nil {
		return kubehealth.AvailabilityUnknown, capability + " availability is not reported"
	}
	switch condition.Status {
	case corev1.ConditionTrue:
		return kubehealth.AvailabilityAvailable, conditionMessage(condition, capability+" is available")
	case corev1.ConditionFalse:
		return kubehealth.AvailabilityUnavailable, conditionMessage(condition, capability+" is unavailable")
	default:
		return kubehealth.AvailabilityUnknown, conditionMessage(condition, capability+" availability is unknown")
	}
}

func conditionMessage(condition *kubehealth.Condition, fallback string) string {
	if condition.Message != "" {
		return condition.Message
	}
	if condition.Reason != "" {
		return condition.Reason
	}
	return fallback
}

func assessment(reconciliation kubehealth.ReconciliationStatus, availability kubehealth.AvailabilityStatus, reconciliationMessage, availabilityMessage string, conditions []kubehealth.Condition) kubehealth.Assessment {
	return kubehealth.Assessment{
		Reconciliation: kubehealth.Dimension[kubehealth.ReconciliationStatus]{Status: reconciliation, Message: reconciliationMessage},
		Availability:   kubehealth.Dimension[kubehealth.AvailabilityStatus]{Status: availability, Message: availabilityMessage},
		Lifecycle:      kubehealth.Dimension[kubehealth.LifecycleStatus]{Status: kubehealth.LifecycleActive},
	}
}

func findCondition(conditions []kubehealth.Condition, conditionType string) *kubehealth.Condition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}
