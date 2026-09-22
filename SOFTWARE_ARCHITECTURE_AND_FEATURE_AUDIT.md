# hakaishield — Software Architecture & Feature Audit

**Date:** 2026-09-22
**Auditor hats:** Senior Staff Engineer · Software Architect · Backend · Frontend · QA · Security · Performance/Cloud-Cost
**Rule followed:** *"Kisi file/function ke hone ka matlab ye nahi ki feature implemented hai."* Har connection actual code se trace kiya gaya.
**Code changes:** ❌ Koi nahi. Ye pure read-only audit hai.

### Legend

| Symbol | Meaning |
|---|---|
| ✅ | Implemented & end-to-end connected |
| ⚠️ | Partially implemented / narrow |
| ❌ | Missing / broken |
| 🟡 | Code exists but unused |
| 🔴 | Exists but wrongly / incompletely connected |

### Verification jo actually chalayi (evidence base)

```text
backend/  go build ./...            → clean
backend/  go vet ./...              → clean
backend/  go test ./...             → 16 packages, all "ok"
dashboard/ npm run typecheck        → clean (tsc --noEmit)
dashboard/ npm run build            → success, 1427 modules, 539.78 kB JS (gzip 139.91 kB)
live:      backend :8080 up, redis connected, Supabase Postgres connected
live:      GET /                       (curl UA)        → 403  (correct)
live:      GET /  (Chrome UA+headers)                    → 200  (correct)
live:      GET /__hakaishield/healthz                    → 404  (BUG - see F-04)
live:      GET /__hakaishield/trap                       → 404  (BUG - see F-05)
live:      GET /api/v1/dashboard/stats (no token)         → 401  (correct)
live:      Supabase JWKS                                 → 200, ES256 P-256 key present
```

---

## 0. Executive Summary (sabse pehle ye padho)

**Kya bana hua hai:** Ek genuinely solid detection proxy. TLS/JA4 fingerprinting, 9-signal scoring engine, JS challenge, deception+honeypot, Redis-backed velocity/crawl-pattern, verified-good-bot DNS, evidence trail, shadows mode, aur ek React dashboard jisme Supabase auth + domains/rules/settings CRUD hai. Code quality high hai, comments honest hain, fail-open design consistent hai.

**Lekin feature-level reality docs se kam hai.** Audit ka core result:

1. **2 features production mein dead hain** (routing bug): honeypot trap aur healthz. — `main.go` ke `ServeMux` prefix conflict (`/__hakaishield/` sab kha jaata hai). Tests pass hote hain kyunki wo Guard ko direct call karte hain.
2. **Dashboard us content ko "protection" bata raha hai jo actually enforcement mein nahi hai:** mitigation rules (CRUD-only, koi scoring read nahi karta), protection settings (thresholds DB mein hain par `score.go` fixed constants use karta hai), challenge type "Turnstile CAPTCHA" (kabhi execute nahi hota), WAF toggles (state local, save hi nahi hote). Kuch jagah honest "not built" notice hai, par **rules aur settings pages pe "Active" badge dikhta hai jab wo effect-less hain** — ye sabse bada product-trust risk hai.
3. **Ek pura file (392 lines) dead hai:** `dashboard/src/data/mockData.ts` — 0 references. Usme fake JA4, fake IPs, fake SIEM, fake domains sab hain. Koi delete karna bhool gaya (good — warna fake data dikh raha hota).
4. **Detached connections:** `DashboardExportHandler` (CSV export) aur `DashboardTopOffendersHandler` (duplicate of wired handler) — implemented par koi route nahi. `DashboardEvidenceHandler` (token-gated) ka koi frontend consumer nahi (frontend JWT-based `/evidence-logs` use karta hai).
5. **Domain lifecycle adhoora:** har naya domain `pending_verification` pe set hota hai, kabhi `active` nahi hota. UI hamesha "pending" badge dikhata rehta hai. Verification flow hai hi nahi.
6. **Onboarding form ke 5 answers collect karke discard kar deta hai** — sirf `onboarding_complete` Supabase metadata mein jaata hai. `users` table (Postgres) ab dead schema hai.
7. **Selected-domain selection kai jagah ignore hoti hai:** domain switcher se domain #2 chuno, par Overview ka "Top offenders" aur "Evidence" still domain #1 (first-created) ka data dikhate hain — kyunki frontend `?tenant=`/`?domain=` bhejta hi nahi.
8. **Security P0s** (pichle audit se + yahan confirm): uncached DB lookup per unknown Host; SSRF via arbitrary origin; JA4 aggregate velocity cap unseeded hone pe real browsers ko challenge karega; domain ownership verification missing.

**Bottom line:** Detection core **chal raha hai**; uske aas-paas ka "governance/control pane" largely **cosmetic** hai. Hosted traffic se pehle P0 fix zaroori.

---

## 1. System Map — cheezein kaise juDi hui hain

```text
┌─────────────────────────────┐
│  Visitor (hostile until     │
│  scored)                    │
└──────────────┬──────────────┘
               │ DNS/CNAME → hakaishield
               ▼
┌──────────────────────────────────────────────────────────────┐
│  hakaishield binary (backend/)  :8080                         │
│                                                               │
│  http.Server ──► observability.Middleware ──► ServeMux        │
│       │                                          │            │
│       │                       ┌──────────────────┼──────────┐│
│       │                       ▼                  ▼          ▼│
│       │            /__hakaishield/          /api/v1/...   /  │
│       │            challenge.Handler()      api handlers  guard│
│       │            (challenge + verify)     (JWT-gated)      ││
│       │                                                      ││
│       │            Guard: JA4 → tenant → goodbot → honeypot  ││
│       │                   → challenge-passed → Evaluate()    ││
│       │                   → allow/challenge/block/deceive    ││
│       │                                                      ││
│       │            tenant.Store → origin ReverseProxy        ││
│       └──────────────────────────────────────────────────────┘│
└───────┬──────────────────┬───────────────────┬────────────────┘
        │                  │                   │
        ▼                  ▼                   ▼
   ┌─────────┐      ┌────────────┐     ┌──────────────┐
   │ Redis   │      │ Postgres   │     │ External     │
   │ :6379   │      │ (Supabase) │     │  • origin    │
   │ velocity│      │ tenants    │     │  • DNS PTR/A │
   │ crawl   │      │ rules      │     │  • Sentry    │
   │ ja4 db  │      │ settings   │     │  • Supabase  │
   │         │      │ users(dead)│     │    Auth/JWKS │
   └─────────┘      └────────────┘     └──────────────┘

┌──────────────────────────────────────────────────────────────┐
│  Dashboard (dashboard/)  :3000  — React + Vite SPA            │
│  Browser ──► Supabase Auth (signup/signin/reset, DIRECT)      │
│          └─► Go backend /api/v1/* (Bearer = Supabase JWT)     │
└──────────────────────────────────────────────────────────────┘
```

**Important architectural fact:** Dashboard aur proxy **same port :8080** pe hain (same binary). `/api/v1/*` exact patterns jeette hain, baaki sab `/` → Guard. Iska matlab API base URL aur proxy address ek hi hai.

**Auth boundary:** Passwords Supabase ke paas jaate hain, Go backend unhe kabhi nahi dekhta. Go sirf JWKS se session JWT verify karta hai (ES256/P-256). ✅ verified live.

---

## 2. Complete Feature Inventory

Har feature ka tree, aur status.

### F-01 · TLS/JA4 Fingerprinting ✅
```text
TLS/JA4 fingerprinting
├── Frontend UI          ❌ (dashboard mein JA4 sirf read-only data hai)
├── API request          ❌
├── Backend endpoint     ✅ captureListener → JA4FromContext (no explicit endpoint)
├── Service/function     ✅ core/capture.go (NewCaptureListener, ConnContext, JA4FromContext)
│                        ✅ signals/fingerprint.go (ParseJA4)
├── Database             ❌ (per-request derive, no store — deliberate)
├── Redis                ❌
├── External service     ❌
├── Background job       ❌
└── Tests                ✅ capture_test.go, fingerprint_test.go, signals_bench_test.go (14.3µs)
```
Note: HTTP/2 fingerprinting ❌ (capture listener sirf h1 offer karta hai). Multi-record ClientHello reassembly ❌ (`unreadable` banta hai).

### F-02 · Reverse proxying to origin ✅ / 🔴
```text
Origin proxying
├── Frontend UI          ❌
├── API request          ❌
├── Backend endpoint     ✅ core.NewOriginProxy (httputil.ReverseProxy, Rewrite path)
├── Backend service      ✅ core/proxy.go (spoofable header stripping, X-Real-IP, X-Forwarded-*)
├── Database             🔴 tenants.target (SSRF validation missing — see SEC-02)
├── Redis                ❌
├── External service     ✅ origin server; 🔴 ProxyFromEnvironment enabled
├── Background job       ❌
└── Tests                ✅ proxy_test.go, identity_test.go
```

### F-03 · Scoring engine (9 signals) ✅
```text
Scoring engine
├── Frontend UI          ⚠️ Overview/EvidenceLogs result dikhate hain, weights config nahi kar sakte
├── API request          ✅ GET /dashboard/stats, /evidence-logs, /top-offenders
├── Backend endpoint     ✅ indirectly via Guard
├── Backend service      ✅ signals/score.go (checks table, Evaluate, DecideWithPolicy)
│                        ✅ signals/useragent.go, headers.go, velocity.go, pattern.go, honeypot.go, ja4db.go
├── Database             🔴 protection_settings thresholds store hote hain par PADHE NAHI JAATE
├── Redis                ✅ velocity/crawl counters, ja4 blocklist sync
├── External service     ❌
├── Background job       ✅ ja4db.StartJA4Sync (30s Redis poll) — par Redis never seeded
└── Tests                ✅ score_test.go, velocity_test.go, pattern_test.go, headers_test.go, honeypot_test.go
```
Signals: `fragmented_handshake`(50), `ua_mismatch`(50), `header_anomaly`(25), `ja4_blocklist`(100), `scripting_tool`(100), `velocity_spike`(50), `ja4_velocity_spike`(50), `crawl_pattern`(50), `honeypot_trap`(50, **dead in prod**).

### F-04 · Health check endpoint ❌ BROKEN
```text
Health check
├── Frontend UI          ❌
├── API request          ❌ (LB/k8s ke liye tha)
├── Backend endpoint     ❌ REGISTERED NOWHERE — guard ka check mux se shadowed
│                        (main.go: "/__hakaishield/" + "/" ; Guard.healthz unreachable)
├── Service/function     🟡 core/guard.go pehla block (dead in production)
├── Database             ❌
├── Redis                ❌ (dependency health reflect nahi karta — even if reachable)
├── External service     ❌
├── Background job       ❌
└── Tests                ⚠️ TestGuardHealthzEndpoint PASS karta hai — par Guard direct call karke (mux bypass)
```
**Live confirmed:** `GET /__hakaishield/healthz` → **404**. Load balancer healthy node ko out kar dega.

### F-05 · Honeypot trap ❌ BROKEN
```text
Honeypot trap
├── Frontend UI          ❌
├── API request          ❌
├── Backend endpoint     ❌ Guard ka /__hakaishield/trap mux se shadowed → challenge handler → 404
├── Service/function     🟡 signals/honeypot.go (RecordHoneypotTrip kabhi call nahi hota prod mein)
│                        ✅ deception.go inject karta hai trap link (par link dead-end hai)
├── Database             ❌
├── Redis                ❌ (per-node in-memory)
├── External service     ❌
├── Background job       ❌
└── Tests                ⚠️ TestHoneypotTrapEndToEnd PASS — par Guard direct call
```
**Live confirmed:** `GET /__hakaishield/trap` → 404 par **koi recording nahi** → `honeypot_trap` signal kabhi fire nahi karega.
Extra: shadow mode mein bhi trap 404 deta hai (forward nahi karta) — "shadow forwards everything" promise ka exception.

### F-06 · Deception / decoy response ✅ / ⚠️
```text
Deception
├── Frontend UI          ⚠️ Overview "Deceived" counter dikhata hai
├── API request          ✅ (stats.deceived)
├── Backend endpoint     ✅ core/proxy.go deceiveResponse (ModifyResponse)
├── Backend service      ✅ deception.InjectPayload (512KiB cap, 200 only, uncompressed only)
├── Database             ⚠️ tenant.Deception flag (startup flag, not dashboard-configurable)
├── Redis                ❌
├── External service     ❌
├── Background job       ❌
└── Tests                ✅ deception_test.go, proxy_test.go (large-body, compressed, non-HTML)
```
Gap: deception ka koi separate false-positive metric nahi (roadmap item 11a risk).

### F-07 · JS challenge ✅
```text
JS challenge
├── Frontend UI          ❌ (challenge page server-rendered, dashboard se control nahi)
├── API request          ❌
├── Backend endpoint     ✅ GET /__hakaishield/challenge, POST /__hakaishield/verify
├── Service/function     ✅ challenge/challenge.go (HMAC token, PoW, canvas proof, automation probes)
├── Database             ❌ (used-nonce in-memory map)
├── Redis                ❌ (multi-node nonce sharing nahi — documented gap)
├── External service     ❌
├── Background job       ❌
└── Tests                ✅ challenge_test.go (13 tests: reuse, tamper, host, headless, automation)
```
⚠️ `protection_settings.challenge_type` me 'captcha' choice UI mein hai par challenge hamesha PoW — Turnstile kabhi nahi.

### F-08 · Rate limiting / velocity / crawl pattern ✅
```text
Velocity + crawl pattern
├── Frontend UI          ❌ (koi rate-limit config UI nahi; WAF card mein dead sliders hain)
├── API request          ❌
├── Backend service      ✅ signals/velocity.go (nav 20/s, asset 300/s, ja4 50/s)
│                        ✅ signals/pattern.go (HLL >60 distinct paths/min)
│                        ✅ signals/redis_circuit.go (fail-open breaker)
├── Database             ❌
├── Redis                ✅ INCR/EXPIRE/PFADD/PFCOUNT
├── External service     ❌
├── Background job       ❌
└── Tests                ✅ velocity_test.go, pattern_test.go, redis_circuit_test.go
```
🔴 Cross-tenant keys: `vel:ip:<ip>`, `crawl:ip:<ip>`, `vel:ja4:<ja4>` — tenant prefix nahi → tenant A ka traffic tenant B ko affect kar sakta hai.

### F-09 · Verified good-bot (SEO) ✅ / ⚠️
```text
Good bot verification
├── Frontend UI          ❌
├── Backend service      ✅ signals/goodbots.go (reverse+forward DNS, 6h cache, 64-slot semaphore)
├── Database             ❌
├── Redis                🔴 cache per-node in-memory (multi-node duplicate lookups)
├── External service     ✅ DNS resolver (request path pe!)
└── Tests                ✅ goodbots_test.go (spoof, lookalike domain, budget)
```
⚠️ Cache full hone pe eviction O(n) map scan write-lock ke andar (perf risk).

### F-10 · Enforce / Shadow mode ✅
```text
Mode
├── Frontend UI          ✅ Overview badge + banner (shadow → "Would block" labels)
├── API request          ✅ stats.mode / enforcing
├── Backend service      ✅ config/mode.go (ParseMode refuses unknown)
├── Tests                ✅ mode_test.go, guard_bench_test.go (shadow never blocks)
```

### F-11 · Evidence trail ✅ / ⚠️
```text
Evidence trail
├── Frontend UI          ✅ EvidenceLogs page (search, filter, sort, 100-row cap)
├── API request          ✅ GET /api/v1/dashboard/evidence-logs  (JWT)
│                        🟡 GET /api/v1/dashboard/evidence      (token) — NO frontend consumer
├── Backend service      ✅ evidence/evidence.go (1000-entry ring, 24h retention)
├── Database             ❌ (in-memory; resets on restart)
├── Redis                ❌
├── Tests                ✅ evidence_test.go, evidence_bench_test.go
```
⚠️ Evidence record mein **path aur IP nahi** hai → "is visitor ne kya maanga" correlate karne ka koi zariya nahi (docs acknowledged).

### F-12 · Dashboard stats ✅
```text
Stats
├── Frontend UI          ✅ Overview metric strip
├── API request          ✅ GET /api/v1/dashboard/stats?tenant=<id>
├── Backend endpoint     ✅ DashboardStatsHandler (RequireAuth + ownership check)
├── Backend service      ✅ stats/stats.go (atomic counters, in-memory)
├── Database             🔴 per-node in-memory → multi-node mein galat
└── Tests                ✅ stats_test.go, report_test.go, handlers_test.go
```
Note: README kehta hai stats "needs no token" — **ab galat hai**, code `RequireAuth` karta hai.

### F-13 · Top offenders ✅ / 🔴
```text
Top offenders
├── Frontend UI          ✅ Overview table
├── API request          ✅ GET /api/v1/dashboard/top-offenders  (⚠️ NO tenant parameter)
├── Backend endpoint     ✅ TopOffendersHandler (callerDomain = FIRST domain)
├── Backend service      ✅ trail aggregation, top 20
├── Database             ❌
└── Tests                ⚠️ report_test.go tests the UNWIRED duplicate handler
```
🔴 Selected domain ignore hota hai — domain switcher ka koi asar nahi.
🟡 `DashboardTopOffendersHandler` (report.go) duplicate + unwired.

### F-14 · Multi-tenancy ✅ / ⚠️
```text
Tenancy
├── Backend service      ✅ tenant/tenant.go (byHost + byID maps, lazy DB load)
├── Database             ✅ tenants table (id, host UNIQUE, target, mode, evidence_token, owner, status)
├── Redis                🔴 tenant scoping missing in rate keys
└── Tests                ✅ tenant_test.go (isolation, concurrent, wildcard)
```
🔴 Har unknown Host pe uncached DB query (SEC-01).
🔴 `status` field ignore hota hai — pending domain bhi serve ho jaata hai.
⚠️ Default tenant `"default"` / host `"*"` hardcoded in `main.go`.

### F-15 · Domain management ✅ / 🔴
```text
Domains
├── Frontend UI          ✅ DomainsSiem page (list + add form)
├── API request          ✅ GET /domains, POST /domains  (matches backend exactly)
├── Backend endpoint     ✅ DomainsHandler (RequireAuth, envelope)
├── Backend service      ✅ db.ListDomains / db.CreateDomain
├── Database             ✅ tenants table (host UNIQUE)
└── Tests                ✅ domains_test.go (normalizeOrigin only), tenant_test.go
```
🔴 status lifecycle dead (always pending_verification).
🔴 host format / IP validation nahi + SSRF origin (SEC-02).
⚠️ Added domain instantly serve nahi hoti (lazy load) — roadmap item 12.6.

### F-16 · Mitigation Rules CRUD 🔴 (cosmetic)
```text
Mitigation rules
├── Frontend UI          ✅ MitigationRules page (visual builder, toggle, managed list)
├── API request          ✅ GET /rules, POST /rules/custom, PUT /rules/{id}/toggle
├── Backend endpoint     ✅ RulesListHandler / CreateRuleHandler / ToggleRuleHandler
├── Backend service      ✅ rules/rules.go (CRUD + Managed() static list)
├── Database             ✅ mitigation_rules table
├── Redis                ❌
├── External service     ❌
└── Tests                ⚠️ rules_test.go sirf encode/decode + nil pool; SQL untested
```
🔴 **Rules ka live traffic pe koi asar nahi.** `pkg/signals/score.go` `mitigation_rules` read nahi karta. UI "Active" badge dikhata hai — misleading.
⚠️ Frontend offers actions `PASS/DECEIVE/LOG` aur fields `Geo/TLS Version` — backend koi enum validate nahi karta.

### F-17 · Protection Settings CRUD 🔴 (cosmetic)
```text
Protection settings
├── Frontend UI          ✅ ProtectionSettings page
├── API request          ✅ GET/PUT /settings/protection
├── Backend endpoint     ✅ ProtectionSettingsHandler
├── Backend service      ✅ settings/settings.go (Upsert validates block>challenge)
├── Database             ✅ protection_settings table
└── Tests                ⚠️ settings_test.go sirf validation; SQL untested
```
🔴 `score.go` fixed constants (100/50) use karta hai — saved thresholds kabhi apply nahi hote.
⚠️ Frontend extra local state: `tarpitDelay`, `sqliProtection`, `xssProtection`, `rateLimitEnabled`, `rateLimitRpm` — **save hi nahi hote**.
⚠️ `challengeType: 'captcha'` save hota hai par execute kabhi nahi hota.

### F-18 · Auth (Supabase) ✅
```text
Auth
├── Frontend UI          ✅ SignIn/SignUp/ForgotPassword/Onboarding + RequireAuth guard
├── Frontend lib         ✅ lib/supabaseClient.ts, supabase.auth.* (DIRECT, Go bypass)
├── Backend endpoint     ✅ RequireAuth middleware on every /api/v1 route
├── Backend service      ✅ auth/jwt.go (JWKS ES256, unknown-kid negative cache)
├── Database             ❌ (Supabase apna; `users` table dead)
├── External service     ✅ Supabase Auth + JWKS
└── Tests                ✅ jwt_test.go (rotation, alg none, expiry, unknown kid), middleware_test.go
```
⚠️ `iss`/`aud` validation missing (SEC-03).
🟡 SignIn ka "Remember me" checkbox — no state, no handler, kuch nahi karta.
⚠️ Backend sirf ES256 accept karta hai; JWKS confirmed ES256 ✅ (live verified).

### F-19 · Onboarding ⚠️
```text
Onboarding
├── Frontend UI          ✅ 3-step wizard (useCase, visitors, website, teamSize, botProblem)
├── API request          ✅ supabase.auth.updateUser({data:{onboarding_complete:true}})
├── Backend endpoint     ❌
├── Database             🔴 answers DISCARDED — koi table nahi
└── Tests                ❌
```
⚠️ 5 sawaal poochhe jaate hain, 0 persist hote hain. `users` table ke `full_name/company/onboarding_complete` columns dead.

### F-20 · Billing / Subscription / Payment ❌ (honestly stubbed)
```text
Billing
├── Frontend UI          ✅ Subscription page (plans), Payment page (honest "not set up")
├── API request          ❌
├── Backend endpoint     ❌ (Stripe endpoints nahi hain)
├── Database             ❌ (no subscription/invoice/usage table)
└── Tests                ❌
```
✅ Positive: pichhle fake card form + fake invoice data hata diya gaya — ab honest notice hai.

### F-21 · SIEM export ❌
```text
SIEM
├── Frontend UI          ⚠️ placeholder ("not built yet")
├── Backend endpoint     ❌
└── Database             ❌
```
🟡 `mockData.siemIntegrations` mein fake connected Datadog/Splunk tha — dead file mein.

### F-22 · WAF ❌
```text
WAF
├── Frontend UI          ⚠️ "Coming Soon", toggles local state only
├── Backend              ❌ (no SQLi/XSS detection)
└── Tests                ❌
```

### F-23 · Marketing pages ✅
```text
Marketing
├── Landing, Pricing, About, Contact, Docs, Changelog, Terms, Privacy
└── Status: ✅ static, no API. (No backend dependency.)
```
⚠️ Plan data `Subscription.tsx` aur `Pricing.tsx` dono mein hardcoded (duplication).

### F-24 · Observability (Sentry) ⚠️
```text
Observability
├── Backend              ✅ observability/sentry.go (panic recover + report)
├── Config               ✅ SENTRY_DSN env only (flag nahi — good)
├── Tests                ✅ sentry_test.go
└── Gap                  ❌ Prometheus metrics / latency-per-layer nahi (roadmap item 15)
```

---

## 3. Frontend ↔ Backend Contract Matrix

### 3.1 Har frontend API call ka verification

| Frontend call | Method | Path | Backend route | Auth | Schema match | Status |
|---|---|---|---|---|---|---|
| `listDomains()` | GET | `/domains` | `DomainsHandler` | ✅ JWT | ✅ `{id,domain,origin,name,status}` ↔ `domainJSON` | ✅ |
| `addDomain(d,o)` | POST | `/domains` | `DomainsHandler` | ✅ JWT | ✅ body `{domain,origin}` ↔ `addDomainRequest` | ✅ |
| `listRules()` | GET | `/rules` | `RulesListHandler` | ✅ JWT | ✅ `{managedRules,customRules,exceptions}` | ✅ |
| `createRule(...)` | POST | `/rules/custom` | `CreateRuleHandler` | ✅ JWT | ✅ `{name,conditions,action}` | ✅ |
| `toggleRule(id,e)` | PUT | `/rules/{id}/toggle` | `ToggleRuleHandler` | ✅ JWT | ✅ `{enabled}` | ✅ |
| `getProtectionSettings()` | GET | `/settings/protection` | `ProtectionSettingsHandler` | ✅ JWT | ✅ | ✅ |
| `updateProtectionSettings(p)` | PUT | `/settings/protection` | `ProtectionSettingsHandler` | ✅ JWT | ✅ | ✅ |
| `getStats(tenantId)` | GET | `/dashboard/stats?tenant=` | `DashboardStatsHandler` | ✅ JWT | ⚠️ raw JSON (no envelope) — intentional, matches backend | ✅ |
| `getTopOffenders()` | GET | `/dashboard/top-offenders` | `TopOffendersHandler` | ✅ JWT | ✅ | 🔴 tenant param missing |
| `getEvidenceLogs()` | GET | `/dashboard/evidence-logs` | `EvidenceLogsHandler` | ✅ JWT | ✅ | 🔴 tenant param missing |
| supabase signUp/signIn/signOut/reset/updateUser | — | Supabase direct | — | — | ✅ | ✅ |

✅ **Koi frontend call nonexistent backend endpoint pe nahi ja raha.** Paths/methods/bodies sab exact match karte hain. Ye ek positive finding hai.

### 3.2 Backend endpoints jinka koi frontend consumer nahi

| Endpoint | Status | Note |
|---|---|---|
| `GET /api/v1/dashboard/evidence` | 🟡 unused by frontend | Operator token-gated; frontend JWT-based `/evidence-logs` use karta hai. Do endpoints same data ke liye — different trust boundary (documented). |
| `DashboardExportHandler` (CSV) | 🟡 **not even routed** | Implemented, zero route, zero frontend. Double-dead. |
| `DashboardTopOffendersHandler` | 🟡 **not routed** | Duplicate of wired `TopOffendersHandler`; sirf tests use karte hain. |
| healthz | ❌ shadowed | (F-04) |
| honeypot trap | ❌ shadowed | (F-05) |

### 3.3 Schema mismatches / subtle bugs

| # | Issue | Detail |
|---|---|---|
| M1 | `getTopOffenders()` tenant-less | `callerDomain()` first-created domain deta hai. Domain switcher ka koi asar nahi. |
| M2 | `getEvidenceLogs()` tenant-less | Same as M1. |
| M3 | `statsResponse.enforcing` unused by UI | UI `mode` use karta hai; `enforcing` dead field. |
| M4 | `Domain.status` never leaves pending | UI hamesha "pending" badge. |
| M5 | Settings PUT sends only 4 of 9 UI fields | Tarpit/WAF fields silently dropped. |
| M6 | `challengeType: 'captcha'` accepted, never executed | UI "Selected" par kuch nahi hota. |
| M7 | `getStats` error path | Raw fetch; `res.ok` check hai ✅ par envelope error message nahi. Acceptable. |

---

## 4. Database Audit

| Table | Purpose | Read by | Written by | Status |
|---|---|---|---|---|
| `tenants` | domain routing + config | `tenant.Store`, `db.GetTenant/ByID`, `db.ListDomains` | `db.CreateDomain`, manual | ✅ (status lifecycle 🔴) |
| `mitigation_rules` | custom rules | `rules.Store.List` (dashboard only) | `rules.Store.Create/SetEnabled` | 🔴 not enforced |
| `protection_settings` | thresholds | `settings.Store.Get` (dashboard only) | `settings.Store.Upsert` | 🔴 not enforced |
| `users` | legacy accounts | ❌ nobody | ❌ nobody | 🟡 **DEAD** (Supabase auth ke baad) |

* **Indexes:** `tenants.host` UNIQUE, `tenants.owner_user_id`, `mitigation_rules.owner_user_id`, `protection_settings` PK. ✅ Koi missing index nahi mila.
* **N+1:** ✅ kahin nahi.
* **Migrations:** ❌ `pkg/db.initSchema` startup pe raw DDL (`ALTER TABLE ... IF NOT EXISTS`) — versioned migrations nahi, multi-node race possible.
* **Pool:** ⚠️ `pgxpool.New` defaults; explicit MaxConns nahi.
* **Over-fetching:** ✅ `ListDomains` sirf zaroori columns.
* **Missing constraints:** ⚠️ `tenants.status` pe CHECK nahi; `mitigation_rules.action` pe CHECK nahi; `conditions_json` TEXT (JSONB nahi — queryable nahi).

---

## 5. Redis Audit

| Key pattern | Purpose | Written by | TTL | Tenant-scoped |
|---|---|---|---|---|
| `vel:ip:<ip>:nav:<window>` | nav rate | INCR | 2s | 🔴 NO |
| `vel:ip:<ip>:asset:<window>` | asset rate | INCR | 2s | 🔴 NO |
| `vel:ja4:<ja4>:<window>` | cross-IP ja4 rate | INCR | 2s | 🔴 NO |
| `crawl:ip:<ip>:<window>` | distinct-path HLL | PFADD/PFCOUNT | 120s | 🔴 NO |
| `ja4:scrapers` (hash) | scraper blocklist | ❌ **kabhi likha nahi jaata** | — | n/a |
| `ja4:browsers` (set) | browser exemption prefixes | ❌ **kabhi likha nahi jaata** | — | n/a |
| `__hakaishield/healthz` | — | — | — | — |

* 🔴 **Do keys jo sirf padhe jaate hain, likhe kabhi nahi** → `ja4_blocklist` effectively sirf 1 hardcoded entry, aur `isCommonBrowserJA4` always false → JA4 velocity cap real browsers pe lagta hai (**P0**).
* 🔴 No tenant prefix → cross-tenant coupling.
* ✅ Circuit breaker (`redis_circuit.go`) bounded fail-open — good.

---

## 6. External Services

| Service | Used for | Where | Cost/risk |
|---|---|---|---|
| Origin server | proxying | `core/proxy.go` | ⚠️ SSRF via configurable target |
| DNS (PTR + A) | good-bot verification | `signals/goodbots.go` | ⚠️ request-path, 2s timeout, 64 slots — amplification vector |
| Supabase Auth | signup/signin/reset | frontend direct | ✅ |
| Supabase JWKS | JWT verification | `auth/jwt.go` | ✅ cached 1h, refresh serialized |
| Supabase Postgres | config store | `pkg/db` | ⚠️ per-unknown-host query |
| Sentry | crash reporting | `observability` | ✅ optional |
| Stripe | payments | ❌ not integrated | — |

---

## 7. Background Jobs / Queues

| Job | File | Interval | Status |
|---|---|---|---|
| JA4 DB sync from Redis | `signals/ja4db.go` | 30s | ✅ runs, 🔴 data source empty |
| Honeypot expiry sweep | `signals/honeypot.go` | lazy, ≥1min | ✅ (but trap unreachable) |
| Good-bot cache expiry | `signals/goodbots.go` | lazy on cap | ⚠️ O(n) scan |
| Challenge nonce expiry | `challenge/challenge.go` | per verify call | ⚠️ O(n) scan under mutex |

**Queues:** ❌ none (no async job system). Sab request-path synchronous.

---

## 8. Broken / Wrongly-Connected (🔴) — consolidated

| ID | What | Where | Impact | Sev |
|---|---|---|---|---|
| B1 | Honeypot trap route shadowed | `main.go` mux vs `guard.go` | Signal never fires | **P0** |
| B2 | Healthz route shadowed | same | LB health always 404 | **P0** |
| B3 | JA4 browser exemption never seeded | `ja4db.go` + `velocity.go` | Real browsers challenged at scale | **P0** |
| B4 | Uncached DB lookup per unknown Host | `tenant.go` | DB DoS / bill | **P0** |
| B5 | No SSRF validation on origin | `domains.go` / `proxy.go` | Internal network read | **P0** |
| B6 | Domain `status` ignored | `tenant.addFromDBRow` | Unverified hosts served | **P1** |
| B7 | Custom rules not enforced | `rules` ↔ `score.go` | UI lies "Active" | **P1** |
| B8 | Protection settings not enforced | `settings` ↔ `score.go` | UI lies thresholds | **P1** |
| B9 | challengeType captcha no-op | `challenge.go` | UI lies | **P2** |
| B10 | WAF toggles not saved | `ProtectionSettings.tsx` | Local state only | **P2** |
| B11 | Onboarding answers discarded | `Onboarding.tsx` | Data loss | **P2** |
| B12 | Selected-domain ignored in 2 calls | `Overview.tsx`, `EvidenceLogs.tsx` | Wrong data shown | **P1** |
| B13 | Cross-tenant Redis keys | `velocity.go`, `pattern.go` | Isolation violation | **P1** |
| B14 | README stale (stats auth) | `README.md` | Doc lie | **P2** |
| B15 | Env var name mismatch | `main.go` vs docs | Cluster secret misconfig | **P2** |

---

## 9. Duplicate / Redundant / Dead Code

| Item | File | Lines (approx) | Status |
|---|---|---|---|
| **`mockData.ts`** | `dashboard/src/data/mockData.ts` | ~392 | 🟡 **100% DEAD** (0 refs) — fake JA4/IPs/SIEM/domains |
| `DashboardTopOffendersHandler` | `backend/pkg/api/report.go` | ~95 | 🟡 unwired duplicate |
| `DashboardExportHandler` (CSV) | `backend/pkg/api/report.go` | ~40 | 🟡 unwired |
| `requestClientIP` | `backend/pkg/core/identity.go` | ~4 | 🟡 test-only |
| `users` table | `backend/pkg/db/db.go` | ~10 | 🟡 dead schema |
| Offender aggregation logic | `dashboard_extra.go` + `report.go` | duplicated | 🟡 duplication |
| Plan data | `Subscription.tsx` + `Pricing.tsx` | duplicated | ⚠️ |
| `statsResponse.enforcing` | `handlers.go` | 1 field | ⚠️ unused by UI |
| `IsScriptingTool` etc. | used ✅ | — | — |

**Note:** `mockData.ts` deleting safe hai — zero imports, build pass. Best to delete.

---

## 10. Security Findings

| ID | Finding | File | Attack path | Sev |
|---|---|---|---|---|
| SEC-01 | Uncached DB query per unknown Host header | `tenant.go` | Random `Host:` → per-request Postgres query | **P0** |
| SEC-02 | SSRF via arbitrary origin | `domains.go`, `proxy.go` | Auth user origin=`169.254.169.254` → metadata fetch | **P0** |
| SEC-03 | JWT `iss`/`aud` not validated | `auth/jwt.go` | Any project-signed token accepted | **P2** |
| SEC-04 | Secrets via CLI flags (`-evidence-token`, `-challenge-secret`) | `main.go` | `ps aux` exposure | **P2** |
| SEC-05 | Domain ownership verification missing | `db.CreateDomain` + `tenant` | Add unowned host, serve it | **P1** |
| SEC-06 | Wildcard CORS on authenticated API | `api/middleware.go` | `*` origin with Authorization | **P3** |
| SEC-07 | Challenge verify endpoint unthrottled | `challenge.go` | Unauth POST spam + O(n) lock scan | **P2** |
| SEC-08 | Verified-bot path skips velocity | `guard.go` | Verified bot = unlimited rate (by design) | **P3** |
| SEC-09 | `ProxyFromEnvironment` on origin transport | `proxy.go` | Unexpected proxy routing | **P3** |
| SEC-10 | Spoofed "Googlebot" UA scores 0 (allowed) | `useragent.go`, `goodbots.go` | crawler marker exemption | **P2** |
| SEC-11 | Honest `curl` UA = score 100 block | `useragent.go` | Uptime monitors/API clients blocked | **P2** |

---

## 11. Performance & Cloud Cost

| ID | Finding | Impact @1M / 10M / 100M | Sev |
|---|---|---|---|
| C-01 | Unknown-Host DB query | 50k / 500k / 5M queries | **P0** |
| C-02 | 3–4 Redis RTT/request (velocity×2 + crawl) | ~250M+ Redis ops @100M | P2 |
| C-03 | `crawl_pattern` HLL write per page view | ~300M Redis ops @100M | P2 |
| C-04 | Good-bot DNS on request path | QPS spike + 2s latency | P2 |
| C-05 | `botCache` O(n) eviction under write lock | serialization at 100k entries | **P1** |
| C-06 | `challenge.consume` O(n) scan under mutex | CPU + contention | **P1** |
| C-07 | `MaxConnsPerHost` unset | unbounded origin conns | P2 |
| C-08 | No `ReadTimeout`/`WriteTimeout` on server | slow-loris hold | P2 |
| C-09 | Per-node in-memory state prevents multi-node | scale-out blocked | P1 |
| C-10 | Frontend bundle 539 KB (no code splitting) | slow first load | P3 |
| C-11 | Sentry 2s flush in panic path | panic storm block | P3 |

---

## 12. QA / Test Gaps

**Strong:** 173 test/benchmark functions, 16 packages green, concurrency/`-race`, forgery/malformed cases, mutation checks documented.

**Gaps:**

1. **Integration tests `main.go` mux bypass karte hain** → B1/B2 chhup gaye. Add: real mux build karke healthz=200 aur trap recording assert karo.
2. **Empty browser-prefix list untested** → B3.
3. **`pkg/db` — `[no test files]`** → SQL regressions untested.
4. **`rules`/`settings` SQL paths untested** (sirf validation).
5. **Dashboard frontend mein 0 tests** (`package.json` mein test script nahi). 19 pages.
6. **No load/soak test** — 1M/10M/100M claims evidence-less.
7. **Shadow-mode honeypot behavior untested** (trap shadow mein bhi 404).
8. **Tenant isolation tests DB-less** — real SQL cross-tenant leak untested.

---

## 13. Architecture & File Structure Critique

**Acha:**
- `signals` (pure detection) / `core` (orchestration) / `tenant` (isolation) / `api` (dashboard) boundaries saaf hain.
- `ProxyFactory` callback se import cycle avoid — smart.
- Fail-open consistent, bounded state everywhere, standard-library-first.
- Comments honest aur useful.

**Bura / risk:**
- **A1 (P2):** `docs/ARCHITECTURE.md` stale paths (`proxy/*.go`, `cmd/hakaishield`) — actual `pkg/core`, `main.go`.
- **A2 (P2):** README stats-auth claim galat.
- **A3 (P2):** `BOTSHIELD_CHALLENGE_SECRET` vs `HAKAISHIELD_CHALLENGE_SECRET` mismatch.
- **A4 (P1):** Per-node state (Stats/Trail/honeypot/botCache/nonce/secret) multi-node design ka blocker.
- **A5 (P1):** "CRUD that doesn't enforce" — architecture mein UI aur enforcement ka contract toota hua hai. Rules/settings ko ya wire karo ya UI mein "inactive" mark karo.
- **A6 (P3):** DDL code ke andar; migrations ka ghar nahi.
- **A7 (P3):** `signals` 15+ files; request-path external deps (Redis/DNS) ek group mein ho sakte hain for testability.

---

## 14. Prioritized Action Plan

### P0 — Turant
1. B1/B2 route shadowing fix + real-mux integration test.
2. B3 JA4 exemption fail-open when prefix list empty + startup warning + test.
3. B4 unknown-Host negative cache (bounded TTL).
4. B5 origin SSRF validation (private/loopback/link-local block + dial-time recheck).

### P1 — Production/high traffic se pehle
5. B6 domain ownership verification + status gate.
6. B12 selected-domain param for top-offenders/evidence-logs.
7. B13 tenant-scope Redis keys.
8. C05 botCache eviction; C06 challenge nonce sweep.
9. B7/B8 wire rules+settings into scoring, OR mark UI inactive.
10. Health check Redis/DB aware.

### P2 — Jaldi
11. SEC-03 JWT aud/iss; SEC-07 verify throttle; SEC-04 env-first secrets.
12. C02/C03 batch Redis; C04 offload/negative-cache DNS.
13. B9/B10/B11 honest UI (challengeType, WAF, onboarding persistence).
14. B14/B15/A1 doc+env-var consistency.
15. Versioned migrations; pgxpool explicit limits.
16. Frontend tests + `npm audit` CVEs.

### P3 — Nice to have
17. Delete `mockData.ts`, `report.go` unwired handlers, `users` table, `requestClientIP` (owner confirm).
18. `go mod tidy`; `rateLimitMs` → const; error style consistent; CORS allowlist.
19. Code-split frontend bundle; Prometheus metrics; periodic DDL → migrations.

---

## 15. "Ye software exactly kya karta hai" — 6 lines mein

> hakaishield ek inline reverse proxy hai jo customer ka TLS terminate karta hai, har connection ka JA4 fingerprint nikalta hai, aur request ko 9 signals pe score karke **allow / challenge / block / deceive** decide karta hai — phir origin ko forward karta hai ya rok deta hai, aur har decision ka evidence record rakhta hai.
> Redis distributed rate/pattern counters deta hai; Postgres customer config (domains/rules/settings) rakhta hai; Supabase auth + JWKS dashboard sessions verify karta hai.
> Dashboard ek React SPA hai jo auth Supabase se karta hai aur config/live stats evidence JWT-signed API se leta hai.
> **Detection chain end-to-end live hai. Control pane (rules/settings/WAF/SIEM/billing) largely cosmetic hai. Do features (honeypot, healthz) routing bug se dead hain.**

---

## 16. Verification Appendix (honest disclosure)

**Chalaya aur pass:**
```text
go build ./... / go vet ./... / go test ./...        → clean / clean / 16 ok
npm run typecheck                                     → clean
npm run build                                         → success
live backend smoke (proxy allow/block, stats 401)     → correct
live healthz/trap                                     → 404 (bug confirmed)
live Supabase JWKS                                    → 200 ES256 key present
```

**Nahi chala / assume kiya (speculation nahi kiya):**
- Real DB/Redis ke against load test (env constraint).
- Multi-node behavior.
- Dashboard browser mein manual click-through (sirf typecheck/build + code trace).
- End-to-end sign-in (test user create karna production Supabase mein — avoid kiya). Auth correctness JWKS + code se infer ki.
- `npm audit` output reproduce nahi kiya (docs mein 2 moderate CVEs noted hain).
