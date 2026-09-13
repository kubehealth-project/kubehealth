package gatewayapi_test

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kubehealth-project/kubehealth"
)

func TestHTTPRouteAndGRPCRouteShareSemantics(t *testing.T) {
	for _, kind := range []string{"HTTPRoute", "GRPCRoute"} {
		t.Run(kind, func(t *testing.T) {
			got := assess(t, routeResource(kind, 2, []any{
				parentStatus("gateway-a", "http", []any{condition("Accepted", "True", "Accepted", 2), condition("ResolvedRefs", "True", "ResolvedRefs", 2)}),
				parentStatus("gateway-b", "http", []any{condition("Accepted", "False", "NoMatchingParent", 2), condition("ResolvedRefs", "True", "ResolvedRefs", 2)}),
			}))
			assertAssessment(t, got, kubehealth.ReconciliationFailed, kubehealth.AvailabilityPartiallyAvailable)
		})
	}
}

func TestRouteAttachmentStates(t *testing.T) {
	tests := []struct {
		name           string
		generation     int64
		parents        []any
		reconciliation kubehealth.ReconciliationStatus
		availability   kubehealth.AvailabilityStatus
	}{
		{
			name: "accepted without nonstandard programmed condition", generation: 2,
			parents: []any{
				parentStatus("gateway-a", "http", []any{condition("Accepted", "True", "Accepted", 2), condition("ResolvedRefs", "True", "ResolvedRefs", 2)}),
				parentStatus("gateway-b", "http", []any{condition("Accepted", "True", "Accepted", 2), condition("ResolvedRefs", "True", "ResolvedRefs", 2)}),
			},
			reconciliation: kubehealth.ReconciliationReconciled, availability: kubehealth.AvailabilityAvailable,
		},
		{
			name: "accepted with unresolved refs is partial", generation: 2,
			parents: []any{
				parentStatus("gateway-a", "http", []any{condition("Accepted", "True", "Accepted", 2), condition("ResolvedRefs", "False", "BackendNotFound", 2)}),
				parentStatus("gateway-b", "http", []any{condition("Accepted", "True", "Accepted", 2), condition("ResolvedRefs", "True", "ResolvedRefs", 2)}),
			},
			reconciliation: kubehealth.ReconciliationFailed, availability: kubehealth.AvailabilityPartiallyAvailable,
		},
		{
			name: "partially invalid rules", generation: 2,
			parents: []any{
				parentStatus("gateway-a", "http", []any{condition("Accepted", "True", "Accepted", 2), condition("PartiallyInvalid", "True", "UnsupportedValue", 2)}),
				parentStatus("gateway-b", "http", []any{condition("Accepted", "True", "Accepted", 2)}),
			},
			reconciliation: kubehealth.ReconciliationFailed, availability: kubehealth.AvailabilityPartiallyAvailable,
		},
		{
			name: "missing desired parent status", generation: 2,
			parents:        []any{parentStatus("gateway-a", "http", []any{condition("Accepted", "True", "Accepted", 2)})},
			reconciliation: kubehealth.ReconciliationInProgress, availability: kubehealth.AvailabilityUnknown,
		},
		{
			name: "stale accepted parent remains available", generation: 2,
			parents: []any{
				parentStatus("gateway-a", "http", []any{condition("Accepted", "True", "Accepted", 1)}),
				parentStatus("gateway-b", "http", []any{condition("Accepted", "True", "Accepted", 2)}),
			},
			reconciliation: kubehealth.ReconciliationInProgress, availability: kubehealth.AvailabilityAvailable,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := assess(t, routeResource("HTTPRoute", tt.generation, tt.parents))
			assertAssessment(t, got, tt.reconciliation, tt.availability)
		})
	}
}

func TestRouteWithoutParents(t *testing.T) {
	got := assess(t, resource("HTTPRoute", 1, map[string]any{"rules": []any{}}, nil))
	assertAssessment(t, got, kubehealth.ReconciliationReconciled, kubehealth.AvailabilityNotApplicable)
}

func TestRouteEvaluatesGenerationPerDecisiveCondition(t *testing.T) {
	got := assess(t, routeResource("HTTPRoute", 2, []any{
		parentStatus("gateway-a", "http", []any{
			condition("Accepted", "True", "Accepted", 2),
			condition("ResolvedRefs", "True", "ResolvedRefs", 2),
			condition("example.io/Extension", "True", "OldExtension", 1),
		}),
		parentStatus("gateway-b", "http", []any{
			condition("Accepted", "True", "Accepted", 3),
			condition("ResolvedRefs", "True", "ResolvedRefs", 3),
		}),
	}))
	assertAssessment(t, got, kubehealth.ReconciliationReconciled, kubehealth.AvailabilityAvailable)
}

func routeResource(kind string, generation int64, parents []any) *unstructured.Unstructured {
	return resource(kind, generation, map[string]any{
		"parentRefs": []any{
			map[string]any{"name": "gateway-a", "sectionName": "http"},
			map[string]any{"name": "gateway-b", "sectionName": "http"},
		},
	}, map[string]any{"parents": parents})
}

func parentStatus(name, section string, conditions []any) map[string]any {
	return map[string]any{
		"parentRef":      map[string]any{"name": name, "sectionName": section},
		"controllerName": "example.io/controller", "conditions": conditions,
	}
}
