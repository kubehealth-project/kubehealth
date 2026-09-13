// Package externalsecrets provides KubeHealth checks for External Secrets Operator resources.
package externalsecrets

import (
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kubehealth-project/kubehealth"
)

// Register adds supported External Secrets Operator checks to an assessor.
func Register(assessor *kubehealth.Assessor) error {
	checks := map[schema.GroupVersionKind]kubehealth.Check{
		{Group: "external-secrets.io", Version: "v1", Kind: "ExternalSecret"}:        externalSecret,
		{Group: "external-secrets.io", Version: "v1", Kind: "ClusterExternalSecret"}: clusterExternalSecret,
		{Group: "external-secrets.io", Version: "v1", Kind: "SecretStore"}:           secretStore,
		{Group: "external-secrets.io", Version: "v1", Kind: "ClusterSecretStore"}:    secretStore,
		{Group: "external-secrets.io", Version: "v1alpha1", Kind: "PushSecret"}:      pushSecret,
	}
	for gvk, check := range checks {
		if err := assessor.Register(gvk, check); err != nil {
			return err
		}
	}
	return nil
}
