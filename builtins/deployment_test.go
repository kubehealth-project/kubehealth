package builtins_test

import (
	"testing"

	"github.com/kubehealth-project/kubehealth"
	"github.com/kubehealth-project/kubehealth/api"
	"github.com/kubehealth-project/kubehealth/builtins"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestDeploymentScenariosKubeHealthAndKStatus(t *testing.T) {
	assessor := newBuiltinsAssessor(t)
	tests := []struct {
		name string
		make func() *appsv1.Deployment

		wantReconciliation kubehealth.ReconciliationStatus
		wantAvailability   kubehealth.AvailabilityStatus
		wantLifecycle      kubehealth.LifecycleStatus
	}{
		{
			name: "healthy rollout",
			make: func() *appsv1.Deployment {
				return deploymentScenario(3, 3, 3, 3, 3)
			},
			wantReconciliation: kubehealth.ReconciliationReconciled,
			wantAvailability:   kubehealth.AvailabilityAvailable,
			wantLifecycle:      kubehealth.LifecycleActive,
		},
		{
			name: "paused",
			make: func() *appsv1.Deployment {
				deployment := deploymentScenario(3, 3, 3, 3, 3)
				deployment.Spec.Paused = true
				return deployment
			},
			wantReconciliation: kubehealth.ReconciliationSuspended,
			wantAvailability:   kubehealth.AvailabilityAvailable,
			wantLifecycle:      kubehealth.LifecycleActive,
		},
		{
			name: "initial rollout progressing without available replicas",
			make: func() *appsv1.Deployment {
				return deploymentScenario(3, 0, 1, 0, 0)
			},
			wantReconciliation: kubehealth.ReconciliationInProgress,
			wantAvailability:   kubehealth.AvailabilityUnavailable,
			wantLifecycle:      kubehealth.LifecycleActive,
		},
		{
			name: "rolling update progressing while old replicas serve",
			make: func() *appsv1.Deployment {
				return deploymentScenario(3, 1, 4, 3, 3)
			},
			wantReconciliation: kubehealth.ReconciliationInProgress,
			wantAvailability:   kubehealth.AvailabilityAvailable,
			wantLifecycle:      kubehealth.LifecycleActive,
		},
		{
			name: "rolling update progressing with reduced capacity",
			make: func() *appsv1.Deployment {
				return deploymentScenario(3, 2, 3, 2, 2)
			},
			wantReconciliation: kubehealth.ReconciliationInProgress,
			wantAvailability:   kubehealth.AvailabilityPartiallyAvailable,
			wantLifecycle:      kubehealth.LifecycleActive,
		},
		{
			name: "failed rollout while old replicas still serve",
			make: func() *appsv1.Deployment {
				deployment := deploymentScenario(3, 1, 4, 3, 3)
				deployment.Status.Conditions = []appsv1.DeploymentCondition{{
					Type: appsv1.DeploymentProgressing, Status: corev1.ConditionFalse,
					Reason: "ProgressDeadlineExceeded",
				}}
				return deployment
			},
			wantReconciliation: kubehealth.ReconciliationFailed,
			wantAvailability:   kubehealth.AvailabilityAvailable,
			wantLifecycle:      kubehealth.LifecycleActive,
		},
		{
			name: "failed initial rollout with no available replicas",
			make: func() *appsv1.Deployment {
				deployment := deploymentScenario(3, 3, 3, 0, 0)
				deployment.Status.Conditions = []appsv1.DeploymentCondition{{
					Type: appsv1.DeploymentProgressing, Status: corev1.ConditionFalse,
					Reason: "ProgressDeadlineExceeded",
				}}
				return deployment
			},
			wantReconciliation: kubehealth.ReconciliationFailed,
			wantAvailability:   kubehealth.AvailabilityUnavailable,
			wantLifecycle:      kubehealth.LifecycleActive,
		},
		{
			name: "new generation not observed while old replicas serve",
			make: func() *appsv1.Deployment {
				deployment := deploymentScenario(3, 3, 3, 3, 3)
				deployment.Generation = 2
				deployment.Status.ObservedGeneration = 1
				return deployment
			},
			wantReconciliation: kubehealth.ReconciliationInProgress,
			wantAvailability:   kubehealth.AvailabilityAvailable,
			wantLifecycle:      kubehealth.LifecycleActive,
		},
		{
			name: "scaled to zero",
			make: func() *appsv1.Deployment {
				return deploymentScenario(0, 0, 0, 0, 0)
			},
			wantReconciliation: kubehealth.ReconciliationReconciled,
			wantAvailability:   kubehealth.AvailabilityNotApplicable,
			wantLifecycle:      kubehealth.LifecycleActive,
		},
		{
			name: "terminating while replicas still serve",
			make: func() *appsv1.Deployment {
				deployment := deploymentScenario(3, 3, 3, 3, 3)
				now := metav1.Now()
				deployment.DeletionTimestamp = &now
				return deployment
			},
			wantReconciliation: kubehealth.ReconciliationUnknown,
			wantAvailability:   kubehealth.AvailabilityAvailable,
			wantLifecycle:      kubehealth.LifecycleTerminating,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := unstructuredFrom(t, tt.make())

			assessment, err := assessor.Assess(obj)
			if err != nil {
				t.Fatal(err)
			}
			if assessment.Reconciliation.Status != tt.wantReconciliation {
				t.Errorf("reconciliation = %q, want %q", assessment.Reconciliation, tt.wantReconciliation)
			}
			if assessment.Availability.Status != tt.wantAvailability {
				t.Errorf("availability = %q, want %q", assessment.Availability, tt.wantAvailability)
			}
			if assessment.Lifecycle.Status != tt.wantLifecycle {
				t.Errorf("lifecycle = %q, want %q", assessment.Lifecycle, tt.wantLifecycle)
			}
		})
	}
}

func newBuiltinsAssessor(t *testing.T) *kubehealth.Assessor {
	t.Helper()
	assessor := kubehealth.NewAssessor()
	for gvk, check := range builtins.Checks() {
		if err := assessor.Register(gvk, api.Check(check)); err != nil {
			t.Fatal(err)
		}
	}
	return assessor
}

func unstructuredFrom(t *testing.T, obj runtime.Object) *unstructured.Unstructured {
	t.Helper()
	content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		t.Fatal(err)
	}
	return &unstructured.Unstructured{Object: content}
}

func deploymentScenario(desired, updated, replicas, ready, available int32) *appsv1.Deployment {
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "example",
			Generation: 1,
		},
		Spec: appsv1.DeploymentSpec{Replicas: &desired},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration: 1,
			Replicas:           replicas,
			UpdatedReplicas:    updated,
			ReadyReplicas:      ready,
			AvailableReplicas:  available,
		},
	}
}
