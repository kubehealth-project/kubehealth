# KubeHealth design decisions

KubeHealth is the name of the health contract and Go package. `Assessment` is
the result of evaluating one resource. This supports natural ecosystem language
such as "KubeHealth-compliant resource" and "KubeHealth assessment".

## Health has three dimensions

The library answers three independent questions:

1. Has the resource reconciled to its latest desired state?
2. Is the resource operational right now?
3. Is the resource active, terminating, or absent?

The canonical result is:

```go
type Assessment struct {
	Reconciliation Dimension[ReconciliationStatus]
	Availability   Dimension[AvailabilityStatus]
	Lifecycle      Dimension[LifecycleStatus]
}
```

Resource fields and Kubernetes conditions are assessment inputs. The output is
only the three normalized dimensions. This prevents consumers from having to
reinterpret resource-specific condition types and avoids two competing sources
of health truth.

## Reconciliation

Reconciliation answers whether desired and actual state have converged:

- `Reconciled`
- `InProgress`
- `Failed`
- `Suspended`
- `Unknown`
- `NotApplicable`

kstatus defines status as a single reconcile state at a point in time. This
dimension retains that scalar contract for predictable waiting and aggregation.

## Availability

Availability answers whether the resource currently provides its intended
capability:

- `Available`
- `PartiallyAvailable`
- `Unavailable`
- `Unknown`
- `NotApplicable`

A failed reconcile does not imply an outage. A Deployment rollout can fail while
healthy old replicas still serve. A certificate renewal can fail while the
current certificate remains valid.

## Lifecycle

Lifecycle answers whether the resource exists and whether it is being deleted:

- `Active`
- `Terminating`
- `NotFound`
- `Unknown`

kstatus places `Terminating` and `NotFound` in the same enum as reconciliation
states. This library separates them because a terminating resource may still be
available, and deletion is not a reconciliation failure.

### Current object-only limitation

`Assess` accepts an object that has already been retrieved. Receiving that object
proves that it exists, so this API can directly infer only `Active` and
`Terminating`. It cannot produce `NotFound` from an absent object, and an error
while interpreting status does not make the object's existence unknown.

`NotFound` is reserved for a future fetch-aware API that can distinguish absence
from lookup failure. Related resources and end-to-end application health are
also outside the object-only assessment boundary.

## Example Deployment states

- `Reconciled + Available + Active`: the desired rollout is serving.
- `InProgress + Available + Active`: old replicas serve during rollout.
- `InProgress + PartiallyAvailable + Active`: rollout proceeds with reduced
  capacity.
- `InProgress + Unavailable + Active`: an initial rollout has no available
  replicas.
- `Failed + Available + Active`: the new rollout failed but old replicas serve.
- `Failed + Unavailable + Active`: the rollout failed and no replicas serve.
- `Unknown + Available + Terminating`: deletion started while replicas still
  serve.

## Precedence and merging

An operator-published report under `status.kubeHealth` has the highest health
precedence when it is complete, valid, current for `metadata.generation`, and
consistent with `metadata.deletionTimestamp`. KubeHealth returns such a report
without evaluating generic signals or running a registered check. This makes
the resource's own controller the authoritative source when it opts into the
contract.

When no authoritative report is available, generic KubeHealth signals determine
reconciliation before resource-specific rules. A resource-specific check still
runs to determine availability.

This prevents custom checks from overriding explicit `Stalled=True` or stale
observed-generation signals, while retaining operational information only the
resource-specific check can compute.

Lifecycle is evaluated separately. A deletion timestamp sets lifecycle to
`Terminating` without claiming that the resource is unavailable.

An observed generation is stale only when it is lower than the object's desired
generation. A higher observed value is accepted rather than treated as stale.

The embedded status contract is stricter: its `observedGeneration` must equal
`metadata.generation` for the fast path. A lower value means the complete report
is stale. In that case KubeHealth reports `InProgress`, keeps the published
availability when no check is registered, and still runs a registered check so
that resource-specific current availability can replace it. A higher value is
invalid because it cannot honestly describe the object's current generation.

### Operator-published status contract

The persisted contract is versioned independently from the owning resource and
lives in `api/v1alpha1`. Its wire location is `status.kubeHealth`:

```yaml
kubeHealth:
  contractVersion: v1alpha1
  observedGeneration: 12
  reconciliation:
    status: Failed
    reason: ProgressDeadlineExceeded
    message: The desired revision did not become ready
    lastTransitionTime: "2026-09-13T12:00:00Z"
  availability:
    status: Available
    reason: PreviousRevisionServing
    message: The previous revision continues to serve traffic
    lastTransitionTime: "2026-09-13T11:55:00Z"
  lifecycle:
    status: Active
    reason: DeletionNotRequested
    lastTransitionTime: "2026-09-13T10:00:00Z"
```

The nested shape keeps each dimension's status, reason, message, and transition
time together. The same `Dimension[T]` type is used by the persisted status and
the in-memory `Assessment`, so all four fields remain available when an
operator-published report is returned. Reason, message, and transition time are
optional in the first contract version.

Checks computed from an object may not know a meaningful transition time and can
leave it empty. Operator-published reports can maintain it across reconciliations
with the `api/v1alpha1` setter helpers.

The report is all-or-nothing. All three status values, the contract version,
and observed generation must be present before it can be authoritative.
Explicit `Unknown` and `NotApplicable` values count as present. A partial report
is ignored instead of being merged dimension by dimension with inference.

`metadata.deletionTimestamp` remains authoritative because it is managed by the
API server and can change after the controller's latest status update. A current
report can return immediately during deletion only when its lifecycle is
`Terminating`. A report that remains `Active` is no longer eligible and the
normal pipeline derives termination from metadata. `Terminating` without a
deletion timestamp is invalid. `NotFound` cannot be written inside an object
that exists.

The contract intentionally does not encode KubeHealth as three
`metav1.Condition` entries. Standard conditions expose only `True`, `False`,
and `Unknown`, which cannot preserve `InProgress`, `Failed`, `Suspended`,
`PartiallyAvailable`, and `NotApplicable` without misusing `reason` as the
primary state. Operators should continue to publish their normal conditions in
parallel for Kubernetes ecosystem tooling.

This follows the relevant Kubernetes API conventions: observed state belongs
under status, controllers should write through the status subresource,
`observedGeneration` identifies freshness, reasons are programmatic CamelCase
identifiers, messages are human-readable, and transition times change only when
the observed status changes. CRDs should not default this field because absence
must mean no authoritative report has been published.

See the Kubernetes
[API conventions for spec and status](https://github.com/kubernetes/community/blob/main/contributors/devel/sig-architecture/api-conventions.md#spec-and-status)
for the conventions reused by this contract.

## Parent health from children

When a consumer represents a parent as a collection of child resources,
KubeHealth provides deliberately simple defaults for reconciliation and
availability. It does not infer the parent's lifecycle because child deletion
does not necessarily mean that the parent is terminating.

Reconciliation uses `Failed`, then `InProgress`, then `Suspended`, then `Unknown`
precedence. The parent is `Reconciled` only when every applicable child is
reconciled. Empty and entirely not-applicable child sets produce
`NotApplicable`.

Availability ignores `NotApplicable`. It returns `PartiallyAvailable` when a
child is explicitly partial or known evidence includes both available and
unavailable children. Otherwise unknown evidence prevents an optimistic all-
available or all-unavailable result. A homogeneous known set produces
`Available` or `Unavailable`, and no applicable children produces
`NotApplicable`.

The default is intentionally a brain-dead, good-enough policy. Availability can
mean quorum, weighted capacity, required roles, or at least one serving child,
depending on the parent. Operators can reuse `AggregateReconciliation` and
replace `AggregateAvailability` with their own function. This keeps override
logic ordinary Go rather than introducing a policy abstraction before concrete
use cases require one.

The complete availability examples and operator override pattern are documented
in [`README.md`](README.md#availability).

## Shared Dimension[T] fields

The API uses one shared generic dimension type for each health axis:

```go
Reconciliation Dimension[ReconciliationStatus]
Availability   Dimension[AvailabilityStatus]
Lifecycle      Dimension[LifecycleStatus]
```

rather than maintaining separate in-memory and persisted dimension models.

Reasons:

- Status, reason, message, and transition time remain together.
- The same type is used by resource checks and operator-published status.
- JSON remains flat at the three dimension keys and nested within each dimension.
- Generic code can work consistently across the three dimension types.

## kstatus is a derived compatibility projection

The three-dimensional KubeHealth Assessment is canonical and is the only health
decision produced by this library. Consumers that already use kstatus types can
request the same decision in a single-value kstatus shape:

The adapter exposes projection for an existing assessment:

```go
func kstatusadapter.Project(assessment kubehealth.Assessment) *kstatus.Result
```

and a convenience wrapper that assesses first:

```go
func kstatusadapter.Assess(obj *unstructured.Unstructured) (*kstatus.Result, error)
```

The optional adapter always calls `kubehealth.Assess` first. It does not duplicate or bypass
resource-specific health logic. The projection itself is fully generic.

### Type compatibility, not decision-logic compatibility

KubeHealth does not call `kstatus.Compute`, reproduce its resource-specific
rules, or promise the same decision that upstream kstatus would make for an
object. All resource interpretation belongs to KubeHealth.

The compatibility contract covers only:

- The upstream `kstatus.Result` return type.
- The upstream `kstatus.Status` values.
- A deterministic projection from KubeHealth dimensions to one kstatus value.

It does not cover:

- Upstream kstatus Deployment, Pod, Service, or other built-in rules.
- Upstream kstatus precedence or optimistic fallback behavior.
- Equality with `kstatus.Compute(obj)`.

This distinction lets an existing kstatus consumer adopt KubeHealth without
changing its surrounding types while still receiving KubeHealth's health
decision.

The compatibility API imports and returns the upstream kstatus result and status
types rather than redefining lookalikes. The projection does not populate
kstatus conditions because source conditions are assessment inputs, not part of
the normalized KubeHealth result.

Lifecycle takes precedence. Active resources then map reconciliation directly.
Availability is omitted because kstatus has no availability dimension.

This conversion is intentionally lossy:

- `Failed + Available + Active` becomes `Failed`.
- `Failed + Unavailable + Active` also becomes `Failed`.
- `Unknown + Available + Terminating` becomes `Terminating`.

Consumers that need operational distinctions must use the canonical Assessment.

Tests for this adapter verify the KubeHealth Assessment and its projected
kstatus shape. They intentionally do not call or compare with
`kstatus.Compute`.

## Dimension metadata

Each dimension has its own optional reason, message, and transition time. This
avoids one ambiguous explanation having to describe reconciliation failure,
current availability, and lifecycle at the same time. Checks normalize source
conditions into this metadata rather than returning the source conditions.

## Exact GVK registration

Checks register by Group, Version, and Kind. Status semantics can change between
API versions, and exact matching prevents a check written for one schema from
silently interpreting another.

## Scale integrations by owning project

Third-party checks do not belong in one flat core package. They are grouped by
the project that owns the APIs:

```text
integrations/<project>/register.go
integrations/<project>/<resource>.go
integrations/<project>/<project>_test.go
```

This layout provides:

- Clear ownership and review boundaries.
- One package per upstream project rather than one package per resource.
- Shared helpers for resources from the same API family.
- Selective imports so consumers do not pull every integration.
- A predictable location when an upstream project adds an API version.

Each integration exports one `Register(*kubehealth.Assessor) error` function.
The central catalog composes these functions as a convenience, but the core
package does not import integrations. This avoids import cycles and keeps the
base library stable as the ecosystem catalog grows.

Checks register exact GVKs. Multiple API versions receive separate entries and
can share internal logic only when their schemas and health semantics are truly
compatible.

## Go and Lua checks share one contract

The assessor registry stores `Check` functions. A Go integration implements the
function directly, while the optional `lua` package adapts a script into the
same function type. The assessor therefore applies identical GVK matching,
generic-signal precedence, validation, and child aggregation regardless of the
implementation language.

Lua is intended as a configuration extension mechanism, not a separate health
model. Scripts receive one unstructured object and may inspect its fields and
conditions. They return only the canonical three-dimensional `Assessment`,
including optional per-dimension reasons and messages. They run with restricted
libraries and a time limit. Loading scripts from files, ConfigMaps, or another
configuration source remains the consumer's responsibility.

The built-in `apps/v1` StatefulSet check is implemented in Lua and embedded in
the Go package. This proves that Lua checks can be shipped as library defaults,
not only loaded dynamically by consumers.

## Separate framework, API, built-ins, and integrations

The implementation is split into layers:

```text
kubehealth/api           stable assessment model
kubehealth/api/v1alpha1  operator-published status contract
kubehealth/builtins      Kubernetes-native resource checks
kubehealth/internal      shared non-public implementation helpers
kubehealth/integrations  third-party project checks
kubehealth/catalog       optional integration composition
kubehealth               public assessor and generic-signal evaluation
```

The `api` package exists to keep dependencies acyclic. Both `builtins` and
third-party integrations depend on the model without depending on the assessor
implementation. The root package aliases the API types, so consumers continue
to use `kubehealth.Assessment` and `kubehealth.Check`.

`kubehealth.NewAssessor()` automatically registers `builtins`. Third-party
integrations remain opt-in, either through their project package or through the
catalog convenience constructor.

This separation prevents the root package from becoming a flat collection of
resource files and creates clear ownership boundaries as the catalog grows.

Generic helpers are placed under `internal`, not under whichever resource group
first needed them. The shared condition parser is therefore
`internal/conditions`, while condition semantics remain in each resource check.
Conditions are inputs and are not exposed in `Assessment`.

Resource behavior tests are colocated with the resource implementation using
`<resource>_test.go`. This applies to both built-ins and integrations. Root tests
are reserved for framework behavior that crosses resources, such as generic
signal precedence and kstatus projection.

## Resource-specific checks own Ready semantics

The core honors generic `Reconciling` and `Stalled` conditions.
`Ready` is not treated as universally sufficient because its meaning and
failure polarity vary across APIs. Integration checks interpret `Ready` and any
project-specific conditions such as cert-manager `Issuing` or KEDA `Fallback`.

This keeps KubeHealth extensible without making the core guess whether
`Ready=False` is temporary, terminal, available through an old version, or
unavailable.

## Unknown rather than assumed reconciled

Original kstatus treats an unknown resource without recognized conditions as
`Current`. This library returns `Unknown`. Absence of a known failure signal is
not evidence that reconciliation succeeded or that the resource is available.

## Availability scope

Availability is based on information available to the check. It is not a promise
of end-to-end application health.

- ClusterIP Service requires EndpointSlice information.
- Bound PVC does not prove successful attachment or mount.
- HPA controls scale and has `NotApplicable` availability.

Checks use `Unknown` when related information is missing and `NotApplicable`
when availability is not meaningful for the resource.
