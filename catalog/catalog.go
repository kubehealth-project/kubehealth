// Package catalog registers KubeHealth checks maintained in this repository.
package catalog

import (
	"github.com/kubehealth-project/kubehealth"
	"github.com/kubehealth-project/kubehealth/integrations/argorollouts"
	"github.com/kubehealth-project/kubehealth/integrations/certmanager"
	"github.com/kubehealth-project/kubehealth/integrations/keda"
	"github.com/kubehealth-project/kubehealth/integrations/kyverno"
)

// NewAssessor returns an assessor with native and catalog integrations.
func NewAssessor() (*kubehealth.Assessor, error) {
	assessor := kubehealth.NewAssessor()
	for _, register := range []func(*kubehealth.Assessor) error{
		argorollouts.Register,
		certmanager.Register,
		kyverno.Register,
		keda.Register,
	} {
		if err := register(assessor); err != nil {
			return nil, err
		}
	}
	return assessor, nil
}
