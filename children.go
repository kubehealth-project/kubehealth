package kubehealth

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ChildrenAssessment contains the dimensions that can be derived generically
// from a set of child resources. A parent's lifecycle is not inferred from its
// children.
type ChildrenAssessment struct {
	Reconciliation ReconciliationStatus `json:"reconciliation"`
	Availability   AvailabilityStatus   `json:"availability"`
}

// AssessChildren assesses each child with the default assessor and applies the
// default reconciliation and availability aggregation policies.
func AssessChildren(children []*unstructured.Unstructured) (ChildrenAssessment, error) {
	return defaultAssessor.AssessChildren(children)
}

// AssessChildren assesses each child and applies the default reconciliation
// and availability aggregation policies.
func (a *Assessor) AssessChildren(children []*unstructured.Unstructured) (ChildrenAssessment, error) {
	assessments, err := a.assessChildren(children)
	if err != nil {
		return ChildrenAssessment{
			Reconciliation: ReconciliationUnknown,
			Availability:   AvailabilityUnknown,
		}, err
	}
	return AggregateChildren(assessments), nil
}

// AssessChildrenReconciliation assesses each child with the default assessor
// and aggregates their reconciliation statuses into a parent status.
func AssessChildrenReconciliation(children []*unstructured.Unstructured) (ReconciliationStatus, error) {
	return defaultAssessor.AssessChildrenReconciliation(children)
}

// AssessChildrenReconciliation assesses each child and aggregates their
// reconciliation statuses into a parent status.
func (a *Assessor) AssessChildrenReconciliation(children []*unstructured.Unstructured) (ReconciliationStatus, error) {
	assessments, err := a.assessChildren(children)
	if err != nil {
		return ReconciliationUnknown, err
	}
	return AggregateReconciliation(assessments), nil
}

func (a *Assessor) assessChildren(children []*unstructured.Unstructured) ([]Assessment, error) {
	assessments := make([]Assessment, 0, len(children))
	for index, child := range children {
		assessment, err := a.Assess(child)
		if err != nil {
			return nil, fmt.Errorf("assess child %d: %w", index, err)
		}
		assessments = append(assessments, assessment)
	}
	return assessments, nil
}

// AggregateChildren applies the default reconciliation and availability
// aggregation policies to existing child assessments.
func AggregateChildren(children []Assessment) ChildrenAssessment {
	return ChildrenAssessment{
		Reconciliation: AggregateReconciliation(children),
		Availability:   AggregateAvailability(children),
	}
}

// AggregateReconciliation derives a parent reconciliation status from child
// assessments. Failure takes precedence over progress, suspension, and unknown.
// The parent is reconciled when every applicable child is
// reconciled. An empty or entirely not-applicable set is not applicable.
func AggregateReconciliation(children []Assessment) ReconciliationStatus {
	hasReconciled := false
	hasFailed := false
	hasInProgress := false
	hasSuspended := false
	hasUnknown := false
	for _, child := range children {
		switch child.Reconciliation.Status {
		case ReconciliationFailed:
			hasFailed = true
		case ReconciliationInProgress:
			hasInProgress = true
		case ReconciliationSuspended:
			hasSuspended = true
		case ReconciliationUnknown, "":
			hasUnknown = true
		case ReconciliationReconciled:
			hasReconciled = true
		case ReconciliationNotApplicable:
			// Not-applicable children do not prevent applicable children from
			// determining the parent's reconciliation status.
		default:
			hasUnknown = true
		}
	}

	if hasFailed {
		return ReconciliationFailed
	}
	if hasInProgress {
		return ReconciliationInProgress
	}
	if hasSuspended {
		return ReconciliationSuspended
	}
	if hasUnknown {
		return ReconciliationUnknown
	}
	if hasReconciled {
		return ReconciliationReconciled
	}
	return ReconciliationNotApplicable
}

// AggregateAvailability derives a conservative parent availability status from
// child assessments. Not-applicable children are ignored. Known partial
// availability takes precedence over unknown evidence, while unknown evidence
// prevents an otherwise all-available or all-unavailable conclusion.
func AggregateAvailability(children []Assessment) AvailabilityStatus {
	hasAvailable := false
	hasUnavailable := false
	hasPartial := false
	hasUnknown := false

	for _, child := range children {
		switch child.Availability.Status {
		case AvailabilityAvailable:
			hasAvailable = true
		case AvailabilityUnavailable:
			hasUnavailable = true
		case AvailabilityPartiallyAvailable:
			hasPartial = true
		case AvailabilityUnknown, "":
			hasUnknown = true
		case AvailabilityNotApplicable:
			// Not-applicable children do not participate in availability.
		default:
			hasUnknown = true
		}
	}

	if hasPartial || hasAvailable && hasUnavailable {
		return AvailabilityPartiallyAvailable
	}
	if hasUnknown {
		return AvailabilityUnknown
	}
	if hasAvailable {
		return AvailabilityAvailable
	}
	if hasUnavailable {
		return AvailabilityUnavailable
	}
	return AvailabilityNotApplicable
}
