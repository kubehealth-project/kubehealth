// Package api defines the stable KubeHealth assessment model.
package api

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

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

// Condition is the common Kubernetes condition shape used by KubeHealth.
type Condition struct {
	Type    string                 `json:"type"`
	Status  corev1.ConditionStatus `json:"status"`
	Reason  string                 `json:"reason,omitempty"`
	Message string                 `json:"message,omitempty"`
}

// Assessment contains three independent health dimensions and their explanations.
type Assessment struct {
	Reconciliation        ReconciliationStatus `json:"reconciliation"`
	Availability          AvailabilityStatus   `json:"availability"`
	Lifecycle             LifecycleStatus      `json:"lifecycle"`
	ReconciliationMessage string               `json:"reconciliationMessage,omitempty"`
	AvailabilityMessage   string               `json:"availabilityMessage,omitempty"`
	LifecycleMessage      string               `json:"lifecycleMessage,omitempty"`
	Conditions            []Condition          `json:"conditions,omitempty"`
}

// Check computes health from resource-specific fields.
type Check func(*unstructured.Unstructured) (Assessment, error)
