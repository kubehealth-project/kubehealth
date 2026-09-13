// Package crossplane provides KubeHealth checks for current Crossplane core
// resources and helpers for explicitly selected Crossplane-managed resources.
package crossplane

import (
	"fmt"

	"github.com/kubehealth-project/kubehealth"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var coreChecks = []struct {
	gvk   schema.GroupVersionKind
	check kubehealth.Check
}{
	{schema.GroupVersionKind{Group: "pkg.crossplane.io", Version: "v1", Kind: "Provider"}, packageCheck},
	{schema.GroupVersionKind{Group: "pkg.crossplane.io", Version: "v1", Kind: "Configuration"}, packageCheck},
	{schema.GroupVersionKind{Group: "pkg.crossplane.io", Version: "v1", Kind: "Function"}, packageCheck},
	{schema.GroupVersionKind{Group: "pkg.crossplane.io", Version: "v1", Kind: "ProviderRevision"}, packageRevisionCheck},
	{schema.GroupVersionKind{Group: "pkg.crossplane.io", Version: "v1", Kind: "ConfigurationRevision"}, packageRevisionCheck},
	{schema.GroupVersionKind{Group: "pkg.crossplane.io", Version: "v1", Kind: "FunctionRevision"}, packageRevisionCheck},
	{schema.GroupVersionKind{Group: "pkg.crossplane.io", Version: "v1beta1", Kind: "DeploymentRuntimeConfig"}, declarativeCheck},
	{schema.GroupVersionKind{Group: "apiextensions.crossplane.io", Version: "v1", Kind: "Composition"}, declarativeCheck},
	{schema.GroupVersionKind{Group: "apiextensions.crossplane.io", Version: "v1", Kind: "CompositionRevision"}, compositionRevisionCheck},
	{schema.GroupVersionKind{Group: "apiextensions.crossplane.io", Version: "v2", Kind: "CompositeResourceDefinition"}, xrdCheck},
}

// Register adds supported current Crossplane core checks to an assessor.
func Register(assessor *kubehealth.Assessor) error {
	if assessor == nil {
		return fmt.Errorf("assessor must not be nil")
	}
	for _, registration := range coreChecks {
		if err := assessor.Register(registration.gvk, registration.check); err != nil {
			return err
		}
	}
	return nil
}

// RegisterManagedResources registers shared Ready/Synced health semantics for
// the exact managed-resource GVKs supplied by the caller.
func RegisterManagedResources(assessor *kubehealth.Assessor, gvks ...schema.GroupVersionKind) error {
	return registerExact(assessor, managedResourceCheck, gvks...)
}

// RegisterCompositeResources registers shared Ready/Synced health semantics
// for the exact composite-resource GVKs supplied by the caller.
func RegisterCompositeResources(assessor *kubehealth.Assessor, gvks ...schema.GroupVersionKind) error {
	return registerExact(assessor, managedResourceCheck, gvks...)
}

func registerExact(assessor *kubehealth.Assessor, check kubehealth.Check, gvks ...schema.GroupVersionKind) error {
	if assessor == nil {
		return fmt.Errorf("assessor must not be nil")
	}
	for _, gvk := range gvks {
		if gvk.Group == "" || gvk.Version == "" || gvk.Kind == "" {
			return fmt.Errorf("resource GVK must include group, version, and kind: %q", gvk.String())
		}
	}
	for _, gvk := range gvks {
		if err := assessor.Register(gvk, check); err != nil {
			return err
		}
	}
	return nil
}
