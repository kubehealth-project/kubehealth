package kubehealth_test

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	kubehealth "github.com/kubehealth-project/kubehealth"
)

func TestStandardStatusTakesPrecedence(t *testing.T) {
	deployment := healthyDeployment()
	deployment.Status.Conditions = append(deployment.Status.Conditions, appsv1.DeploymentCondition{
		Type: "Stalled", Status: corev1.ConditionTrue,
		Reason: "BrokenDependency", Message: "dependency is permanently broken",
	})

	got := assess(t, deployment)
	if got.Reconciliation.Status != kubehealth.ReconciliationFailed {
		t.Fatalf("status = %q, want %q", got.Reconciliation, kubehealth.ReconciliationFailed)
	}
	if got.Reconciliation.Message != "dependency is permanently broken" {
		t.Fatalf("message = %q", got.Reconciliation.Message)
	}
	if got.Reconciliation.Reason != "BrokenDependency" {
		t.Fatalf("reason = %q", got.Reconciliation.Reason)
	}
	if got.Availability.Status != kubehealth.AvailabilityAvailable {
		t.Fatalf("availability = %q, want %q", got.Availability, kubehealth.AvailabilityAvailable)
	}
}

func TestFailedDeploymentCanRemainAvailable(t *testing.T) {
	deployment := healthyDeployment()
	deployment.Spec.Replicas = int32Pointer(3)
	deployment.Status.Replicas = 4
	deployment.Status.UpdatedReplicas = 1
	deployment.Status.ReadyReplicas = 3
	deployment.Status.AvailableReplicas = 3
	deployment.Status.Conditions = []appsv1.DeploymentCondition{{
		Type: appsv1.DeploymentProgressing, Status: corev1.ConditionFalse,
		Reason: "ProgressDeadlineExceeded", Message: "new ReplicaSet timed out",
	}}

	got := assess(t, deployment)
	if got.Reconciliation.Status != kubehealth.ReconciliationFailed {
		t.Fatalf("status = %q, want %q", got.Reconciliation, kubehealth.ReconciliationFailed)
	}
	if got.Availability.Status != kubehealth.AvailabilityAvailable {
		t.Fatalf("availability = %q, want %q", got.Availability, kubehealth.AvailabilityAvailable)
	}
}

func TestFailedDeploymentCanBeUnavailable(t *testing.T) {
	deployment := healthyDeployment()
	deployment.Status.AvailableReplicas = 0
	deployment.Status.ReadyReplicas = 0
	deployment.Status.Conditions = []appsv1.DeploymentCondition{{
		Type: appsv1.DeploymentProgressing, Status: corev1.ConditionFalse,
		Reason: "ProgressDeadlineExceeded",
	}}

	got := assess(t, deployment)
	if got.Reconciliation.Status != kubehealth.ReconciliationFailed {
		t.Fatalf("status = %q, want %q", got.Reconciliation, kubehealth.ReconciliationFailed)
	}
	if got.Availability.Status != kubehealth.AvailabilityUnavailable {
		t.Fatalf("availability = %q, want %q", got.Availability, kubehealth.AvailabilityUnavailable)
	}
}

func TestCustomCheckRunsWhenStandardStatusIsAbsent(t *testing.T) {
	deployment := healthyDeployment()
	deployment.Status.UpdatedReplicas = 0
	deployment.Status.AvailableReplicas = 0
	deployment.Status.Conditions = nil

	got := assess(t, deployment)
	if got.Reconciliation.Status != kubehealth.ReconciliationInProgress {
		t.Fatalf("status = %q, want %q", got.Reconciliation, kubehealth.ReconciliationInProgress)
	}
	if got.Reconciliation.Message != "Updated: 0/1" {
		t.Fatalf("message = %q", got.Reconciliation.Message)
	}
	if got.Availability.Status != kubehealth.AvailabilityUnavailable {
		t.Fatalf("availability = %q, want %q", got.Availability, kubehealth.AvailabilityUnavailable)
	}
}

func TestResourceCheckInterpretsReadyCondition(t *testing.T) {
	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Service"},
		Spec:     corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer},
	}
	obj := unstructuredFrom(t, service)
	obj.Object["status"] = map[string]any{
		"conditions": []any{map[string]any{
			"type": "Ready", "status": "True", "reason": "Provisioned", "message": "provider says ready",
		}},
	}

	got, err := kubehealth.Assess(obj)
	if err != nil {
		t.Fatal(err)
	}
	if got.Reconciliation.Status != kubehealth.ReconciliationInProgress || got.Reconciliation.Message != "Waiting for load balancer ingress" {
		t.Fatalf("result = %#v", got)
	}
	if got.Availability.Status != kubehealth.AvailabilityUnavailable {
		t.Fatalf("availability = %q, want %q", got.Availability, kubehealth.AvailabilityUnavailable)
	}
}

func TestObservedGenerationTakesPrecedence(t *testing.T) {
	deployment := healthyDeployment()
	deployment.Generation = 2
	deployment.Status.ObservedGeneration = 1

	got := assess(t, deployment)
	if got.Reconciliation.Status != kubehealth.ReconciliationInProgress {
		t.Fatalf("status = %q, want %q", got.Reconciliation, kubehealth.ReconciliationInProgress)
	}
	if got.Availability.Status != kubehealth.AvailabilityAvailable {
		t.Fatalf("availability = %q, want %q", got.Availability, kubehealth.AvailabilityAvailable)
	}
	if got.Reconciliation.Reason != "LatestGenerationNotObserved" {
		t.Fatalf("reason = %q", got.Reconciliation.Reason)
	}
}

func TestDeletionUsesLifecycleDimension(t *testing.T) {
	deployment := healthyDeployment()
	now := metav1.Now()
	deployment.DeletionTimestamp = &now

	got := assess(t, deployment)
	if got.Lifecycle.Status != kubehealth.LifecycleTerminating {
		t.Fatalf("lifecycle = %q, want %q", got.Lifecycle, kubehealth.LifecycleTerminating)
	}
	if got.Reconciliation.Status != kubehealth.ReconciliationUnknown {
		t.Fatalf("reconciliation = %q, want %q", got.Reconciliation, kubehealth.ReconciliationUnknown)
	}
	if got.Availability.Status != kubehealth.AvailabilityAvailable {
		t.Fatalf("availability = %q, want %q", got.Availability, kubehealth.AvailabilityAvailable)
	}
	if got.Lifecycle.Message != "Resource scheduled for deletion" {
		t.Fatalf("lifecycle message = %q", got.Lifecycle.Message)
	}
}

func TestRegisterCustomResourceCheck(t *testing.T) {
	assessor := kubehealth.NewAssessor()
	gvk := schema.GroupVersionKind{Group: "policies.kyverno.io", Version: "v1", Kind: "CleanupPolicy"}
	if err := assessor.Register(gvk, func(obj *unstructured.Unstructured) (kubehealth.Assessment, error) {
		state, _, _ := unstructured.NestedString(obj.Object, "status", "state")
		if state == "Active" {
			return kubehealth.Assessment{Reconciliation: kubehealth.Dimension[kubehealth.ReconciliationStatus]{Status: kubehealth.ReconciliationReconciled, Message: "CleanupPolicy is active"}, Availability: kubehealth.Dimension[kubehealth.AvailabilityStatus]{Status: kubehealth.AvailabilityNotApplicable}, Lifecycle: kubehealth.Dimension[kubehealth.LifecycleStatus]{Status: kubehealth.LifecycleActive}}, nil
		}
		return kubehealth.Assessment{Reconciliation: kubehealth.Dimension[kubehealth.ReconciliationStatus]{Status: kubehealth.ReconciliationInProgress, Message: "CleanupPolicy is becoming active"}, Availability: kubehealth.Dimension[kubehealth.AvailabilityStatus]{Status: kubehealth.AvailabilityNotApplicable}, Lifecycle: kubehealth.Dimension[kubehealth.LifecycleStatus]{Status: kubehealth.LifecycleActive}}, nil
	}); err != nil {
		t.Fatal(err)
	}

	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "policies.kyverno.io/v1",
		"kind":       "CleanupPolicy",
		"metadata":   map[string]any{"name": "example"},
		"status":     map[string]any{"state": "Active"},
	}}
	got, err := assessor.Assess(obj)
	if err != nil {
		t.Fatal(err)
	}
	if got.Reconciliation.Status != kubehealth.ReconciliationReconciled {
		t.Fatalf("status = %q, want %q", got.Reconciliation, kubehealth.ReconciliationReconciled)
	}
}

func TestUnknownResourceDoesNotDefaultToCurrent(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "example.io/v1",
		"kind":       "Widget",
	}}
	got, err := kubehealth.Assess(obj)
	if err != nil {
		t.Fatal(err)
	}
	if got.Reconciliation.Status != kubehealth.ReconciliationUnknown {
		t.Fatalf("status = %q, want %q", got.Reconciliation, kubehealth.ReconciliationUnknown)
	}
	if got.Availability.Status != kubehealth.AvailabilityUnknown {
		t.Fatalf("availability = %q, want %q", got.Availability, kubehealth.AvailabilityUnknown)
	}
}

func TestStaticResourcesAreCurrent(t *testing.T) {
	configMap := &corev1.ConfigMap{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"}}
	if got := assess(t, configMap); got.Reconciliation.Status != kubehealth.ReconciliationReconciled {
		t.Fatalf("status = %q, want %q", got.Reconciliation, kubehealth.ReconciliationReconciled)
	}
}

func int32Pointer(value int32) *int32 {
	return &value
}

func healthyDeployment() *appsv1.Deployment {
	replicas := int32(1)
	return &appsv1.Deployment{
		TypeMeta:   metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{Name: "example", Generation: 1},
		Spec:       appsv1.DeploymentSpec{Replicas: &replicas},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration: 1,
			Replicas:           1,
			UpdatedReplicas:    1,
			ReadyReplicas:      1,
			AvailableReplicas:  1,
		},
	}
}

func assess(t *testing.T, obj runtime.Object) kubehealth.Assessment {
	t.Helper()
	result, err := kubehealth.Assess(unstructuredFrom(t, obj))
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func unstructuredFrom(t *testing.T, obj runtime.Object) *unstructured.Unstructured {
	t.Helper()
	content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		t.Fatal(err)
	}
	return &unstructured.Unstructured{Object: content}
}
