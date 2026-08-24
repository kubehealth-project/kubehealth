// Package kubehealth provides extensible Kubernetes resource health checks.
package kubehealth

import (
	"fmt"
	"sync"

	"github.com/kubehealth-project/kubehealth/api"
	"github.com/kubehealth-project/kubehealth/builtins"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ReconciliationStatus = api.ReconciliationStatus

const (
	ReconciliationInProgress    = api.ReconciliationInProgress
	ReconciliationFailed        = api.ReconciliationFailed
	ReconciliationSuspended     = api.ReconciliationSuspended
	ReconciliationReconciled    = api.ReconciliationReconciled
	ReconciliationUnknown       = api.ReconciliationUnknown
	ReconciliationNotApplicable = api.ReconciliationNotApplicable
)

// AvailabilityStatus describes whether the resource is operational right now.
type AvailabilityStatus = api.AvailabilityStatus

const (
	AvailabilityAvailable          = api.AvailabilityAvailable
	AvailabilityPartiallyAvailable = api.AvailabilityPartiallyAvailable
	AvailabilityUnavailable        = api.AvailabilityUnavailable
	AvailabilityNotApplicable      = api.AvailabilityNotApplicable
	AvailabilityUnknown            = api.AvailabilityUnknown
)

// LifecycleStatus describes whether a resource is active, terminating, or absent.
type LifecycleStatus = api.LifecycleStatus

const (
	LifecycleActive      = api.LifecycleActive
	LifecycleTerminating = api.LifecycleTerminating
	LifecycleNotFound    = api.LifecycleNotFound
	LifecycleUnknown     = api.LifecycleUnknown
)

// Assessment contains three independent health dimensions and their explanations.
type Assessment = api.Assessment

// Check computes status from resource-specific fields.
type Check = api.Check

// Assessor evaluates standard status signals before resource-specific checks.
type Assessor struct {
	mu     sync.RWMutex
	checks map[schema.GroupVersionKind]Check
}

var defaultAssessor = NewAssessor()

// NewAssessor returns an assessor containing the library's built-in checks.
func NewAssessor() *Assessor {
	assessor := &Assessor{checks: make(map[schema.GroupVersionKind]Check)}
	for gvk, check := range builtins.Checks() {
		assessor.checks[gvk] = check
	}
	return assessor
}

// Assess evaluates a resource with the default assessor.
func Assess(obj *unstructured.Unstructured) (Assessment, error) {
	return defaultAssessor.Assess(obj)
}

// Register adds or replaces a resource-specific check.
func (a *Assessor) Register(gvk schema.GroupVersionKind, check Check) error {
	if gvk.Empty() {
		return fmt.Errorf("resource GVK must not be empty")
	}
	if check == nil {
		return fmt.Errorf("health check must not be nil")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.checks[gvk] = check
	return nil
}

// Assess honors standard kstatus signals exposed by the resource for the
// reconciliation status. A resource-specific check supplies availability and
// supplies reconciliation when standard signals do not decide it.
func (a *Assessor) Assess(obj *unstructured.Unstructured) (Assessment, error) {
	if obj == nil {
		return Assessment{}, fmt.Errorf("resource must not be nil")
	}
	if obj.GroupVersionKind().Empty() {
		return Assessment{}, fmt.Errorf("resource GVK must not be empty")
	}

	standard, decided, err := assessStandardStatus(obj)
	if err != nil {
		return unknownResult(err), err
	}

	a.mu.RLock()
	check := a.checks[obj.GroupVersionKind()]
	a.mu.RUnlock()
	if check == nil {
		if decided {
			standard.Availability = AvailabilityUnknown
			return standard, nil
		}
		return Assessment{
			Reconciliation:        ReconciliationUnknown,
			Availability:          AvailabilityUnknown,
			Lifecycle:             LifecycleActive,
			ReconciliationMessage: fmt.Sprintf("No health check registered for %s", obj.GroupVersionKind()),
		}, nil
	}

	result, err := check(obj)
	if err != nil {
		return unknownResult(err), err
	}
	if decided {
		result.Reconciliation = standard.Reconciliation
		result.ReconciliationMessage = standard.ReconciliationMessage
		result.Lifecycle = standard.Lifecycle
		result.LifecycleMessage = standard.LifecycleMessage
		result.Conditions = append(standard.Conditions, result.Conditions...)
	}
	if result.Reconciliation == "" {
		err = fmt.Errorf("health check for %s returned an empty status", obj.GroupVersionKind())
		return unknownResult(err), err
	}
	if result.Availability == "" {
		result.Availability = AvailabilityUnknown
	}
	if result.Lifecycle == "" {
		result.Lifecycle = LifecycleActive
	}
	return result, nil
}

func unknownResult(err error) Assessment {
	return Assessment{
		Reconciliation: ReconciliationUnknown, Availability: AvailabilityUnknown,
		Lifecycle: LifecycleUnknown, ReconciliationMessage: err.Error(),
	}
}
