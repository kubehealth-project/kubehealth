// Package gatewayapi provides KubeHealth checks for Kubernetes Gateway API resources.
package gatewayapi

import (
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kubehealth-project/kubehealth"
)

const group = "gateway.networking.k8s.io"

// Register adds supported Gateway API checks to an assessor.
func Register(assessor *kubehealth.Assessor) error {
	checks := map[schema.GroupVersionKind]kubehealth.Check{
		{Group: group, Version: "v1", Kind: "GatewayClass"}:     gatewayClass,
		{Group: group, Version: "v1", Kind: "Gateway"}:          gateway,
		{Group: group, Version: "v1", Kind: "HTTPRoute"}:        route,
		{Group: group, Version: "v1", Kind: "GRPCRoute"}:        route,
		{Group: group, Version: "v1", Kind: "BackendTLSPolicy"}: backendTLSPolicy,
	}
	for gvk, check := range checks {
		if err := assessor.Register(gvk, check); err != nil {
			return err
		}
	}
	return nil
}
