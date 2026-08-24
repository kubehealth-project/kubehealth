// Package certmanager provides KubeHealth checks for cert-manager resources.
package certmanager

import (
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kubehealth-project/kubehealth"
)

// Register adds supported cert-manager checks to an assessor.
func Register(assessor *kubehealth.Assessor) error {
	checks := map[schema.GroupVersionKind]kubehealth.Check{
		{Group: "cert-manager.io", Version: "v1", Kind: "Certificate"}: certificate,
		{Group: "cert-manager.io", Version: "v1", Kind: "Issuer"}:      issuer,
	}
	for gvk, check := range checks {
		if err := assessor.Register(gvk, check); err != nil {
			return err
		}
	}
	return nil
}
