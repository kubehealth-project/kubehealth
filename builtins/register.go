// Package builtins provides KubeHealth checks for Kubernetes-native resources.
package builtins

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kubehealth-project/kubehealth/api"
)

// Checks returns all Kubernetes-native checks keyed by exact GVK.
func Checks() map[schema.GroupVersionKind]api.Check {
	return map[schema.GroupVersionKind]api.Check{
		{Group: "apps", Version: "v1", Kind: "Deployment"}:                     assessDeployment,
		{Group: "apps", Version: "v1", Kind: "StatefulSet"}:                    statefulSetCheck,
		{Group: "apps", Version: "v1", Kind: "DaemonSet"}:                      assessDaemonSet,
		{Version: "v1", Kind: "Service"}:                                       assessService,
		{Version: "v1", Kind: "Pod"}:                                           assessPod,
		{Version: "v1", Kind: "PersistentVolumeClaim"}:                         assessPersistentVolumeClaim,
		{Version: "v1", Kind: "ConfigMap"}:                                     assessAlwaysCurrent("ConfigMap"),
		{Version: "v1", Kind: "Secret"}:                                        assessAlwaysCurrent("Secret"),
		{Group: "autoscaling", Version: "v1", Kind: "HorizontalPodAutoscaler"}: assessHPAV1,
		{Group: "autoscaling", Version: "v2", Kind: "HorizontalPodAutoscaler"}: assessHPAV2,
	}
}

func assessAlwaysCurrent(kind string) api.Check {
	return func(_ *unstructured.Unstructured) (api.Assessment, error) {
		return api.Assessment{
			Reconciliation: api.Dimension[api.ReconciliationStatus]{Status: api.ReconciliationReconciled, Message: kind + " is reconciled"},
			Availability:   api.Dimension[api.AvailabilityStatus]{Status: api.AvailabilityAvailable, Message: kind + " exists"},
			Lifecycle:      api.Dimension[api.LifecycleStatus]{Status: api.LifecycleActive},
		}, nil
	}
}
