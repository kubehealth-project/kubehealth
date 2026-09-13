package lua

import (
	"fmt"

	luavm "github.com/yuin/gopher-lua"

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
	reconciliationReason, err := optionalString(table, "reconciliationReason")
	if err != nil {
		return api.Assessment{}, err
	}
	availabilityReason, err := optionalString(table, "availabilityReason")
	if err != nil {
		return api.Assessment{}, err
	}
	lifecycleReason, err := optionalString(table, "lifecycleReason")
	if err != nil {
		return api.Assessment{}, err
	}

	assessment := api.Assessment{
		Reconciliation: api.Dimension[api.ReconciliationStatus]{Status: api.ReconciliationStatus(reconciliation), Reason: reconciliationReason, Message: reconciliationMessage},
		Availability:   api.Dimension[api.AvailabilityStatus]{Status: api.AvailabilityStatus(availability), Reason: availabilityReason, Message: availabilityMessage},
		Lifecycle:      api.Dimension[api.LifecycleStatus]{Status: api.LifecycleStatus(lifecycle), Reason: lifecycleReason, Message: lifecycleMessage},
	}
	if err := validateAssessment(assessment); err != nil {
		return api.Assessment{}, err
	}

	return assessment, nil
}

func validateAssessment(assessment api.Assessment) error {
	switch assessment.Reconciliation.Status {
	case api.ReconciliationReconciled, api.ReconciliationInProgress,
		api.ReconciliationFailed, api.ReconciliationSuspended, api.ReconciliationUnknown,
		api.ReconciliationNotApplicable:
	default:
		return fmt.Errorf("Lua health check returned invalid reconciliation value %q", assessment.Reconciliation.Status)
	}

	switch assessment.Availability.Status {
	case api.AvailabilityAvailable, api.AvailabilityPartiallyAvailable,
		api.AvailabilityUnavailable, api.AvailabilityUnknown,
		api.AvailabilityNotApplicable:
	default:
		return fmt.Errorf("Lua health check returned invalid availability value %q", assessment.Availability.Status)
	}

	switch assessment.Lifecycle.Status {
	case api.LifecycleActive, api.LifecycleTerminating,
		api.LifecycleNotFound, api.LifecycleUnknown:
	default:
		return fmt.Errorf("Lua health check returned invalid lifecycle value %q", assessment.Lifecycle.Status)
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
