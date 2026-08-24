package lua

import (
	"fmt"

	luavm "github.com/yuin/gopher-lua"
	corev1 "k8s.io/api/core/v1"

	"github.com/kubehealth-project/kubehealth/api"
)

func decodeAssessment(table *luavm.LTable) (api.Assessment, error) {
	reconciliation, err := requiredString(table, "reconciliation")
	if err != nil {
		return api.Assessment{}, err
	}
	availability, err := requiredString(table, "availability")
	if err != nil {
		return api.Assessment{}, err
	}
	lifecycle, err := requiredString(table, "lifecycle")
	if err != nil {
		return api.Assessment{}, err
	}

	reconciliationMessage, err := optionalString(table, "reconciliationMessage")
	if err != nil {
		return api.Assessment{}, err
	}
	availabilityMessage, err := optionalString(table, "availabilityMessage")
	if err != nil {
		return api.Assessment{}, err
	}
	lifecycleMessage, err := optionalString(table, "lifecycleMessage")
	if err != nil {
		return api.Assessment{}, err
	}

	assessment := api.Assessment{
		Reconciliation:        api.ReconciliationStatus(reconciliation),
		Availability:          api.AvailabilityStatus(availability),
		Lifecycle:             api.LifecycleStatus(lifecycle),
		ReconciliationMessage: reconciliationMessage,
		AvailabilityMessage:   availabilityMessage,
		LifecycleMessage:      lifecycleMessage,
	}
	if err := validateAssessment(assessment); err != nil {
		return api.Assessment{}, err
	}

	conditions, err := decodeConditions(table.RawGetString("conditions"))
	if err != nil {
		return api.Assessment{}, err
	}
	assessment.Conditions = conditions
	return assessment, nil
}

func validateAssessment(assessment api.Assessment) error {
	switch assessment.Reconciliation {
	case api.ReconciliationReconciled, api.ReconciliationInProgress,
		api.ReconciliationFailed, api.ReconciliationSuspended, api.ReconciliationUnknown,
		api.ReconciliationNotApplicable:
	default:
		return fmt.Errorf("Lua health check returned invalid reconciliation value %q", assessment.Reconciliation)
	}

	switch assessment.Availability {
	case api.AvailabilityAvailable, api.AvailabilityPartiallyAvailable,
		api.AvailabilityUnavailable, api.AvailabilityUnknown,
		api.AvailabilityNotApplicable:
	default:
		return fmt.Errorf("Lua health check returned invalid availability value %q", assessment.Availability)
	}

	switch assessment.Lifecycle {
	case api.LifecycleActive, api.LifecycleTerminating,
		api.LifecycleNotFound, api.LifecycleUnknown:
	default:
		return fmt.Errorf("Lua health check returned invalid lifecycle value %q", assessment.Lifecycle)
	}
	return nil
}

func requiredString(table *luavm.LTable, field string) (string, error) {
	value := table.RawGetString(field)
	text, ok := value.(luavm.LString)
	if !ok || text == "" {
		return "", fmt.Errorf("Lua health check field %q must be a non-empty string", field)
	}
	return string(text), nil
}

func optionalString(table *luavm.LTable, field string) (string, error) {
	value := table.RawGetString(field)
	if value == luavm.LNil {
		return "", nil
	}
	text, ok := value.(luavm.LString)
	if !ok {
		return "", fmt.Errorf("Lua health check field %q must be a string", field)
	}
	return string(text), nil
}

func decodeConditions(value luavm.LValue) ([]api.Condition, error) {
	if value == luavm.LNil {
		return nil, nil
	}
	table, ok := value.(*luavm.LTable)
	if !ok {
		return nil, fmt.Errorf("Lua health check field %q must be a table", "conditions")
	}

	conditions := make([]api.Condition, 0, table.Len())
	for index := 1; index <= table.Len(); index++ {
		entry, ok := table.RawGetInt(index).(*luavm.LTable)
		if !ok {
			return nil, fmt.Errorf("Lua health check condition %d must be a table", index)
		}
		typeValue, err := requiredString(entry, "type")
		if err != nil {
			return nil, fmt.Errorf("condition %d: %w", index, err)
		}
		statusValue, err := requiredString(entry, "status")
		if err != nil {
			return nil, fmt.Errorf("condition %d: %w", index, err)
		}
		status := corev1.ConditionStatus(statusValue)
		if status != corev1.ConditionTrue && status != corev1.ConditionFalse && status != corev1.ConditionUnknown {
			return nil, fmt.Errorf("condition %d has invalid status %q", index, status)
		}
		reason, err := optionalString(entry, "reason")
		if err != nil {
			return nil, fmt.Errorf("condition %d: %w", index, err)
		}
		message, err := optionalString(entry, "message")
		if err != nil {
			return nil, fmt.Errorf("condition %d: %w", index, err)
		}
		conditions = append(conditions, api.Condition{
			Type: typeValue, Status: status,
			Reason: reason, Message: message,
		})
	}
	return conditions, nil
}
