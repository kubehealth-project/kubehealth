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
	Reconciliation ReconciliationStatus
	Availability   AvailabilityStatus
	Lifecycle      LifecycleStatus

	ReconciliationMessage string
	AvailabilityMessage   string
	LifecycleMessage      string

	Conditions []Condition
}
```

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

This leaves an intentional open question for review: either lifecycle should be
limited to `Active` and `Terminating`, or absence should be handled by a separate
fetch-aware API that can distinguish `NotFound` from lookup failure. Related
resources and end-to-end application health are also outside the object-only
assessment boundary.

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

Generic KubeHealth signals determine reconciliation before resource-specific
rules. A resource-specific check still runs to determine availability.

This prevents custom checks from overriding explicit `Stalled=True` or stale
observed-generation signals, while retaining operational information only the
resource-specific check can compute.

Lifecycle is evaluated separately. A deletion timestamp sets lifecycle to
`Terminating` without claiming that the resource is unavailable.

An observed generation is stale only when it is lower than the object's desired
generation. A higher observed value is accepted rather than treated as stale.

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

## Scalar fields instead of Dimension[T]

The API intentionally uses direct scalar fields:

```go
Reconciliation ReconciliationStatus
Availability   AvailabilityStatus
Lifecycle      LifecycleStatus
```

instead of:

```go
Reconciliation Dimension[ReconciliationStatus]
```

Reasons:

- Direct values are easy to compare in consumer policy.
- JSON remains flat and easy to read.
- The three-dimensional model is visible without generic syntax.
- Each dimension currently needs only a value and a message.

A generic wrapper can be reconsidered if every dimension later needs shared
metadata such as reason, timestamp, confidence, or evidence.

## kstatus is a derived compatibility projection

The three-dimensional KubeHealth Assessment is canonical and is the only health
decision produced by this library. Consumers that already use kstatus types can
request the same decision in a single-value kstatus shape:

```go
func (a Assessment) KStatus() *kstatus.Result
```

or the convenience wrapper:

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
- The upstream `kstatus.Condition` representation.
- A deterministic projection from KubeHealth dimensions to one kstatus value.

It does not cover:

- Upstream kstatus Deployment, Pod, Service, or other built-in rules.
- Upstream kstatus precedence or optimistic fallback behavior.
- Equality with `kstatus.Compute(obj)`.

This distinction lets an existing kstatus consumer adopt KubeHealth without
changing its surrounding types while still receiving KubeHealth's health
decision.

The compatibility API imports and returns the upstream kstatus types rather
than redefining lookalike status, result, and condition types. This provides
compile-time compatibility for existing kstatus consumers and avoids drift when
using the wrapper.

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

## Dimension-specific messages

Each dimension has its own optional message. This avoids one ambiguous message
having to explain reconciliation failure, current availability, and lifecycle at
the same time.

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
standard-signal precedence, validation, and child aggregation regardless of the
implementation language.

Lua is intended as a configuration extension mechanism, not a separate health
model. Scripts receive one unstructured object and return the canonical
three-dimensional `Assessment`. They run with restricted libraries and a time
limit. Loading scripts from files, ConfigMaps, or another configuration source
remains the consumer's responsibility.

The built-in `apps/v1` StatefulSet check is implemented in Lua and embedded in
the Go package. This proves that Lua checks can be shipped as library defaults,
not only loaded dynamically by consumers.

## Separate framework, API, built-ins, and integrations

The implementation is split into layers:

```text
kubehealth/api           stable model and kstatus adapter
kubehealth/builtins      Kubernetes-native resource checks
kubehealth/internal      shared non-public implementation helpers
kubehealth/integrations  third-party project checks
kubehealth/catalog       optional integration composition
kubehealth               public assessor and standard-signal evaluation
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

Resource behavior tests are colocated with the resource implementation using
`<resource>_test.go`. This applies to both built-ins and integrations. Root tests
are reserved for framework behavior that crosses resources, such as standard
signal precedence and kstatus projection.

## Resource-specific checks own Ready semantics

The core honors kstatus-standard `Reconciling` and `Stalled` conditions.
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
