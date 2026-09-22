# Codex Continuation Handoff

Updated: 2026-09-22

## Read first

- `CLAUDE.md`
- `docs/BACKEND_IMPLEMENTATION_PLAN.md`
- `docs/PROGRESS.md`
- `docs/DECISIONS.md`
- `docs/ARCHITECTURE.md`
- `docs/ROADMAP.md`

Run Go validation from `D:\bot-shield\backend`, not the repo root.

Do not print, copy, or commit `backend/.env`; it contains local credentials.

## Current status

Phase 0 is closed in code and verified. The next real backend work is Phase 1:
live per-tenant policy/rule enforcement in shadow mode.

The dashboard rules/settings CRUD is real persistence, but it is not active
protection yet. Do not describe custom rules or protection settings as enforced
until Phase 1 wires them into the live decision path.

## Completed in the latest pass

### Host, SNI, and tenant lifecycle

Files:

- `backend/pkg/core/identity.go`
- `backend/pkg/core/guard.go`
- `backend/pkg/tenant/tenant.go`
- `backend/pkg/db/db.go`
- `backend/main.go`
- tests under `backend/pkg/core` and `backend/pkg/tenant`

Behavior:

- Malformed HTTP Host values are rejected before tenant lookup.
- TLS requests with non-empty SNI that differs from HTTP Host return 421.
- Tenant host mappings are canonicalized before storage and lookup.
- One canonical host cannot map to two tenants in the same process.
- DB tenant rows carry lifecycle `status`.
- `pending_verification` domains are readable by dashboard ID but do not route
  visitor traffic by Host.

### Challenge replay state

Files:

- `backend/pkg/challenge/challenge.go`
- `backend/pkg/challenge/challenge_test.go`
- `backend/main.go`

Behavior:

- Challenge tokens and passed cookies remain host-bound.
- Solved challenge nonces can be consumed through Redis, so replay across nodes
  is rejected while Redis is healthy.
- Redis failure degrades to bounded local replay protection instead of locking
  out real visitors.
- Operators still need the same `-challenge-secret` or
  `HAKAISHIELD_CHALLENGE_SECRET` on every node so issued tokens/cookies verify
  everywhere.

### Admin and tenant isolation coverage

Files:

- `backend/pkg/api/isolation_test.go`
- `backend/pkg/api/postgres_isolation_integration_test.go`
- `backend/pkg/api/handlers.go`
- `backend/pkg/api/domains.go`
- `backend/pkg/api/dashboard_extra.go`

Behavior and coverage:

- Deterministic tests prove one user cannot read another tenant's stats,
  evidence logs, or top offenders.
- Real Postgres integration test covers stats ownership, cross-owner rule toggle
  rejection, and settings owner scoping.
- The real Postgres test runs only when `HAKAISHIELD_TEST_DATABASE_URL` is set.
  For a remote dev DB, also set `HAKAISHIELD_ALLOW_REMOTE_TEST_DATABASE=1`.
- A live Supabase dev Postgres run passed on 2026-09-22 using the DB URL from
  `backend/.env`; the first sandboxed network attempt was blocked, then the
  approved remote-network retry passed.

### Observability counters

Files:

- `backend/pkg/observability/counters.go`
- `backend/pkg/observability/counters_test.go`
- `backend/pkg/auth/jwt.go`
- `backend/pkg/signals/goodbots.go`
- `backend/pkg/signals/redis_circuit.go`
- `backend/pkg/core/proxy.go`
- `backend/pkg/core/guard.go`
- `backend/pkg/core/identity.go`
- `backend/main.go`

Behavior:

- `/__hakaishield/observability` exposes aggregate counters only when
  `-observability-token` or `HAKAISHIELD_OBSERVABILITY_TOKEN` is configured.
- The endpoint is bearer-token protected and off by default.
- Counters cover JWKS refresh/failure, unknown-kid rejection, Goodbot DNS budget
  rejection, Redis circuit opens/probes/skips, origin proxy errors, invalid
  origin targets, malformed forwarded client IP headers, malformed Host, unknown
  Host, and SNI/Host mismatch.
- Counters contain no raw IPs, hosts, tokens, credentials, or visitor payloads.

## Verification already passed

From `D:\bot-shield\backend`:

```powershell
$env:GOCACHE='D:\bot-shield\.gocache'
go test ./...
go vet ./...
go build ./...
```

From `D:\bot-shield`:

```powershell
git diff --check
graphify update .
```

Real Supabase dev Postgres integration:

```powershell
go test ./pkg/api -run TestPostgresTenantIsolationIntegration -v
```

This passed after deliberately setting:

- `HAKAISHIELD_TEST_DATABASE_URL` from `backend/.env`
- `HAKAISHIELD_ALLOW_REMOTE_TEST_DATABASE=1`

`go test -race` was attempted but could not run on this machine because the Go
race detector requires cgo and `gcc` is not installed in `%PATH%`.

## Remaining work

### Next: Phase 1 policy and explainable decisions

Primary files:

- `backend/pkg/rules/rules.go`
- `backend/pkg/settings/settings.go`
- `backend/pkg/signals/score.go`
- `backend/pkg/core/guard.go`
- `backend/pkg/evidence/evidence.go`
- relevant API/dashboard files only if contracts need to stay accurate

Build:

- versioned immutable per-tenant policy model
- ordered safe rules with actions: allow, rate-limit, challenge, deceive, block
- validated safe conditions: normalized path, method, CIDR, verified agent,
  score band, fired signals, endpoint class, tenant allowlist
- endpoint classes such as login, API, browse, checkout, static asset
- immutable decision object containing facts, signals, score, matched rule,
  action, and enforcement status
- shadow preview before enforce
- tenant ownership checks on every policy/rule read and write
- auditability and rollback path
- evidence that exactly matches the real decision

Guardrails:

- Start in shadow mode.
- A User-Agent-only allow rule must never skip verification/scoring.
- Deception must require a stricter confidence threshold than block.
- Mutation-test each enforcement condition.
- Do not claim dashboard CRUD is active protection until live scoring reads it.

### Later backlog

- Phase 2: adaptive/progressive challenge, trust decay, signed telemetry, and
  challenge UX/recovery improvements. Redis nonce replay protection is already
  built, but Phase 2 still owns policy-driven challenge behavior.
- Phase 3: browser integrity, behavior, asset fidelity, request graph, session
  consistency, HTTP/2/HTTP/3 intelligence. Introduce in shadow mode first.
- Phase 4: maintained browser fingerprint registry, safe intelligence feed,
  verified agent policy, Web Bot Auth.
- Phase 5: durable hosted-product backend: domain ownership verification, ACME,
  metering, durable evidence, audit trail, roles/membership.
- Phase 6: production metrics/histograms, readiness, quotas, origin protection,
  soak/adversarial suite, release safety.

## Documentation state

Latest Phase 0 closeout was recorded in:

- `docs/BACKEND_IMPLEMENTATION_PLAN.md`
- `docs/PROGRESS.md`
- `docs/DECISIONS.md`
- `docs/ARCHITECTURE.md`
- `docs/ROADMAP.md`
- this handoff

Update relevant docs after the next meaningful change.

## Workspace cautions

- Preserve unrelated `graphify-out/` updates unless intentionally refreshing the
  graph.
- Preserve untracked `inspired/` and `.claude` settings.
- Do not reset the worktree or overwrite concurrent/user changes.
- `docs/CODEX_HANDOFF.md` is deleted in the current working tree; use this file
  for the next continuation unless the owner asks otherwise.
