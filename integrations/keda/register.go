// Package keda provides KubeHealth checks for KEDA resources.
package keda

import (
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kubehealth-project/kubehealth"
)

// Register adds supported KEDA checks to an assessor.
func Register(assessor *kubehealth.Assessor) error {
	return assessor.Register(
		schema.GroupVersionKind{Group: "keda.sh", Version: "v1alpha1", Kind: "ScaledObject"},
		scaledObject,
	)
}
