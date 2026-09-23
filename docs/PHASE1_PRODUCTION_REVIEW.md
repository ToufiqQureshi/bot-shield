# Phase 1 backend production review — 2026-09-23

## Status

The backend policy implementation is coded and exercised in shadow. It is
**not approved for customer traffic enforcement** yet: representative traffic
has not been measured, and the activation summary/gate uses node-local
bounded aggregates rather than durable data across replicas. The API
defaults every newly saved or rolled-back revision to shadow. A tenant must
explicitly activate a measured revision before policy changes any visitor
response. Legacy account-wide rules never enforce.

## Implemented and reviewed

- Versioned per-tenant policy snapshots with explicit rule order, immutable
  history, owner isolation, optimistic version checks, and shadow rollback.
- Conditions on canonical path/method, CIDR, score, verified search/monitor
  status, signal and endpoint class, plus tenant CIDR allowlist.
- First-match action semantics for PASS, RATE_LIMIT, CHALLENGE, DECEIVE, BLOCK,
  and LOG. A rate-limit rule must depend on a velocity signal. DECEIVE needs a
  score floor above the live hard-block threshold. High-risk path/UA-only PASS
  cannot override a hard block without verified bot/allowlist evidence.
- Baseline, proposed and effective decisions and matched-rule/version facts in
  a deep-copied evidence trail. Per-version shadow aggregates survive ring
  wraps and support the authenticated disagreement summary and rollout gate.
- Bounded asynchronous policy refresh (32 concurrent loads, 4096 tenant and
  owner cache entries). Legacy DB reads cap policy materialization at 200 rows;
  regexes compile at load rather than on the request path.
- Dedicated honeypot and solved-challenge paths record an explicit policy
  skip reason and do not count toward activation. DNS-verified search bots can
  enter policy evaluation while keeping their baseline allow decision.

## Verification performed

- Unit tests cover path/class matching, invalid conditions, regex
  precompilation, document validation, Guard action/evidence outcomes, hard
  block PASS guardrail, activation timing, and evidence immutability.
- A real Postgres integration test against an isolated temporary schema ran
  the actual startup migration and verified owner isolation, append-only
  versioning, optimistic conflicts including concurrent writes, proxy loads,
  history, and rollback to shadow.
- The 200-rule compiled regex worst-case benchmark measured about 37 µs per
  evaluation and zero allocations on this Windows development machine.
- The exact staged Phase 1 snapshot was applied in a separate temporary
  worktree, without concurrent Phase 2 files: `go test ./...`, `go vet ./...`,
  and `go build ./...` all passed. The temporary worktree was removed.
- A deliberate overlay mutation removing the hard-block PASS guardrail made
  `TestTenantPolicyDoesNotAllowHighRiskPathOnlyRule` fail (200 instead of 403).
- The shared working tree's `go test ./...` still fails in the concurrent,
  uncommitted Phase 2 challenge edit at `TestChallengeRealFlowPasses`.

## Remaining production gates

1. Review at least 30 minutes of representative real traffic, including
   legitimate users, verified agents, high-volume API clients, and adversarial
   traffic. Investigate proposed block/deceive disagreements and false
   positives before any activation.
2. Replace or supplement the node-local shadow gate with durable aggregate
   metrics before a multi-replica deployment. A restart or API call to an
   idle node currently refuses activation, which is safe but operationally
   inconvenient.
3. Re-run full Go tests after the concurrent challenge edit stabilizes, then
   race/soak tests on a cgo-equipped host. Production readiness cannot be
   claimed while the shared working tree's full suite is failing.
4. The existing dashboard rules page still edits legacy account-wide rules.
   Wire a tenant policy editor to the new API before calling Phase 1 a
   self-service customer feature. The backend API contract is documented in
   `docs/PHASE1_POLICY.md`.

No production traffic was switched to policy enforcement by this change.
