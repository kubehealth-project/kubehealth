package lua_test

import (
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kubehealth-project/kubehealth"
	kubehealthlua "github.com/kubehealth-project/kubehealth/lua"
)

const readyScript = `
local ready = false
if obj.status ~= nil and obj.status.conditions ~= nil then
  for _, condition in ipairs(obj.status.conditions) do
    if condition.type == "Ready" and condition.status == "True" then
      ready = true
    end
  end
end

if ready then
  return {
    reconciliation = "Reconciled",
    availability = "Available",
    lifecycle = "Active",
    reconciliationReason = "ReconciliationSucceeded",
    reconciliationMessage = "Widget is reconciled",
    availabilityReason = "Ready",
    availabilityMessage = "Widget is ready",
  }
end

return {
  reconciliation = "InProgress",
  availability = "Unavailable",
  lifecycle = "Active",
  reconciliationMessage = "Waiting for Widget",
  availabilityMessage = "Widget is not ready"
}
`

func TestNewCheckAssessesObject(t *testing.T) {
	check, err := kubehealthlua.NewCheck(readyScript)
	if err != nil {
		t.Fatal(err)
	}

	assessment, err := check(widget(true))
	if err != nil {
		t.Fatal(err)
	}
	if assessment.Reconciliation.Status != kubehealth.ReconciliationReconciled {
		t.Fatalf("reconciliation = %q", assessment.Reconciliation)
	}
	if assessment.Availability.Status != kubehealth.AvailabilityAvailable {
		t.Fatalf("availability = %q", assessment.Availability)
	}
	if assessment.Reconciliation.Reason != "ReconciliationSucceeded" || assessment.Availability.Reason != "Ready" {
		t.Fatalf("reasons = %q, %q", assessment.Reconciliation.Reason, assessment.Availability.Reason)
	}
}

func TestRegisterUsesNormalAssessorPrecedence(t *testing.T) {
	assessor := kubehealth.NewAssessor()
	gvk := schema.GroupVersionKind{Group: "example.io", Version: "v1", Kind: "Widget"}
	if err := kubehealthlua.Register(assessor, gvk, readyScript); err != nil {
		t.Fatal(err)
	}

	obj := widget(true)
	obj.SetGeneration(2)
	obj.Object["status"].(map[string]any)["observedGeneration"] = int64(1)
	assessment, err := assessor.Assess(obj)
	if err != nil {
		t.Fatal(err)
	}
	if assessment.Reconciliation.Status != kubehealth.ReconciliationInProgress {
		t.Fatalf("reconciliation = %q, want %q", assessment.Reconciliation, kubehealth.ReconciliationInProgress)
	}
	if assessment.Availability.Status != kubehealth.AvailabilityAvailable {
		t.Fatalf("availability = %q, want %q", assessment.Availability, kubehealth.AvailabilityAvailable)
	}
}

func TestNewCheckRejectsInvalidResults(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "syntax", script: `return {`, want: "compile Lua health check"},
		{name: "non-table", script: `return "healthy"`, want: "must return a table"},
		{name: "missing field", script: `return { availability = "Available", lifecycle = "Active" }`, want: `field "reconciliation"`},
		{name: "invalid enum", script: `return { reconciliation = "Perfect", availability = "Available", lifecycle = "Active" }`, want: `invalid reconciliation value "Perfect"`},
		{name: "invalid message", script: `return { reconciliation = "Reconciled", availability = "Available", lifecycle = "Active", availabilityMessage = 42 }`, want: `field "availabilityMessage" must be a string`},
		{name: "invalid reason", script: `return { reconciliation = "Reconciled", reconciliationReason = 42, availability = "Available", lifecycle = "Active" }`, want: `field "reconciliationReason" must be a string`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			check, err := kubehealthlua.NewCheck(test.script)
			if err == nil {
				_, err = check(widget(false))
			}
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestCheckTimesOut(t *testing.T) {
	check, err := kubehealthlua.NewCheck(`while true do end`, kubehealthlua.WithTimeout(10*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	_, err = check(widget(false))
	if err == nil || !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("error = %v, want timeout", err)
	}
}

func TestRestrictedLibraries(t *testing.T) {
	check, err := kubehealthlua.NewCheck(`
if os ~= nil or io ~= nil or package ~= nil or require ~= nil or dofile ~= nil or loadfile ~= nil then
  error("unsafe library is available")
end
return { reconciliation = "Reconciled", availability = "Available", lifecycle = "Active" }
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := check(widget(false)); err != nil {
		t.Fatal(err)
	}
}

func widget(ready bool) *unstructured.Unstructured {
	status := "False"
	if ready {
		status = "True"
	}
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "example.io/v1",
		"kind":       "Widget",
		"metadata": map[string]any{
			"name": "example", "generation": int64(1),
		},
		"status": map[string]any{
			"observedGeneration": int64(1),
			"conditions": []any{map[string]any{
				"type": "Ready", "status": status,
			}},
		},
	}}
}
