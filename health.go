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

type Dimension[T ~string] = api.Dimension[T]

// Check computes status from resource-specific fields.
type Check = api.Check

// Assessor evaluates operator-published health, generic status signals, and
// resource-specific checks in that order.
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

// Assess returns a complete, current status.kubeHealth report before running
// generic or resource-specific checks. Otherwise, it honors generic lifecycle
// and reconciliation signals. A resource-specific check supplies availability
// and supplies reconciliation when generic signals do not decide it.
func (a *Assessor) Assess(obj *unstructured.Unstructured) (Assessment, error) {
	if obj == nil {
		return Assessment{}, fmt.Errorf("resource must not be nil")
	}
	if obj.GroupVersionKind().Empty() {
		return Assessment{}, fmt.Errorf("resource GVK must not be empty")
	}

	published, publishedState, err := readPublishedStatus(obj)
	if err != nil {
		return unknownResult(err), err
	}
	if publishedState == publishedStatusCurrent {
		return published, nil
	}

	standard, decided, err := assessStandardStatus(obj)
	if err != nil {
		return unknownResult(err), err
	}
	if publishedState == publishedStatusStale {
		if obj.GetDeletionTimestamp() == nil {
			standard = published
			decided = true
		} else {
			standard.Availability = published.Availability
		}
	}

	a.mu.RLock()
	check := a.checks[obj.GroupVersionKind()]
	a.mu.RUnlock()
	if check == nil {
		if decided {
			if standard.Availability.Status == "" {
				standard.Availability.Status = AvailabilityUnknown
			}
			return standard, nil
		}
		return Assessment{
			Reconciliation: Dimension[ReconciliationStatus]{Status: ReconciliationUnknown, Message: fmt.Sprintf("No health check registered for %s", obj.GroupVersionKind())},
			Availability:   Dimension[AvailabilityStatus]{Status: AvailabilityUnknown},
			Lifecycle:      Dimension[LifecycleStatus]{Status: LifecycleActive},
		}, nil
	}

	result, err := check(obj)
	if err != nil {
		return unknownResult(err), err
	}
	if decided {
		result.Reconciliation = standard.Reconciliation
		result.Lifecycle = standard.Lifecycle
	}
	if result.Reconciliation.Status == "" {
		err = fmt.Errorf("health check for %s returned an empty status", obj.GroupVersionKind())
		return unknownResult(err), err
	}
	if result.Availability.Status == "" {
		result.Availability.Status = AvailabilityUnknown
	}
	if result.Lifecycle.Status == "" {
		result.Lifecycle.Status = LifecycleActive
	}
	return result, nil
}

func unknownResult(err error) Assessment {
	return Assessment{
		Reconciliation: Dimension[ReconciliationStatus]{Status: ReconciliationUnknown, Message: err.Error()},
		Availability:   Dimension[AvailabilityStatus]{Status: AvailabilityUnknown},
		Lifecycle:      Dimension[LifecycleStatus]{Status: LifecycleUnknown},
	}
}
