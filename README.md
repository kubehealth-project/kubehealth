# KubeHealth

KubeHealth is a Kubernetes resource health contract. It combines kstatus
conventions with a registry of resource-specific health checks and separates
reconciliation, availability, and lifecycle.

A resource check is KubeHealth-compliant when it returns all three dimensions,
uses the defined values, reports unknown information as `Unknown`, and does not
conflate failed reconciliation with unavailable operation.

```go
assessment, err := kubehealth.Assess(obj)
```

`Assess` is the primary API. Consumers that require the original single-value
kstatus model can use:

```go
kstatusResult, err := kstatusadapter.Assess(obj)
```

## Aggregate child health

Consumers that model a parent from a set of child resources can assess and
aggregate those children with:

```go
assessment, err := kubehealth.AssessChildren(children)
```

An assessor with registered integrations exposes the same operation:

```go
assessment, err := assessor.AssessChildren(children)
```

The result contains only the dimensions that can be derived generically from
children:

```go
type ChildrenAssessment struct {
	Reconciliation ReconciliationStatus
	Availability   AvailabilityStatus
}
```

Lifecycle is omitted because a terminating child does not imply that its parent
is terminating.

### Reconciliation

The aggregation precedence is:

1. `Failed` when any child is failed.
2. `InProgress` when no child failed and any child is in progress.
3. `Suspended` when no child failed or is in progress and any child is suspended.
4. `Unknown` when no child failed, is in progress, or is suspended and any applicable child is
   unknown.
5. `Reconciled` when every applicable child is reconciled.
6. `NotApplicable` when the list is empty or every child is not applicable.

`NotApplicable` children are ignored when applicable children exist. Consumers
that already have assessments can use `AggregateReconciliation` directly.

### Availability

The default availability policy is deliberately conservative. `NotApplicable`
children are ignored.

| Child availability values | Parent availability |
| --- | --- |
| No children | `NotApplicable` |
| `NotApplicable`, `NotApplicable` | `NotApplicable` |
| `Available`, `Available` | `Available` |
| `Unavailable`, `Unavailable` | `Unavailable` |
| `Available`, `Unavailable` | `PartiallyAvailable` |
| `PartiallyAvailable`, `Available` | `PartiallyAvailable` |
| `Available`, `Unknown` | `Unknown` |
| `Unavailable`, `Unknown` | `Unknown` |
| `PartiallyAvailable`, `Unknown` | `PartiallyAvailable` |
| `Available`, `Unavailable`, `Unknown` | `PartiallyAvailable` |
| `NotApplicable`, `Available` | `Available` |

`Unknown` means that evidence is missing, not that the child is partially
available. However, an explicitly partial child or a known mix of available and
unavailable children already proves partial availability, even if another child
is unknown.

This is a brain-dead, good-enough default rather than universal application
semantics. An operator that needs quorum, optional children, weighted capacity,
or role-aware behavior should assess children normally and replace only the
availability step:

```go
assessment := kubehealth.ChildrenAssessment{
	Reconciliation: kubehealth.AggregateReconciliation(children),
	Availability:   myAvailabilityPolicy(children),
}
```

Callers that accept the defaults can use `AggregateChildren`. The lower-level
`AssessChildrenReconciliation` API remains available for callers interested only
in reconciliation.

## Assessment

The KubeHealth assessment uses three scalar dimensions with separate messages:

```go
type Assessment struct {
	Reconciliation ReconciliationStatus `json:"reconciliation"`
	Availability   AvailabilityStatus   `json:"availability"`
	Lifecycle      LifecycleStatus      `json:"lifecycle"`

	ReconciliationMessage string `json:"reconciliationMessage,omitempty"`
	AvailabilityMessage   string `json:"availabilityMessage,omitempty"`
	LifecycleMessage      string `json:"lifecycleMessage,omitempty"`

	Conditions []Condition `json:"conditions,omitempty"`
}
```

Reconciliation uses `Reconciled`, `InProgress`, `Failed`, `Suspended`, `Unknown`, or
`NotApplicable`.

Availability uses `Available`, `PartiallyAvailable`, `Unavailable`, `Unknown`,
or `NotApplicable`.

Lifecycle uses `Active`, `Terminating`, `NotFound`, or `Unknown`.

A failed Deployment rollout with healthy old replicas can return:

```json
{
  "reconciliation": "Failed",
  "availability": "Available",
  "lifecycle": "Active",
  "reconciliationMessage": "Deployment \"api\" exceeded its progress deadline",
  "availabilityMessage": "3 replicas are available"
}
```

## Assessment order

1. Evaluate deletion and presence signals for lifecycle.
2. Evaluate standard kstatus signals for reconciliation.
3. Run the registered resource-specific check to compute availability.
4. Use the custom check's reconciliation result only when no standard signal
   already decided it.
5. If no check is registered, retain any standard reconciliation result and set
   availability to `Unknown`.
6. If neither standard signals nor a check can assess the resource, return
   `Unknown` for reconciliation and availability.

Standard reconciliation signals do not suppress resource-specific checks. This
allows combinations such as `Failed + Available` and
`InProgress + PartiallyAvailable`.

## Built-in checks

Kubernetes-native checks live in the dedicated `builtins` package rather than
the KubeHealth framework root:

```text
kubehealth/
  api/
    model.go
  builtins/
    register.go
    deployment.go
    statefulset.go
    lua/
      statefulset.lua
    service.go
    pod.go
    persistentvolumeclaim.go
    hpa.go
  internal/
    conditions/
      conditions.go
  integrations/
  catalog/
  health.go
  standard.go
```

`kubehealth.NewAssessor()` registers `builtins` automatically, so this
organization does not change the public API.

Generic parsing and implementation helpers live under `internal`, separate
from both resource checks and the public API.

- `apps/v1` Deployment
- `apps/v1` StatefulSet, implemented as an embedded Lua check
- `apps/v1` DaemonSet
- `v1` Service
- `v1` Pod
- `v1` PersistentVolumeClaim
- `v1` ConfigMap
- `v1` Secret
- `autoscaling/v1` HorizontalPodAutoscaler
- `autoscaling/v2` HorizontalPodAutoscaler

The `autoscaling/v2` HPA check evaluates its standardized status conditions.
The `autoscaling/v1` API does not expose those conditions, so its object-only
assessment reports reconciliation as `Unknown`. KubeHealth does not interpret
the historical alpha conditions annotation or register removed beta APIs.

## Integration catalog

Ecosystem checks live outside the core package, grouped by owning project:

```text
integrations/
    certmanager/
      register.go
      certificate.go
      certificate_test.go
      issuer.go
      issuer_test.go
    kyverno/
      register.go
      policy.go
      policy_test.go
      clusterpolicy_test.go
    keda/
      register.go
      scaledobject.go
      scaledobject_test.go
    argorollouts/
      register.go
      rollout.go
      rollout_test.go
```

The demonstration catalog includes:

- cert-manager `Certificate`
- cert-manager `Issuer`
- Kyverno `Policy`
- Kyverno `ClusterPolicy`
- KEDA `ScaledObject`
- Argo Rollouts `Rollout`

The Rollout check reports reconciliation and replica availability independently.
A failed or paused rollout can therefore remain `Available` when its stable
replicas continue serving.

Consumers can select only the integrations they need:

```go
assessor := kubehealth.NewAssessor()
if err := certmanager.Register(assessor); err != nil {
	return err
}
```

Or use the catalog convenience constructor:

```go
assessor, err := catalog.NewAssessor()
```

Tests are colocated at resource level. `builtins/deployment_test.go` tests
Deployment, while files such as `integrations/certmanager/certificate_test.go`
test one CRD. Every added resource must include tests for its exact GVK and
meaningful health states. The catalog has only an additional registration smoke
test.

Core `kubehealth.Assess` automatically includes checks from `builtins`. It does
not automatically import ecosystem integrations.

## Write checks in Go or Lua

Resource health checks are first-class whether they are implemented in Go or
Lua. Both register an exact Group, Version, and Kind in the same assessor and
produce the same `Assessment`. Standard KubeHealth reconciliation signals retain
the same precedence for both implementations.

Use Go for compiled integrations:

```go
err := assessor.Register(gvk, func(obj *unstructured.Unstructured) (kubehealth.Assessment, error) {
	return kubehealth.Assessment{
		Reconciliation: kubehealth.ReconciliationReconciled,
		Availability:   kubehealth.AvailabilityAvailable,
		Lifecycle:      kubehealth.LifecycleActive,
	}, nil
})
```

Use Lua when the check should be supplied as configuration:

```go
err := lua.Register(assessor, gvk, script)
```

The script receives the resource as the global `obj` table and returns the same
three dimensions:

```lua
if obj.status ~= nil and obj.status.ready == true then
  return {
    reconciliation = "Reconciled",
    availability = "Available",
    lifecycle = "Active",
    availabilityMessage = "Resource is ready"
  }
end

return {
  reconciliation = "InProgress",
  availability = "Unavailable",
  lifecycle = "Active",
  reconciliationMessage = "Waiting for readiness"
}
```

Lua checks run with a restricted standard library and a default 500 millisecond
timeout. They cannot load files, packages, operating-system functions, or I/O.
The returned table is validated against the KubeHealth status values. A custom
timeout can be configured with `lua.WithTimeout`.

## Deployment scenario tests

The optional `kstatus` adapter module can project a Deployment assessment:

```go
assessment, err := kubehealth.Assess(obj)
kstatusResult := kstatusadapter.Project(assessment)
```

The scenarios demonstrate that kstatus is a lossy projection. For example:

- A failed rollout with old replicas serving is
  `Failed + Available + Active`, projected to kstatus `Failed`.
- A failed initial rollout is `Failed + Unavailable + Active`, also projected
  to kstatus `Failed`.
- A rolling update while old replicas serve is
  `InProgress + Available + Active`, projected to kstatus `InProgress`.
- A terminating Deployment can be `Unknown + Available + Terminating`,
  projected to kstatus `Terminating`.

## Register a CRD check

```go
assessor := kubehealth.NewAssessor()

err := assessor.Register(
	schema.GroupVersionKind{
		Group:   "policies.kyverno.io",
		Version: "v1",
		Kind:    "CleanupPolicy",
	},
	func(obj *unstructured.Unstructured) (kubehealth.Assessment, error) {
		state, _, err := unstructured.NestedString(
			obj.Object,
			"status",
			"state",
		)
		if err != nil {
			return kubehealth.Assessment{}, err
		}

		if state == "Active" {
			return kubehealth.Assessment{
				Reconciliation:        kubehealth.ReconciliationReconciled,
				Availability:          kubehealth.AvailabilityNotApplicable,
				Lifecycle:             kubehealth.LifecycleActive,
				ReconciliationMessage: "CleanupPolicy is active",
			}, nil
		}

		return kubehealth.Assessment{
			Reconciliation:        kubehealth.ReconciliationInProgress,
			Availability:          kubehealth.AvailabilityNotApplicable,
			Lifecycle:             kubehealth.LifecycleActive,
			ReconciliationMessage: "CleanupPolicy is becoming active",
		}, nil
	},
)
```

Standard kstatus conditions retain precedence for reconciliation. The custom
check still runs to supply availability. Checks register by exact Group,
Version, and Kind so health logic can evolve with API versions.

## kstatus compatibility

The optional `kstatus` adapter module first calls the main `Assess`
implementation and then projects
the three dimensions into one kstatus code. It returns the upstream
`sigs.k8s.io/cli-utils/pkg/kstatus/status.Result` type directly. The projection
is generic and does not contain resource-specific rules.

This is type and shape compatibility, not compatibility with upstream
`kstatus.Compute` decision logic. KubeHealth remains the only health decision
engine. The wrapper does not call `kstatus.Compute` or copy its resource rules.

Projection order:

1. `Lifecycle=Terminating` becomes `Terminating`.
2. `Lifecycle=NotFound` becomes `NotFound`.
3. `Lifecycle=Unknown` becomes `Unknown`.
4. Active resources map reconciliation directly:
   - `Reconciled` becomes `Current`.
   - `InProgress` becomes `InProgress`.
   - `Failed` becomes `Failed`.
   - `Unknown` or `NotApplicable` becomes `Unknown`.

Availability does not alter the kstatus projection. Both `Failed + Available`
and `Failed + Unavailable` project to `Failed`. Consumers that need the
distinction should use `kubehealth.Assess` rather than the optional adapter.

The same projection is available for an existing assessment:

```go
kstatusResult := kstatusadapter.Project(assessment)
```

## Design summary

- `Reconciled`, `InProgress`, and `Failed` belong to reconciliation.
- `Terminating` and `NotFound` belong to lifecycle.
- Availability remains independent from both.
- Each dimension has one scalar value and its own optional message.
- Unknown resources do not default optimistically to `Reconciled`.
- A generic `Dimension[T]` wrapper is intentionally deferred.
- The kstatus wrapper is a lossy compatibility projection, not the canonical
  health model.

See [`DESIGN.md`](DESIGN.md) for the full rationale.

## KubeHealth compliance

To call a check KubeHealth-compliant, it must:

1. Return reconciliation, availability, and lifecycle.
2. Evaluate the latest desired generation where the resource supports it.
3. Preserve standard kstatus reconciliation signals.
4. Keep availability independent from reconciliation failure.
5. Use `Unknown` when the object does not provide enough evidence.
6. Use `NotApplicable` only when a dimension is not meaningful.
7. Register against an exact Group, Version, and Kind.
8. Include tests for the resource's meaningful state combinations.

See [`CONTRIBUTING_CHECKS.md`](CONTRIBUTING_CHECKS.md) for the integration
layout and contribution rules.
