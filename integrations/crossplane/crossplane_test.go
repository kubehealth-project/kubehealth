package crossplane_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kubehealth-project/kubehealth"
	"github.com/kubehealth-project/kubehealth/integrations/crossplane"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestManagedResourceProfile(t *testing.T) {
	gvk := schema.GroupVersionKind{Group: "storage.example.org", Version: "v1", Kind: "Bucket"}
	assessor := kubehealth.NewAssessor()
	if err := crossplane.RegisterManagedResources(assessor, gvk); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		fixture            string
		wantReconciliation kubehealth.ReconciliationStatus
		wantAvailability   kubehealth.AvailabilityStatus
	}{
		{"managed-healthy.yaml", kubehealth.ReconciliationReconciled, kubehealth.AvailabilityAvailable},
		{"managed-failed-update.yaml", kubehealth.ReconciliationFailed, kubehealth.AvailabilityAvailable},
		{"managed-creating.yaml", kubehealth.ReconciliationReconciled, kubehealth.AvailabilityUnavailable},
		{"managed-paused-available.yaml", kubehealth.ReconciliationSuspended, kubehealth.AvailabilityAvailable},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			got := assessFixture(t, assessor, tt.fixture)
			if got.Reconciliation != tt.wantReconciliation || got.Availability != tt.wantAvailability {
				t.Fatalf("assessment = %#v, want reconciliation %q and availability %q", got, tt.wantReconciliation, tt.wantAvailability)
			}
			if got.Lifecycle != kubehealth.LifecycleActive {
				t.Errorf("lifecycle = %q, want %q", got.Lifecycle, kubehealth.LifecycleActive)
			}
		})
	}
}

func TestManagedResourceAmbiguousStates(t *testing.T) {
	gvk := schema.GroupVersionKind{Group: "database.example.org", Version: "v1", Kind: "Database"}
	assessor := kubehealth.NewAssessor()
	if err := crossplane.RegisterCompositeResources(assessor, gvk); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name               string
		conditions         []any
		wantReconciliation kubehealth.ReconciliationStatus
		wantAvailability   kubehealth.AvailabilityStatus
	}{
		{"no status", nil, kubehealth.ReconciliationInProgress, kubehealth.AvailabilityUnknown},
		{"synced without ready", []any{condition("Synced", "True", "ReconcileSuccess", "")}, kubehealth.ReconciliationReconciled, kubehealth.AvailabilityUnknown},
		{"ready without synced", []any{condition("Ready", "True", "Available", "")}, kubehealth.ReconciliationInProgress, kubehealth.AvailabilityAvailable},
		{"unknown", []any{condition("Ready", "Unknown", "Unknown", ""), condition("Synced", "Unknown", "Unknown", "")}, kubehealth.ReconciliationUnknown, kubehealth.AvailabilityUnknown},
		{"initial failure", []any{condition("Ready", "False", "Creating", ""), condition("Synced", "False", "ReconcileError", "create failed")}, kubehealth.ReconciliationFailed, kubehealth.AvailabilityUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := resource(gvk, tt.conditions)
			got := assess(t, assessor, obj)
			if got.Reconciliation != tt.wantReconciliation || got.Availability != tt.wantAvailability {
				t.Fatalf("assessment = %#v", got)
			}
		})
	}
}

func TestPackageProfiles(t *testing.T) {
	assessor := registeredAssessor(t)

	tests := []struct {
		fixture            string
		wantReconciliation kubehealth.ReconciliationStatus
		wantAvailability   kubehealth.AvailabilityStatus
	}{
		{"provider-healthy.yaml", kubehealth.ReconciliationReconciled, kubehealth.AvailabilityAvailable},
		{"provider-unhealthy.yaml", kubehealth.ReconciliationReconciled, kubehealth.AvailabilityUnavailable},
		{"provider-revision-awaiting-activation.yaml", kubehealth.ReconciliationReconciled, kubehealth.AvailabilityAvailable},
		{"provider-revision-inactive.yaml", kubehealth.ReconciliationSuspended, kubehealth.AvailabilityNotApplicable},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			got := assessFixture(t, assessor, tt.fixture)
			if got.Reconciliation != tt.wantReconciliation || got.Availability != tt.wantAvailability {
				t.Fatalf("assessment = %#v", got)
			}
		})
	}
}

func TestAllCurrentPackageKindsAreRegistered(t *testing.T) {
	assessor := registeredAssessor(t)
	for _, kind := range []string{"Provider", "Configuration", "Function"} {
		t.Run(kind, func(t *testing.T) {
			obj := resource(schema.GroupVersionKind{Group: "pkg.crossplane.io", Version: "v1", Kind: kind}, []any{
				condition("Installed", "True", "ActivePackageRevision", ""),
				condition("Healthy", "True", "HealthyPackageRevision", ""),
			})
			got := assess(t, assessor, obj)
			if got.Reconciliation != kubehealth.ReconciliationReconciled || got.Availability != kubehealth.AvailabilityAvailable {
				t.Fatalf("assessment = %#v", got)
			}
		})
	}
}

func TestAllCurrentPackageRevisionKindsAreRegistered(t *testing.T) {
	assessor := registeredAssessor(t)
	for _, kind := range []string{"ProviderRevision", "ConfigurationRevision", "FunctionRevision"} {
		t.Run(kind, func(t *testing.T) {
			obj := resource(schema.GroupVersionKind{Group: "pkg.crossplane.io", Version: "v1", Kind: kind}, []any{
				condition("RevisionHealthy", "True", "HealthyPackageRevision", ""),
			})
			got := assess(t, assessor, obj)
			if got.Reconciliation != kubehealth.ReconciliationReconciled || got.Availability != kubehealth.AvailabilityAvailable {
				t.Fatalf("assessment = %#v", got)
			}
		})
	}
}

func TestCoreProfiles(t *testing.T) {
	assessor := registeredAssessor(t)

	tests := []struct {
		fixture            string
		wantReconciliation kubehealth.ReconciliationStatus
		wantAvailability   kubehealth.AvailabilityStatus
	}{
		{"composition.yaml", kubehealth.ReconciliationNotApplicable, kubehealth.AvailabilityNotApplicable},
		{"composition-revision-invalid.yaml", kubehealth.ReconciliationFailed, kubehealth.AvailabilityNotApplicable},
		{"xrd-established.yaml", kubehealth.ReconciliationReconciled, kubehealth.AvailabilityAvailable},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			got := assessFixture(t, assessor, tt.fixture)
			if got.Reconciliation != tt.wantReconciliation || got.Availability != tt.wantAvailability {
				t.Fatalf("assessment = %#v", got)
			}
		})
	}

	deploymentRuntimeConfig := resource(schema.GroupVersionKind{Group: "pkg.crossplane.io", Version: "v1beta1", Kind: "DeploymentRuntimeConfig"}, nil)
	if got := assess(t, assessor, deploymentRuntimeConfig); got.Reconciliation != kubehealth.ReconciliationNotApplicable || got.Availability != kubehealth.AvailabilityNotApplicable {
		t.Fatalf("DeploymentRuntimeConfig assessment = %#v", got)
	}
}

func TestExactRegistration(t *testing.T) {
	assessor := registeredAssessor(t)

	current := resource(schema.GroupVersionKind{Group: "pkg.crossplane.io", Version: "v1", Kind: "Function"}, []any{
		condition("Installed", "True", "ActivePackageRevision", ""),
		condition("Healthy", "True", "HealthyPackageRevision", ""),
	})
	if got := assess(t, assessor, current); got.Reconciliation != kubehealth.ReconciliationReconciled {
		t.Fatalf("current Function assessment = %#v", got)
	}

	historical := current.DeepCopy()
	historical.SetAPIVersion("pkg.crossplane.io/v1beta1")
	if got := assess(t, assessor, historical); got.Reconciliation != kubehealth.ReconciliationUnknown {
		t.Fatalf("unregistered historical Function assessment = %#v", got)
	}

	xrdV1 := resource(schema.GroupVersionKind{Group: "apiextensions.crossplane.io", Version: "v1", Kind: "CompositeResourceDefinition"}, nil)
	if got := assess(t, assessor, xrdV1); got.Reconciliation != kubehealth.ReconciliationUnknown {
		t.Fatalf("unregistered v1 XRD assessment = %#v", got)
	}
}

func TestExplicitRegistrationValidationIsAtomic(t *testing.T) {
	assessor := kubehealth.NewAssessor()
	valid := schema.GroupVersionKind{Group: "storage.example.org", Version: "v1", Kind: "Bucket"}
	invalid := schema.GroupVersionKind{Group: "storage.example.org", Kind: "Bucket"}
	if err := crossplane.RegisterManagedResources(assessor, valid, invalid); err == nil {
		t.Fatal("RegisterManagedResources() error = nil")
	}
	if got := assess(t, assessor, resource(valid, nil)); got.Reconciliation != kubehealth.ReconciliationUnknown {
		t.Fatalf("valid GVK was partially registered: %#v", got)
	}
	if err := crossplane.RegisterManagedResources(nil, valid); err == nil {
		t.Fatal("RegisterManagedResources(nil) error = nil")
	}
	if err := crossplane.Register(nil); err == nil {
		t.Fatal("Register(nil) error = nil")
	}
}

func TestManagedResourceStandardPrecedenceAndConditions(t *testing.T) {
	gvk := schema.GroupVersionKind{Group: "storage.example.org", Version: "v1", Kind: "Bucket"}
	assessor := kubehealth.NewAssessor()
	if err := crossplane.RegisterManagedResources(assessor, gvk); err != nil {
		t.Fatal(err)
	}
	obj := resource(gvk, []any{
		condition("Ready", "True", "Available", ""),
		condition("Synced", "True", "ReconcileSuccess", ""),
	})
	obj.SetGeneration(2)
	if err := unstructured.SetNestedField(obj.Object, int64(1), "status", "observedGeneration"); err != nil {
		t.Fatal(err)
	}
	got := assess(t, assessor, obj)
	if got.Reconciliation != kubehealth.ReconciliationInProgress || got.Availability != kubehealth.AvailabilityAvailable {
		t.Fatalf("assessment = %#v", got)
	}
	if len(got.Conditions) != 3 {
		t.Fatalf("conditions = %#v, want synthetic standard condition plus two resource conditions", got.Conditions)
	}
}

func TestManagedResourceTerminationPreservesAvailability(t *testing.T) {
	gvk := schema.GroupVersionKind{Group: "storage.example.org", Version: "v1", Kind: "Bucket"}
	assessor := kubehealth.NewAssessor()
	if err := crossplane.RegisterManagedResources(assessor, gvk); err != nil {
		t.Fatal(err)
	}
	obj := resource(gvk, []any{
		condition("Ready", "True", "Available", ""),
		condition("Synced", "True", "ReconcileSuccess", ""),
	})
	if err := unstructured.SetNestedField(obj.Object, "2026-08-24T12:00:00Z", "metadata", "deletionTimestamp"); err != nil {
		t.Fatal(err)
	}
	got := assess(t, assessor, obj)
	if got.Lifecycle != kubehealth.LifecycleTerminating || got.Availability != kubehealth.AvailabilityAvailable {
		t.Fatalf("assessment = %#v", got)
	}
}

func TestMalformedConditionsReturnError(t *testing.T) {
	gvk := schema.GroupVersionKind{Group: "storage.example.org", Version: "v1", Kind: "Bucket"}
	assessor := kubehealth.NewAssessor()
	if err := crossplane.RegisterManagedResources(assessor, gvk); err != nil {
		t.Fatal(err)
	}
	obj := resource(gvk, nil)
	obj.Object["status"] = map[string]any{"conditions": "invalid"}
	got, err := assessor.Assess(obj)
	if err == nil {
		t.Fatal("Assess() error = nil")
	}
	if got.Reconciliation != kubehealth.ReconciliationUnknown || got.Availability != kubehealth.AvailabilityUnknown || got.Lifecycle != kubehealth.LifecycleUnknown {
		t.Fatalf("assessment = %#v", got)
	}
}

func registeredAssessor(t *testing.T) *kubehealth.Assessor {
	t.Helper()
	assessor := kubehealth.NewAssessor()
	if err := crossplane.Register(assessor); err != nil {
		t.Fatal(err)
	}
	return assessor
}

func assessFixture(t *testing.T, assessor *kubehealth.Assessor, name string) kubehealth.Assessment {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	obj := &unstructured.Unstructured{}
	if err := obj.UnmarshalJSON(b); err != nil {
		t.Fatal(err)
	}
	return assess(t, assessor, obj)
}

func assess(t *testing.T, assessor *kubehealth.Assessor, obj *unstructured.Unstructured) kubehealth.Assessment {
	t.Helper()
	got, err := assessor.Assess(obj)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func resource(gvk schema.GroupVersionKind, conditions []any) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": gvk.GroupVersion().String(),
		"kind":       gvk.Kind,
		"metadata":   map[string]any{"name": "example"},
	}}
	if conditions != nil {
		obj.Object["status"] = map[string]any{"conditions": conditions}
	}
	return obj
}

func condition(conditionType, status, reason, message string) map[string]any {
	return map[string]any{"type": conditionType, "status": status, "reason": reason, "message": message}
}
