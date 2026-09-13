# Contributing KubeHealth checks

## Implementation language

A KubeHealth check can be implemented in Go or Lua. Both forms must return the
same reconciliation, availability, and lifecycle dimensions, and both are
registered for an exact Group, Version, and Kind.

- Use Go for integrations compiled and maintained with the library.
- Use Lua for checks loaded from consumer configuration or changed without
  recompiling the consumer.

Lua checks use `lua.Register`. They receive the resource as
the global `obj` table and must return a KubeHealth assessment table. The same
requirements and test expectations in this guide apply to both languages.

## Directory layout

Add third-party resources under the package for their owning project:

```text
integrations/<project>/
  register.go
  <resource>.go
  <resource>_test.go
```

Use a stable Go package name without punctuation. For example,
`cert-manager.io` uses `integrations/certmanager`.

Do not add resource checks directly to the root `kubehealth` package.
Kubernetes-native checks belong in `builtins`. Ecosystem CRDs belong in their
own `integrations/<project>` package. The root is reserved for the assessor and
standard-signal framework, while `api` contains the stable model.

Generic implementation helpers belong under `internal`. For example, the
common unstructured condition parser lives in `internal/conditions`. A helper
should not live in `builtins` merely because a built-in check was its first
consumer.

## Registration

Every integration package exports:

```go
func Register(assessor *kubehealth.Assessor) error
```

Register each supported Group, Version, and Kind explicitly:

```go
assessor.Register(
	schema.GroupVersionKind{
		Group:   "example.io",
		Version: "v1",
		Kind:    "Example",
	},
	example,
)
```

Do not register only by Group and Kind. API versions can have different schemas
and semantics.

## Check requirements

A contributed check must:

1. Return reconciliation, availability, and lifecycle.
2. Use `Unknown` when the object does not provide enough evidence.
3. Use `NotApplicable` only when a dimension is not meaningful.
4. Keep availability independent from reconciliation failure.
5. Normalize useful condition details into dimension-specific reasons and
   messages rather than returning the original conditions.
6. Avoid network or cluster access in the object-only check.
7. Include tests for reconciled, progressing, failed, and ambiguous states that the
   resource can represent.
8. Cite the upstream API documentation or implementation that defines the
   semantics.

An upstream operator can avoid a library-maintained resource check by adopting
the versioned `status.kubeHealth` contract documented in
[`README.md`](README.md#publish-health-from-an-operator). A complete, valid,
current report is authoritative and skips both generic evaluation and registered
checks. Do not add a check merely to translate an operator-published KubeHealth
report.

## Tests are mandatory and colocated

Every integration contribution must add tests next to the resource check:

```text
integrations/certmanager/certificate.go
integrations/certmanager/certificate_test.go
integrations/certmanager/issuer.go
integrations/certmanager/issuer_test.go
integrations/kyverno/policy.go
integrations/kyverno/policy_test.go
integrations/kyverno/clusterpolicy_test.go
integrations/keda/scaledobject.go
integrations/keda/scaledobject_test.go
```

Tests should create complete unstructured resources or load YAML fixtures and
call an Assessor after registering the integration. Do not test only a private
helper function. This verifies the exact GVK registration as well as the check.

For each resource, cover every meaningful outcome it can represent:

- Current and available.
- In progress.
- Terminal reconciliation failure.
- Available despite failed or in-progress reconciliation, when possible.
- Unavailable despite the same reconciliation state, when possible.
- Missing or ambiguous status fields.
- Any project-specific conditions that affect health.

If one implementation supports multiple kinds or API versions, include at
least one resource-level test file for every kind and one registration test for
every exact GVK. Shared logic does not remove the need to prove that each GVK is
wired correctly.

Built-in tests follow the same rule. For example:

```text
builtins/deployment.go
builtins/deployment_test.go
```

The built-in StatefulSet demonstrates the Lua form:

```text
builtins/statefulset.go
builtins/statefulset_test.go
builtins/lua/statefulset.lua
```

The small Go file embeds the script and adapts it to `api.Check`; the resource
semantics themselves remain in Lua.

Framework behavior tests remain in the root package because they test Assessor
merging, standard signals, and compatibility APIs rather than one resource.

The central `catalog/catalog_test.go` provides an additional smoke test that
catalog integrations are registered. It does not replace the detailed tests in
the owning integration package.

## Catalog

Add the integration's `Register` function to `catalog/catalog.go` only when it
is intended to be part of the convenience catalog. Consumers can always import
an integration package directly to keep their dependency surface smaller.

## Ownership

The preferred maintainer is someone familiar with the upstream project. Health
semantics are part of the API contract and should evolve alongside upstream
status changes. A future catalog should record upstream project, supported
versions, maintainers, and provenance for each integration.
