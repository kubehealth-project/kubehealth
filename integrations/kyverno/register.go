// Package kyverno provides KubeHealth checks for Kyverno resources.
package kyverno

import (
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kubehealth-project/kubehealth"
)

// Register adds supported Kyverno checks to an assessor.
func Register(assessor *kubehealth.Assessor) error {
	for _, kind := range []string{"Policy", "ClusterPolicy"} {
		if err := assessor.Register(schema.GroupVersionKind{Group: "kyverno.io", Version: "v1", Kind: kind}, policy); err != nil {
			return err
		}
	}
	return nil
}
