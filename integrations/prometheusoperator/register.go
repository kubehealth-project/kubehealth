// Package prometheusoperator provides KubeHealth checks for Prometheus Operator resources.
package prometheusoperator

import (
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kubehealth-project/kubehealth"
)

// Register adds checks for Prometheus Operator workload resources to an assessor.
func Register(assessor *kubehealth.Assessor) error {
	for _, kind := range []string{"Prometheus", "Alertmanager", "ThanosRuler"} {
		if err := assessor.Register(
			schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: kind},
			workload,
		); err != nil {
			return err
		}
	}
	return nil
}
