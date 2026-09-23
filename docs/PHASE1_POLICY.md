# Phase 1 tenant policy API and rollout

## Policy model

`tenant_policy_revisions` stores immutable, ordered JSON snapshots per domain
(`tenants.id`). Every write checks the authenticated owner, locks the tenant
row, compares `expectedVersion`, and appends a new version. A rollback copies
an older document into a new **shadow** revision; history is never rewritten.
Legacy `/api/v1/rules` rows are account-wide and remain shadow-only. A tenant
revision takes precedence. A domain with no revision keeps the existing signal
scorer as its only enforcer.

Rules are evaluated top to bottom. The first enabled match wins; no match
preserves the signal scorer's decision. Conditions within one rule are ANDed.
Supported actions are `PASS`, `RATE_LIMIT`, `CHALLENGE`, `DECEIVE`, `BLOCK`, and
`LOG`. `RATE_LIMIT` requires a `velocity_spike` or `ja4_velocity_spike` signal
condition and returns HTTP 429 with `Retry-After: 60` only when it fires.
`DECEIVE` requires a score floor strictly above the live hard-block threshold;
the proxy forwards with its established deception decision header. A path or
User-Agent `PASS` cannot override a hard-block score unless the visitor is a
DNS-verified search bot or matches the tenant's CIDR allowlist. Scoring always
runs before a policy decision. The existing honeypot trap and passed-challenge
paths retain their dedicated behavior and appear in evidence as skipped policy
evaluations; they do not count toward activation.

Conditions support `JA4 Fingerprint`, `Threat Score`, `IP Address`, `User-Agent`,
`Request Path`, `Request Method`, `IP Range`, `Verified Agent`, `Signal`,
`Request Class`, and `Tenant Allowlist`. `Verified Agent` is `search` only after
reverse/forward DNS verification, or `monitor` when the resolved client IP is
inside the tenant allowlist. Classes are `login`, `api`, `browse`, `checkout`,
and `static_asset`; the classifier uses fixed normalized-path defaults.
Malformed/ambiguous paths skip policy matching. Unsupported fields and operator
combinations are rejected. The provider compiles regular expressions once on
load; both rule count (200) and condition count (16) are bounded.

## Authenticated endpoints

All routes below require the existing Supabase bearer session and return the
standard `{success,data|error}` envelope. `{id}` is the domain/tenant ID from
`GET /api/v1/domains`, never a visitor-supplied owner ID.

| Route | Purpose |
|---|---|
| `GET /api/v1/domains/{id}/policy` | Current revision. |
| `PUT /api/v1/domains/{id}/policy` | Append a shadow revision; body has `expectedVersion` and `document`. Use 0 for the first version. |
| `GET /api/v1/domains/{id}/policy/history` | Latest 100 immutable revision summaries (version, actor, time, mode). |
| `GET /api/v1/domains/{id}/policy/history/{version}` | Read one complete historical revision before rollback. |
| `POST /api/v1/domains/{id}/policy/rollback` | Body: `expectedVersion`, `targetVersion`; creates a new shadow revision. |
| `POST /api/v1/domains/{id}/policy/preview` | Hypothetical `facts` for the saved revision, with no visitor action. Path/method/class and CIDR allowlist are normalized server-side; a supplied `search` verification is only a simulation assumption. |
| `GET /api/v1/domains/{id}/policy/shadow` | Bounded local summary: matches, proposed disagreements, actions, skipped requests, readiness. |
| `POST /api/v1/domains/{id}/policy/activate` | Body: `expectedVersion`; appends an enforce revision after the shadow gate. |

Example `PUT` body:

```json
{
  "expectedVersion": 0,
  "document": {
    "mode": "shadow",
    "allowlist": ["192.0.2.0/24"],
    "challengeTheme": "branded",
    "blockMessage": "Access denied by site policy",
    "rules": [
      {
        "id": "login-velocity",
        "name": "Limit fast login traffic",
        "enabled": true,
        "action": "RATE_LIMIT",
        "conditions": [
          {"field": "Request Class", "operator": "EQUALS", "value": "login"},
          {"field": "Signal", "operator": "EQUALS", "value": "velocity_spike"}
        ]
      }
    ]
  }
}
```

## Rollout and limits

The API accepts edits only in shadow mode. Activation requires at least 100
unskipped evaluations of the current revision, with the oldest at least 30
minutes old and the newest no more than five minutes old. The operator should
inspect `/shadow` and the evidence trail before activating. Shadow records
the baseline and proposed outcomes; after activation evidence also records the
effective decision and whether policy changed it. A tenant in global shadow
mode still forwards traffic regardless of the activated policy.

The gate uses bounded in-memory per-revision aggregates on the node handling
the API call. These keep the first observation even when the detailed 1000
entry evidence trail wraps. They reset on restart and may be unavailable on
a node that has not served the domain. The strict local gate fails closed for
activation; it does not aggregate across replicas. A multi-node rollout needs
durable aggregate shadow telemetry before production activation at scale. No
live customer traffic measurement has been performed for this change.

Policy refresh is asynchronous on a bounded worker pool (32 concurrent
loads). Cache misses use the existing scorer until the revision is ready;
Postgres never runs synchronously in the policy request path. Tenant revisions
refresh within 30 seconds, and failed/empty lookups are cached for 10 seconds.
The in-memory cache is bounded to 4096 tenant and 4096 legacy owner entries.
