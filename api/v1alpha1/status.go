package v1alpha1

import (
	"fmt"
	"regexp"
	"unicode/utf8"

	"github.com/kubehealth-project/kubehealth/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ContractVersion identifies the wire format read from status.kubeHealth.
const ContractVersion = "v1alpha1"

var reasonPattern = regexp.MustCompile(`^[A-Za-z]([A-Za-z0-9_,:]*[A-Za-z0-9_])?$`)

// Status is an operator-reported KubeHealth assessment.
//
// It is intended to be embedded as an optional status.kubeHealth field in a
// Kubernetes API type. An authoritative report contains all three dimensions
// and the generation that the operator observed.
type Status struct {
	// ContractVersion is the version of this embedded KubeHealth contract.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=v1alpha1
	ContractVersion string `json:"contractVersion"`

	// ObservedGeneration is the metadata.generation used to produce this report.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=0
	ObservedGeneration int64 `json:"observedGeneration"`

	// +kubebuilder:validation:Required
	Reconciliation api.Dimension[api.ReconciliationStatus] `json:"reconciliation"`
	// +kubebuilder:validation:Required
	Availability api.Dimension[api.AvailabilityStatus] `json:"availability"`
	// +kubebuilder:validation:Required
	Lifecycle api.Dimension[api.LifecycleStatus] `json:"lifecycle"`
}

// DeepCopyInto copies this status into out.
func (s *Status) DeepCopyInto(out *Status) {
	*out = *s
	out.Reconciliation = *s.Reconciliation.DeepCopy()
	out.Availability = *s.Availability.DeepCopy()
	out.Lifecycle = *s.Lifecycle.DeepCopy()
}

// DeepCopy returns an independent copy of this status.
func (s *Status) DeepCopy() *Status {
	if s == nil {
		return nil
	}
	out := new(Status)
	s.DeepCopyInto(out)
	return out
}

// ReconciliationDimension is the shared reconciliation dimension type.
type ReconciliationDimension = api.Dimension[api.ReconciliationStatus]

// AvailabilityDimension is the shared availability dimension type.
type AvailabilityDimension = api.Dimension[api.AvailabilityStatus]

// LifecycleDimension is the shared lifecycle dimension type.
type LifecycleDimension = api.Dimension[api.LifecycleStatus]

// ToAssessment converts the reported status to the canonical assessment.
func (s Status) ToAssessment() api.Assessment {
	return api.Assessment{
		Reconciliation: s.Reconciliation,
		Availability:   s.Availability,
		Lifecycle:      s.Lifecycle,
	}
}

// NewStatus creates an empty report for the supplied resource generation.
// Set all three dimensions before reporting it.
func NewStatus(observedGeneration int64) *Status {
	return &Status{
		ContractVersion:    ContractVersion,
		ObservedGeneration: observedGeneration,
	}
}

// SetReconciliation updates reconciliation and maintains LastTransitionTime.
func (s *Status) SetReconciliation(status api.ReconciliationStatus, reason, message string) {
	s.SetReconciliationAt(status, reason, message, metav1.Now())
}

// SetReconciliationAt is SetReconciliation with an explicit time.
func (s *Status) SetReconciliationAt(status api.ReconciliationStatus, reason, message string, now metav1.Time) {
	s.Reconciliation.LastTransitionTime = transitionTime(
		s.Reconciliation.Status, status, s.Reconciliation.LastTransitionTime, now,
	)
	s.Reconciliation.Status = status
	s.Reconciliation.Reason = reason
	s.Reconciliation.Message = message
}

// SetAvailability updates availability and maintains LastTransitionTime.
func (s *Status) SetAvailability(status api.AvailabilityStatus, reason, message string) {
	s.SetAvailabilityAt(status, reason, message, metav1.Now())
}

// SetAvailabilityAt is SetAvailability with an explicit time.
func (s *Status) SetAvailabilityAt(status api.AvailabilityStatus, reason, message string, now metav1.Time) {
	s.Availability.LastTransitionTime = transitionTime(
		s.Availability.Status, status, s.Availability.LastTransitionTime, now,
	)
	s.Availability.Status = status
	s.Availability.Reason = reason
	s.Availability.Message = message
}

// SetLifecycle updates lifecycle and maintains LastTransitionTime.
func (s *Status) SetLifecycle(status api.LifecycleStatus, reason, message string) {
	s.SetLifecycleAt(status, reason, message, metav1.Now())
}

// SetLifecycleAt is SetLifecycle with an explicit time.
func (s *Status) SetLifecycleAt(status api.LifecycleStatus, reason, message string, now metav1.Time) {
	s.Lifecycle.LastTransitionTime = transitionTime(
		s.Lifecycle.Status, status, s.Lifecycle.LastTransitionTime, now,
	)
	s.Lifecycle.Status = status
	s.Lifecycle.Reason = reason
	s.Lifecycle.Message = message
}

// Validate verifies that Status is a complete v1alpha1 report.
func (s Status) Validate() error {
	if s.ContractVersion != ContractVersion {
		return fmt.Errorf("unsupported KubeHealth contract version %q", s.ContractVersion)
	}
	if s.ObservedGeneration < 0 {
		return fmt.Errorf("observedGeneration must not be negative")
	}

	switch s.Reconciliation.Status {
	case api.ReconciliationReconciled, api.ReconciliationInProgress,
		api.ReconciliationFailed, api.ReconciliationSuspended,
		api.ReconciliationUnknown, api.ReconciliationNotApplicable:
	default:
		return fmt.Errorf("invalid reconciliation status %q", s.Reconciliation.Status)
	}

	switch s.Availability.Status {
	case api.AvailabilityAvailable, api.AvailabilityPartiallyAvailable,
		api.AvailabilityUnavailable, api.AvailabilityUnknown,
		api.AvailabilityNotApplicable:
	default:
		return fmt.Errorf("invalid availability status %q", s.Availability.Status)
	}

	switch s.Lifecycle.Status {
	case api.LifecycleActive, api.LifecycleTerminating:
	default:
		return fmt.Errorf("invalid lifecycle status %q", s.Lifecycle.Status)
	}
	for name, dimension := range map[string]api.Dimension[string]{
		"reconciliation": api.Dimension[string]{Status: string(s.Reconciliation.Status), Reason: s.Reconciliation.Reason, Message: s.Reconciliation.Message},
		"availability":   api.Dimension[string]{Status: string(s.Availability.Status), Reason: s.Availability.Reason, Message: s.Availability.Message},
		"lifecycle":      api.Dimension[string]{Status: string(s.Lifecycle.Status), Reason: s.Lifecycle.Reason, Message: s.Lifecycle.Message},
	} {
		if utf8.RuneCountInString(dimension.Reason) > 1024 || dimension.Reason != "" && !reasonPattern.MatchString(dimension.Reason) {
			return fmt.Errorf("%s reason is invalid", name)
		}
		if utf8.RuneCountInString(dimension.Message) > 32768 {
			return fmt.Errorf("%s message exceeds 32768 characters", name)
		}
	}

	return nil
}

func transitionTime[T ~string](previous, next T, current *metav1.Time, now metav1.Time) *metav1.Time {
	if previous == next && current != nil {
		return current
	}
	return &now
}
