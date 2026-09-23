# Phase 1 backend production review — 2026-09-23

## Verdict

The policy matcher and rule-creation validation are suitable as an **inert,
shadow-only foundation**. Phase 1 is **not production-ready as a customer
feature**: `main.go` does not attach a `PolicyProvider`, so stored rules do not
produce shadow opinions or change live traffic. Enforcement must remain off.

## Reviewed and fixed

- `pkg/rules` now validates the dashboard's fields, operators, actions, and
  safety constraints at Create. Bad rules return HTTP 400 instead of being
  stored or reported as a server error. DECEIVE must exceed both the owner's
  saved block threshold and `signals.HardBlockThreshold()`.
- The matcher rejects unknown actions, malformed CIDR operators, oversized
  condition lists and values, and an unvalidated oversized regex at runtime.
  UA-only BLOCK and PASS rules are rejected at creation.
- The guard's per-tenant shadow test now sends requests for two tenants and
  checks both evidence trails and unchanged origin forwarding.
- A real Postgres integration test covers rule insertion, owner-scoped listing,
  and DECEIVE rejection against the owner's configured threshold.

## Verification

- `go test ./...`, `go vet ./...`, `go build ./...`, and `git diff --check` passed
  from the backend/repository directories as appropriate.
- `TestPostgresTenantIsolationIntegration` passed against the configured dev
  Postgres in an isolated temporary schema, which the test removes afterward.
- The Go race detector was unavailable on this host (`CGO_ENABLED=0`; no GCC).
- An isolated mutation-test command was rejected by automatic command policy;
  mutation failure was not verified in this review.

## Remaining production gates

1. Bind rules to the validated tenant identity through `TenantConfig` and a
   bounded, cached provider. Existing rules are owner-scoped, not tenant-scoped.
   Test two owners and two domains through the actual DB-to-guard path.
2. Add explicit policy ordering, version/audit/rollback, and a preview contract
   before any rule can enforce. The current `Policy.Version` has no storage or
   rollback behavior.
3. Define behavior for verified bots, solved challenges, and honeypot requests:
   those guard branches return before policy shadow evaluation today.
4. Bound the number of rules loaded per tenant and compile regular expressions
   once when loading a policy. `regexp.Compile` currently runs on each matching
   request when a policy provider is attached.
5. Collect shadow agreement, false-positive, latency, and resource data on
   representative traffic. Run race/soak tests in an environment with cgo and
   a C compiler before enabling enforcement.

Passing unit tests here proves the isolated matcher and current guard hook;
it does not prove the missing DB-to-guard production wiring.
