# Codex Session Handoff

Updated: 2026-09-22

This file records the current Codex session so the next session can continue
without redoing the repository audit or guessing which changes are safe.

## Scope and Source of Truth

- User asked to implement the best, production-grade backend features from the
  five reference repositories, following `CLAUDE.md`.
- Scope was narrowed explicitly to `backend/` Go code. The frontend was not
  used as the implementation target.
- Reference repositories are research inputs only. No reference code was
  copied.
- The implementation plan is in `docs/BACKEND_IMPLEMENTATION_PLAN.md`.
- Current GitHub/main changed during this session. The latest observed commit
  is `a19618c` (`feat: migrate dashboard auth from custom bcrypt+JWT to Supabase Auth`).
  Do not reset, merge, or overwrite the user's concurrent changes.

## Backend Audit Findings

The backend already has TLS/JA4 capture, reverse proxying, scoring, Redis
velocity/crawl checks, challenges, deception, honeypot evidence, tenant state,
dashboard stats/evidence, and recently added Supabase-authenticated domains,
rules, and settings APIs.

Important existing gaps identified:

- Scoring and evidence used to execute stateful Redis-backed checks twice for a
  normal request.
- Verified good-bot DNS matching used a plain suffix check, allowing lookalike
  names such as `evilgooglebot.com` to match `googlebot.com`.
- Good-bot cache keys included arbitrary complete User-Agent strings and the
  cache had no hard size bound.
- Guard parsed `RemoteAddr` and `Host` by splitting on a colon, breaking IPv6
  addresses and canonical tenant lookup.
- Challenge token/cookie state was signed but not host-bound or single-use.
- Redis-backed rate and crawl evidence now has a shared, bounded fail-open
  circuit. It is intentionally per-process; add observability and tune the
  recovery cadence from production data before a large multi-node rollout.
- Client identity now supports an explicit trusted-proxy CIDR boundary. Direct
  peers still ignore forwarded headers; a configured CDN/LB chain uses a
  validated `X-Forwarded-For` chain.
- New dashboard custom rules/settings are currently persistence/API work; they
  are not yet wired into live scoring/enforcement.

## Changes Made in This Session

### 1. Single scoring evaluation

Files:

- `backend/pkg/signals/score.go`
- `backend/pkg/signals/score_test.go`
- `backend/pkg/core/guard.go`

Added `signals.Evaluation` and `signals.Evaluate`. Guard now evaluates the
request once and uses the same score and fired signals for the decision and
evidence record. Existing `Score` and `Analyze` wrappers remain for callers
that only need one result.

Added a Redis-backed regression test proving one evaluation increments the IP
and JA4 velocity counters once each.

### 2. Verified good-bot DNS hardening

Files:

- `backend/pkg/signals/goodbots.go`
- `backend/pkg/signals/goodbots_test.go`

Added exact-or-dot-boundary hostname matching, so lookalike domains do not pass
reverse/forward DNS verification. Good-bot cache keys now use the bot family
instead of the arbitrary full User-Agent, and the cache has a hard cap with
expiry cleanup/oldest-entry eviction.

Added tests for lookalike rejection and domain-boundary behaviour.

### 3. Canonical request identity

Files:

- `backend/pkg/core/identity.go`
- `backend/pkg/core/identity_test.go`
- `backend/pkg/core/guard.go`

Added standard-library `net.SplitHostPort`/`net.ParseIP` based helpers for
direct peer IP and Host canonicalization. Guard now uses them instead of
colon-splitting. Forwarded headers remain intentionally untrusted until an
explicit trusted-proxy configuration exists.

Added IPv4, IPv6, invalid-input, hostname case, port, and IPv6-literal tests.

### 4. Host-bound, single-use challenge state

Files:

- `backend/pkg/challenge/challenge.go`
- `backend/pkg/challenge/challenge_test.go`

Challenge tokens now include the canonical host under the HMAC. Verification
rejects a token on a different host. Passed cookies contain a URL-safe encoded
timestamp/host payload and are also host-bound. Valid challenge nonces are
single-use in a bounded in-memory map with expiry and oldest-entry eviction.

The host-only binding deliberately does not bind cookies to IP/JA4, because
mobile networks and NAT would create unacceptable false positives. Velocity
checks still protect a solved session from unlimited request volume.

Added tests for cross-host token rejection, single-use replay rejection, and
cross-host cookie rejection.

### 5. Bounded Redis outage circuit

Files:

- `backend/pkg/signals/redis_circuit.go`
- `backend/pkg/signals/redis_circuit_test.go`
- `backend/pkg/signals/velocity.go`
- `backend/pkg/signals/pattern.go`
- `backend/main.go`

Velocity, JA4-velocity, and crawl-pattern checks now share a one-second,
in-process fail-open circuit. The first Redis pipeline error opens the circuit;
subsequent checks skip Redis completely until the cooldown expires; exactly one
request is allowed to probe recovery. A successful probe closes the circuit and
a failed probe reopens it.

The `go-redis` client is configured with command retries disabled
(`MaxRetries = -1`) and one dial attempt, preventing client retries from
amplifying the first request during an outage. Rate evidence can therefore be
temporarily absent, but an optional detector dependency cannot add repeated
timeout latency or block customer traffic.

Added circuit tests for cooldown skipping, one-probe recovery, success reset,
and failed-probe cooldown extension. Mutation check: removing the in-flight
probe guard caused `TestRedisCircuitSkipsRequestsUntilSingleRecoveryProbe` to
fail; the guard was restored.

### 6. Trusted-proxy CIDR client identity and challenge mutation review

Files:

- `backend/pkg/core/identity.go`
- `backend/pkg/core/identity_test.go`
- `backend/pkg/core/guard.go`
- `backend/main.go`

Added `core.ClientIPResolver` and the `-trusted-proxy-cidrs` comma-separated
flag. With no configured CIDR, the direct TCP peer remains the only client IP.
When the direct peer is explicitly trusted, the resolver validates all
`X-Forwarded-For` values, walks from the closest hop backwards, skips trusted
proxy hops, and selects the first untrusted address. Invalid/missing headers
fall back to the direct peer; invalid CIDR configuration fails startup. IPv4
and IPv6 paths are covered.

The challenge host-token, nonce replay, and passed-cookie host tests were
mutation reviewed: removing each protected condition independently caused its
cross-host/replay test to fail, then the condition was restored.

## Verification Status

Passed before the latest challenge edit:

- focused `pkg/signals` and `pkg/core` tests;
- full `go test ./...`;
- `go vet ./...`;
- `go build ./...`.

The challenge change initially exposed a real issue: raw `|` in a cookie value
was sanitized by HTTP cookie serialization, invalidating every passed session.
It was corrected by base64url-encoding the cookie payload.

After that correction, the pending commands were run from `D:\bot-shield\backend`:

```powershell
gofmt -w pkg/challenge/challenge.go pkg/challenge/challenge_test.go pkg/core/guard_test.go
go test ./pkg/challenge ./pkg/core ./pkg/signals
go test ./...
go vet ./...
go build ./...
```

Result: focused challenge/core/signals tests, full `go test ./...`,
`go vet ./...`, and `go build ./...` all pass.

After the Redis circuit change, the following also passed from
`D:\bot-shield\backend` (with a workspace-local Go build cache because the
default user cache is restricted in this environment):

```powershell
go test ./pkg/signals
go test ./...
go vet ./...
go build ./...
gofmt -d main.go pkg/signals/redis_circuit.go pkg/signals/redis_circuit_test.go pkg/signals/velocity.go pkg/signals/pattern.go
git diff --check
```

After the trusted-proxy change, these passed from `D:\bot-shield\backend`:

```powershell
go test ./...
go vet ./...
go build ./...
gofmt -d main.go pkg/core/identity.go pkg/core/identity_test.go pkg/core/guard.go
git diff --check
```

Local `go test -race ./...` previously could not run because CGO was disabled;
the configured CI race job remains the authoritative environment. Local
`golangci-lint` was not installed in the earlier environment.

## Immediate Next Steps

1. Add observability for circuit opens/recovery probes and tune its one-second
   cooldown from real Redis and traffic data before a large multi-node rollout.
2. Wire the authenticated custom rules/settings model into live tenant policy
   only after validating fields, actions, tenant ownership, cache invalidation,
   and shadow rollout. Persistence alone must not be advertised as enforcement.
3. Add tenant/host binding to the runtime challenge context where needed and
   design the distributed Redis nonce store before claiming multi-node replay
   protection.
4. Add tests for API tenant isolation, rule ownership, invalid settings,
   CORS/auth exposure, and origin validation.
5. Update `docs/PROGRESS.md`, `docs/DECISIONS.md`, and `docs/ROADMAP.md` only
   after verification reflects the actual implementation.

## Do Not Touch Without User Direction

- `backend/main.go`, `docs/PROGRESS.md`, `backend/main_test.go`, and
  `backend/.env.example` currently have user/concurrent changes.
- `graphify-out/` has watcher-generated modifications; do not revert them.
- `inspired/` is the user's untracked reference-repository directory.
- Do not reset branches, delete untracked files, or overwrite the frontend.
