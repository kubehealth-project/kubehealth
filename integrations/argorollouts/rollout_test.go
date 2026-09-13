package argorollouts_test

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kubehealth-project/kubehealth"
	"github.com/kubehealth-project/kubehealth/integrations/argorollouts"
)

func TestRollout(t *testing.T) {
	tests := []struct {
		name                    string
		mutate                  func(map[string]any)
		wantReconciliation      kubehealth.ReconciliationStatus
		wantAvailability        kubehealth.AvailabilityStatus
		wantReconcileMessage    string
		wantAvailabilityMessage string
	}{
		{
			name: "healthy canary", wantReconciliation: kubehealth.ReconciliationReconciled,
			wantAvailability: kubehealth.AvailabilityAvailable, wantReconcileMessage: "Rollout is reconciled",
			wantAvailabilityMessage: "5 replicas are available",
		},
		{
			name: "new rollout without status", mutate: func(obj map[string]any) {
				delete(obj, "status")
			}, wantReconciliation: kubehealth.ReconciliationInProgress, wantAvailability: kubehealth.AvailabilityUnavailable,
			wantReconcileMessage:    "Waiting for rollout spec update to be observed",
			wantAvailabilityMessage: "No Rollout replicas are available",
		},
		{
			name: "new generation remains available", mutate: func(obj map[string]any) {
				obj["metadata"].(map[string]any)["generation"] = int64(2)
			}, wantReconciliation: kubehealth.ReconciliationInProgress, wantAvailability: kubehealth.AvailabilityAvailable,
			wantReconcileMessage:    "Rollout generation is 2, but latest observed generation is 1",
			wantAvailabilityMessage: "5 replicas are available",
		},
		{
			name: "new workload generation remains available", mutate: func(obj map[string]any) {
				obj["spec"].(map[string]any)["workloadRef"] = map[string]any{"kind": "Deployment", "name": "example"}
				obj["metadata"].(map[string]any)["annotations"] = map[string]any{workloadGenerationAnnotation: "2"}
				obj["status"].(map[string]any)["workloadObservedGeneration"] = "1"
			}, wantReconciliation: kubehealth.ReconciliationInProgress, wantAvailability: kubehealth.AvailabilityAvailable,
			wantReconcileMessage:    "Waiting for rollout spec update to be observed",
			wantAvailabilityMessage: "5 replicas are available",
		},
		{
			name: "failed rollout remains available", mutate: func(obj map[string]any) {
				obj["status"].(map[string]any)["conditions"] = []any{condition("Progressing", "False", "ProgressDeadlineExceeded", "ReplicaSet timed out")}
			}, wantReconciliation: kubehealth.ReconciliationFailed, wantAvailability: kubehealth.AvailabilityAvailable,
			wantReconcileMessage: "ReplicaSet timed out", wantAvailabilityMessage: "5 replicas are available",
		},
		{
			name: "failed initial rollout is unavailable", mutate: func(obj map[string]any) {
				status := obj["status"].(map[string]any)
				status["availableReplicas"] = int64(0)
				status["conditions"] = []any{condition("InvalidSpec", "True", "InvalidSpec", "strategy is invalid")}
			}, wantReconciliation: kubehealth.ReconciliationFailed, wantAvailability: kubehealth.AvailabilityUnavailable,
			wantReconcileMessage: "strategy is invalid", wantAvailabilityMessage: "No Rollout replicas are available",
		},
		{
			name: "controller pause remains available", mutate: func(obj map[string]any) {
				obj["status"].(map[string]any)["pauseConditions"] = []any{map[string]any{"reason": "CanaryPauseStep"}}
			}, wantReconciliation: kubehealth.ReconciliationSuspended, wantAvailability: kubehealth.AvailabilityAvailable,
			wantReconcileMessage: "Rollout is paused", wantAvailabilityMessage: "5 replicas are available",
		},
		{
			name: "phase healthy", mutate: func(obj map[string]any) {
				status := obj["status"].(map[string]any)
				status["phase"] = "Healthy"
				status["message"] = "Rollout is healthy"
			}, wantReconciliation: kubehealth.ReconciliationReconciled, wantAvailability: kubehealth.AvailabilityAvailable,
			wantReconcileMessage: "Rollout is healthy", wantAvailabilityMessage: "5 replicas are available",
		},
		{
			name: "phase degraded", mutate: func(obj map[string]any) {
				status := obj["status"].(map[string]any)
				status["phase"] = "Degraded"
				status["message"] = "InvalidSpec"
			}, wantReconciliation: kubehealth.ReconciliationFailed, wantAvailability: kubehealth.AvailabilityAvailable,
			wantReconcileMessage: "InvalidSpec", wantAvailabilityMessage: "5 replicas are available",
		},
		{
			name: "phase paused", mutate: func(obj map[string]any) {
				status := obj["status"].(map[string]any)
				status["phase"] = "Paused"
				status["message"] = "CanaryPauseStep"
			}, wantReconciliation: kubehealth.ReconciliationSuspended, wantAvailability: kubehealth.AvailabilityAvailable,
			wantReconcileMessage: "CanaryPauseStep", wantAvailabilityMessage: "5 replicas are available",
		},
		{
			name: "unknown phase is not assumed healthy", mutate: func(obj map[string]any) {
				status := obj["status"].(map[string]any)
				status["phase"] = "Mystery"
				status["message"] = "Controller returned an unfamiliar phase"
			}, wantReconciliation: kubehealth.ReconciliationUnknown, wantAvailability: kubehealth.AvailabilityAvailable,
			wantReconcileMessage: "Controller returned an unfamiliar phase", wantAvailabilityMessage: "5 replicas are available",
		},
		{
			name: "canary old replicas pending", mutate: func(obj map[string]any) {
				obj["status"].(map[string]any)["replicas"] = int64(6)
			}, wantReconciliation: kubehealth.ReconciliationInProgress, wantAvailability: kubehealth.AvailabilityAvailable,
			wantReconcileMessage:    "Waiting for roll out to finish: old replicas are pending termination",
			wantAvailabilityMessage: "5 replicas are available",
		},
		{
			name: "canary steps pending", mutate: func(obj map[string]any) {
				obj["status"].(map[string]any)["stableRS"] = "old-hash"
			}, wantReconciliation: kubehealth.ReconciliationInProgress, wantAvailability: kubehealth.AvailabilityAvailable,
			wantReconcileMessage: "Waiting for rollout to finish steps", wantAvailabilityMessage: "5 replicas are available",
		},
		{
			name: "partially available while updating", mutate: func(obj map[string]any) {
				status := obj["status"].(map[string]any)
				status["updatedReplicas"] = int64(2)
				status["availableReplicas"] = int64(3)
			}, wantReconciliation: kubehealth.ReconciliationInProgress, wantAvailability: kubehealth.AvailabilityPartiallyAvailable,
			wantReconcileMessage:    "Waiting for roll out to finish: More replicas need to be updated",
			wantAvailabilityMessage: "3 of 5 desired replicas are available",
		},
		{
			name: "scaled to zero", mutate: func(obj map[string]any) {
				obj["spec"].(map[string]any)["replicas"] = int64(0)
				status := obj["status"].(map[string]any)
				status["replicas"] = int64(0)
				status["updatedReplicas"] = int64(0)
				status["availableReplicas"] = int64(0)
			}, wantReconciliation: kubehealth.ReconciliationReconciled, wantAvailability: kubehealth.AvailabilityNotApplicable,
			wantReconcileMessage: "Rollout is reconciled", wantAvailabilityMessage: "Rollout is scaled to zero",
		},
	}

	assessor := kubehealth.NewAssessor()
	if err := argorollouts.Register(assessor); err != nil {
		t.Fatal(err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			object := healthyCanary()
			if tt.mutate != nil {
				tt.mutate(object)
			}
			got, err := assessor.Assess(&unstructured.Unstructured{Object: object})
			if err != nil {
				t.Fatal(err)
			}
			if got.Reconciliation.Status != tt.wantReconciliation || got.Availability.Status != tt.wantAvailability {
				t.Errorf("assessment = %s + %s, want %s + %s", got.Reconciliation, got.Availability, tt.wantReconciliation, tt.wantAvailability)
			}
			if got.Reconciliation.Message != tt.wantReconcileMessage {
				t.Errorf("reconciliation message = %q, want %q", got.Reconciliation.Message, tt.wantReconcileMessage)
			}
			if got.Availability.Message != tt.wantAvailabilityMessage {
				t.Errorf("availability message = %q, want %q", got.Availability.Message, tt.wantAvailabilityMessage)
			}
		})
	}
}

func TestRolloutBlueGreen(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(map[string]any)
		want    kubehealth.ReconciliationStatus
		message string
	}{
		{name: "healthy blue green", want: kubehealth.ReconciliationReconciled, message: "Rollout is reconciled"},
		{name: "active service cutover pending", mutate: func(obj map[string]any) {
			obj["status"].(map[string]any)["blueGreen"] = map[string]any{"activeSelector": "old-hash"}
		}, want: kubehealth.ReconciliationInProgress, message: "active service cutover pending"},
		{name: "analysis pending", mutate: func(obj map[string]any) {
			obj["status"].(map[string]any)["stableRS"] = "old-hash"
		}, want: kubehealth.ReconciliationInProgress, message: "waiting for analysis to complete"},
	}

	assessor := kubehealth.NewAssessor()
	if err := argorollouts.Register(assessor); err != nil {
		t.Fatal(err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			object := healthyBlueGreen()
			if tt.mutate != nil {
				tt.mutate(object)
			}
			got, err := assessor.Assess(&unstructured.Unstructured{Object: object})
			if err != nil {
				t.Fatal(err)
			}
			if got.Reconciliation.Status != tt.want || got.Reconciliation.Message != tt.message {
				t.Errorf("reconciliation = %q (%q), want %q (%q)", got.Reconciliation, got.Reconciliation.Message, tt.want, tt.message)
			}
		})
	}
}

func healthyCanary() map[string]any {
	return map[string]any{
		"apiVersion": "argoproj.io/v1alpha1", "kind": "Rollout",
		"metadata": map[string]any{"name": "example", "generation": int64(1)},
		"spec":     map[string]any{"replicas": int64(5), "strategy": map[string]any{"canary": map[string]any{}}},
		"status": map[string]any{
			"observedGeneration": "1", "currentPodHash": "new-hash", "stableRS": "new-hash",
			"replicas": int64(5), "updatedReplicas": int64(5), "availableReplicas": int64(5),
			"conditions": []any{condition("Available", "True", "AvailableReason", "Rollout has minimum availability")},
		},
	}
}

func healthyBlueGreen() map[string]any {
	object := healthyCanary()
	object["spec"].(map[string]any)["strategy"] = map[string]any{"blueGreen": map[string]any{"activeService": "example-active"}}
	object["status"].(map[string]any)["blueGreen"] = map[string]any{"activeSelector": "new-hash"}
	return object
}

func condition(conditionType, status, reason, message string) map[string]any {
	return map[string]any{"type": conditionType, "status": status, "reason": reason, "message": message}
}

const workloadGenerationAnnotation = "rollout.argoproj.io/workload-generation"
