// Package api defines the stable KubeHealth assessment model.
package api

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Condition is the common Kubernetes condition shape accepted as check input.
// Conditions are normalized into dimensions and are not exposed by Assessment.
type Condition struct {
	Type    string                 `json:"type"`
	Status  corev1.ConditionStatus `json:"status"`
	Reason  string                 `json:"reason,omitempty"`
	Message string                 `json:"message,omitempty"`
}

// ReconciliationStatus describes convergence to the latest desired state.
type ReconciliationStatus string

const (
	ReconciliationInProgress    ReconciliationStatus = "InProgress"
	ReconciliationFailed        ReconciliationStatus = "Failed"
	ReconciliationSuspended     ReconciliationStatus = "Suspended"
	ReconciliationReconciled    ReconciliationStatus = "Reconciled"
	ReconciliationUnknown       ReconciliationStatus = "Unknown"
	ReconciliationNotApplicable ReconciliationStatus = "NotApplicable"
)

// AvailabilityStatus describes whether the resource is operational right now.
type AvailabilityStatus string

const (
	AvailabilityAvailable          AvailabilityStatus = "Available"
	AvailabilityPartiallyAvailable AvailabilityStatus = "PartiallyAvailable"
	AvailabilityUnavailable        AvailabilityStatus = "Unavailable"
	AvailabilityNotApplicable      AvailabilityStatus = "NotApplicable"
	AvailabilityUnknown            AvailabilityStatus = "Unknown"
)

// LifecycleStatus describes whether a resource is active, terminating, or absent.
type LifecycleStatus string

const (
	LifecycleActive      LifecycleStatus = "Active"
	LifecycleTerminating LifecycleStatus = "Terminating"
	LifecycleNotFound    LifecycleStatus = "NotFound"
	LifecycleUnknown     LifecycleStatus = "Unknown"
)

// Dimension contains a health value and the metadata explaining that value.
//
// The same type is used for in-memory assessments and operator-published
// status. This keeps the health model identical at both boundaries.
type Dimension[T ~string] struct {
	Status             T            `json:"status"`
	Reason             string       `json:"reason,omitempty"`
	Message            string       `json:"message,omitempty"`
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty"`
}

// DeepCopyInto copies this dimension into out.
func (d *Dimension[T]) DeepCopyInto(out *Dimension[T]) {
	*out = *d
	if d.LastTransitionTime != nil {
		out.LastTransitionTime = d.LastTransitionTime.DeepCopy()
	}
}

// DeepCopy returns an independent copy of this dimension.
func (d *Dimension[T]) DeepCopy() *Dimension[T] {
	if d == nil {
		return nil
	}
	out := new(Dimension[T])
	d.DeepCopyInto(out)
	return out
}

// Assessment contains three independent health dimensions.
type Assessment struct {
	Reconciliation Dimension[ReconciliationStatus] `json:"reconciliation"`
	Availability   Dimension[AvailabilityStatus]   `json:"availability"`
	Lifecycle      Dimension[LifecycleStatus]      `json:"lifecycle"`
}

// Check computes health from resource-specific fields.
type Check func(*unstructured.Unstructured) (Assessment, error)
