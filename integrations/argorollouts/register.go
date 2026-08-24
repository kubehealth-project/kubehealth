// Package argorollouts provides KubeHealth checks for Argo Rollouts resources.
package argorollouts

import (
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kubehealth-project/kubehealth"
)

// Register adds supported Argo Rollouts checks to an assessor.
func Register(assessor *kubehealth.Assessor) error {
	return assessor.Register(
		schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Rollout"},
		rollout,
	)
}
