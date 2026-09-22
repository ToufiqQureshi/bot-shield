# hakaishield — Production-Grade Full Software Audit

**Date:** 2026-09-22
**Auditor hat:** Senior Staff Engineer + Security + QA + Performance + Cloud Cost
**Scope:** `backend/` (Go proxy, ~69 source files), `dashboard/` (React/Vite), docs, config
**Method:** code padha, request path trace kiya, `go build` + `go vet` + `go test ./...` chalaya, aur har finding ko actual code line se verify kiya.
**Verification result:** `go build ./...` clean, `go vet ./...` clean, `go test ./...` **sab pass** (16 packages). Lekin — niche jo P0 bugs mile hain, unme se do aise hain jinke tests pass hote hain par production binary galat behave karti hai. Details neeche.

> Ye report **read-only audit** hai. Instruction ke mutabik maine koi code change nahi kiya — sirf ye ek naya report file banaya hai. Fixes "Recommended fix" section mein diye hain.

---

## 1. Executive Summary (2 minute ka TL;DR)

Product ka core idea solid hai aur code quality overall **good-to-very-good** hai — comment discipline, bounded state, fail-open design, standard-library-first approach sab genuinely acche hain. Ye "tutorial project" nahi hai.

Lekin production scale pe jaane se pehle **4 serious cheezein** block karti hain:

1. **Honeypot signal aur healthz endpoint production mein dead hain (P0).** `main.go` ke `ServeMux` route prefix conflict ki wajah se `/__hakaishield/trap` aur `/__hakaishield/healthz` dono challenge handler ke paas jaate hain, Guard tak pahunchte hi nahi. Tests pass hote hain kyunki tests Guard ko direct call karte hain, mux ko bypass karke. Matlab: honeypot kabhi fire nahi hoga, aur load balancer ka health probe 404 dega.
2. **JA4 aggregate velocity check real browsers ko challenge karega (P0).** `isCommonBrowserJA4()` in production **kabhi true nahi hota** kyunki `ja4:browsers` Redis set kabhi seed nahi kiya jaata. Iska matlab har `maxJA4Requests = 50/s` global cap — ek hi Chrome build ke saare users uspe aa jaate hain. Thoda traffic aate hi legit Chrome users challenge pe challenge karenge.
3. **Uncached Postgres lookup har unknown Host header pe (P0/P1).** Koi bhi `Host:` header badal ke request bhej sakta hai, aur har request ek DB query trigger karti hai. 100M requests pe 100M queries — ye direct cloud bill aur DB DoS hai.
4. **SSRF + domain ownership verification missing (P0 security).** Dashboard user jo bhi origin URL/host:port de sakta hai — including `169.254.169.254` (cloud metadata), `127.0.0.1`, internal 10.x/192.168.x. Proxy usko fetch karke response wapas de dega. Hosted multi-tenant SaaS ke liye ye serious hai.

Uske baad P1/P2 level pe: cross-tenant rate-limit key collision, botCache ka O(n) eviction request path pe, challenge nonce store ka O(n) scan mutex ke andar, JWT `iss`/`aud` validation missing, CLI flags se secret leak, aur per-request 3–4 Redis round trips.

**Production status: READY FOR CURRENT SCOPE — lekin public/hosted traffic se pehle P0 block fix karna zaroori hai.**

---

## 2. Critical Issues (P0)

### 2.1 Honeypot trap aur healthz production mein unreachable hain (ROUTE SHADOWING)

* **File:** `backend/main.go` (approx. line 213–215), `backend/pkg/core/guard.go` (ServeHTTP ka pehla block + honeypot branch), `backend/pkg/challenge/challenge.go` (`Handler()`)
* **Problem:** `mux.Handle("/__hakaishield/", challengeHandler.Handler())` aur `mux.Handle("/", guard)`. Go ka `http.ServeMux` longest-prefix match karta hai. `/__hakaishield/` subtree pattern hai, isliye:
  * `/__hakaishield/trap` → challenge handler ke inner mux pe jaata hai → inner mux mein sirf `/__hakaishield/challenge` aur `/__hakaishield/verify` registered hain → **404**, Guard ka `RecordHoneypotTrip` kabhi call nahi hota.
  * `/__hakaishield/healthz` → wahi challenge handler → **404**. Guard ka healthz check (`r.URL.Path == "/__hakaishield/healthz"`) tak request pahunchti hi nahi.
* **Why it matters:** Honeypot ek core detection layer hai (weight 50, docs mein "BUILT" likha hai). Production mein wo signal silently zero coverage hai. Docs `docs/SIGNAL_COVERAGE.md` aur `ROADMAP.md` claim karte hain ye chalu hai — ye **doc vs reality mismatch** hai. Healthz 404 dene ka matlab LB/k8s liveness probe fail karega aur healthy node ko bhi out-of-rotation kar dega.
* **Evidence:**
  * `guard_test.go` → `TestHoneypotTrapEndToEnd` aur `TestGuardHealthzEndpoint` Guard ko **direct** call karte hain (`guard.ServeHTTP(rec, req)`), `main.go` ke mux ko nahi. Isliye test green hai par binary broken.
  * `challenge.Handler()` ka inner mux mein trap/healthz register nahi hain.
* **Severity:** **P0**
* **Production impact:** Detection gap (honeypot = dead) + reliability gap (health probe false-negative). Attacker ke liye free pass; operator ko false healthy signal.
* **Recommended fix:** Ya to honeypot/healthz ko explicitly `mux.HandleFunc("/__hakaishield/trap", ...)` aur `mux.HandleFunc("/__hakaishield/healthz", ...)` pe register karo (Guard se pehle, longer/exact pattern), ya `/__hakaishield/` prefix route hatao aur sirf exact challenge/verify paths register karo. **Aur ek integration test likho jo real `main.go` ka mux build karke request bheje** — direct handler call se ye class of bug pakda nahi jaata.
* **Expected benefit:** Honeypot detection actually live; health probe sach bolne lagega.

### 2.2 JA4 aggregate velocity cap real browsers ko challenge karega

* **File:** `backend/pkg/signals/velocity.go` (`checkJA4VelocitySpike`), `backend/pkg/signals/ja4db.go` (`isCommonBrowserJA4`, `browserPrefixes`)
* **Problem:** `maxJA4Requests = 50` per 1-second window ka cap **har non-exempt JA4** pe lagta hai. Exemption `isCommonBrowserJA4()` se aata hai, jo `browserPrefixes` slice pe based hai. Wo slice sirf `AddCommonBrowserPrefix()` ya Redis `ja4:browsers` set se bharti hai. Repo mein **kahin bhi** (test ke alawa) `AddCommonBrowserPrefix` call nahi hota, aur `ja4:browsers`/`ja4:scrapers` Redis keys **kahin bhi likhe nahi jaate** (grep-confirmed).
* **Why it matters:** Ek hi Chrome/Firefox build ka JA4 fingerprint duniya bhar ke laakhon users share karte hain. Uspe 50 req/sec global cap — matlab ~50 req/s ke baad har us-build ka user `ja4_velocity_spike` (+50) score karega → balanced policy mein **DecisionChallenge**. Ye fresh deployment pe hi visible ho jaayega.
* **Evidence:** `isCommonBrowserJA4` early-return `for _, p := range browserPrefixes` — slice empty hone pe loop hi nahi chalta, `false` return. `StartJA4Sync` sirf Redis se **read** karta hai, seed koi nahi karta.
* **Severity:** **P0** (false-positive catastrophe at even modest scale)
* **Production impact (approx):**
  * 1M req/day (~12 req/s peak nahi, but bursty): intermittent mass challenges ho sakte hain ek popular browser build pe.
  * 10M req/day: ek browser build ka JA4 gauranteed cap cross karega → us build ke **saare legitimate users** challenge loop mein.
  * 100M req/day: product apne hi primary customers ko block kar dega.
* **Recommended fix:** Do mein se ek — (a) seed `ja4:browsers` via a real maintained pipeline, ya (b) **default ko fail-safe banao**: agar `browserPrefixes` empty hai to aggregate JA4 cap **disable** karo (fail open), ya cap ko per-(JA4,tenant) aur sirf non-crawler-UAs pe rakho. Plus ek startup warning agar list empty hai. Aur ek mutation test jo empty prefix list pe cap off hone ki guarantee de.
* **Expected benefit:** Real browsers pe mass false-positive khatam; signal sirf tab chale jab usko safely chalane ka data maujood ho.

### 2.3 Unknown Host header → har request pe uncached Postgres query

* **File:** `backend/pkg/tenant/tenant.go` (`Store.GetByHost` → `fetchFromDB` → `db.GetTenant`)
* **Problem:** `GetByHost` pehle memory map dekhta hai; miss hone pe, agar `ProxyFactory` set hai, turant `fetchFromDB(host)` call karta hai — 2s timeout wali **Postgres query, per request**. Negative cache ya "unknown host" memoization **nahi hai**.
* **Why it matters:** Attacker (ya literally koi bhi) random `Host:` headers bhej ke per-request DB load generate kar sakta hai. Ye honeypot se pehle hota hai, kyunki route shadowing ke baad bhi normal path chal raha hai. Ye classic "unbounded work per request" hai — CLAUDE.md Section 15/18 ka direct violation.
* **Evidence:** `fetchFromDB` memory check ke baad bina cache ke call hota hai; `GetByHost` har miss pe repeat karega. `SIGNAL_COVERAGE.md` Section 3 ne ise 2026-09-18 review mein note kiya tha — **fix nahi hua**.
* **Severity:** **P0** (cost + DoS)
* **Production impact:**
  * 1M req: agar 5% bogus hosts → ~50k DB queries; pool (pgx default ~CPU-based) pk sakta hai.
  * 10M req: ~500k queries, latency spike + Supabase compute bill.
  * 100M req: ~5M queries, DB saturation — ek attacker ek node ke saare tenants ko latka sakta hai.
* **Recommended fix:** Negative cache (bounded TTL, e.g. 30–60s) for unknown hosts in `Store`, plus LRU cap. Optionally route unknown-host pe 421 turant de do without DB (only known/verified hosts load from DB). Test: same bogus host 100 baar → DB only once.
* **Expected benefit:** DB load attacker-controlled se nahi, config-change controlled ho jaata hai.

### 2.4 SSRF + origin validation missing (dashboard → proxy)

* **File:** `backend/pkg/api/domains.go` (`normalizeOrigin`), `backend/pkg/core/proxy.go` (`NewOriginProxy`)
* **Problem:** `normalizeOrigin` koi bhi host:port/URL accept karta hai. `CreateDomain` usko DB mein daalta hai. `tenant.Store.addFromDBRow` us target se real reverse proxy bana deta hai. Koi blocklist nahi for loopback/link-local/private/metadata ranges.
* **Attack path (realistic):** Signed-up (free/authenticated) user → `POST /api/v1/domains` with `origin = "169.254.169.254:80"` (ya `127.0.0.1:5432`, `10.0.0.5:6379`, `http://[::1]:8080`) → apna domain add karta hai → us host pe ek request bhejta hai → hakaishield us URL ko server-side fetch karke response user ko de deta hai. Cloud metadata (IAM tokens), internal admin panels, Redis/Postgres ports — sab reachable.
* **Why it matters:** Hosted multi-tenant SaaS mein ye classic full-read SSRF hai. `CLAUDE.md` Section 16/17/18 ka direct violation.
* **Evidence:** `NewOriginProxy(u)` bas `u.Scheme`/`u.Host` non-empty check karta hai; koi IP-range policy nahi. Transport mein `Proxy: http.ProxyFromEnvironment` bhi hai.
* **Severity:** **P0** (security)
* **Production impact:** Credential/metadata exfiltration, internal network scan, origin abuse through us.
* **Recommended fix:** Origin add karte waqt validate karo — reject private/loopback/link-local/multicast/broadcast ranges (IPv4+IPv6), reject non-http(s) schemes, optionally resolve + re-check at dial time (`Control` hook in `net.Dialer`) to defeat DNS rebinding. Domain ownership verification (next item) ke bina origin add hi na ho.
* **Expected benefit:** SSRF closed; internal infra hakaishield ke through reachable nahi.

---

## 3. Security Findings

| # | Finding | File / Function | Severity | Attack path + Fix |
|---|---|---|---|---|
| S1 | **SSRF via arbitrary origin** (upar 2.4) | `api/domains.go`, `core/proxy.go` | **P0** | Authenticated user internal IP daal ke server-side fetch karwa sakta hai. Fix: IP-range blocklist + dial-time re-check. |
| S2 | **Domain ownership verification missing + `status` ignore** | `db.CreateDomain` (status `pending_verification`), `tenant.addFromDBRow` | **P1** | `store.GetByHost` DB row milte hi serve karta hai, `status` dekhta hi nahi. Koi bhi apna host add karke us host pe traffic serve kar sakta hai bina verify kiye. Roadmap item 21 isi ko gate kehta hai. Fix: `status='active'` wale hi load karo; DNS TXT/CNAME verification flow. |
| S3 | **JWT `iss`/`aud` validation missing** | `pkg/auth/jwt.go` → `Verify` | **P2** | Sirf signature + expiry + `sub` check hai. Usi project key se signed koi bhi non-session JWT accept ho sakta hai. Fix: expected `aud` (`authenticated`) aur `iss` validate karo via `jwt.WithAudience(...)`, `jwt.WithIssuer(...)`. |
| S4 | **Secrets CLI flags se process list mein visible** | `main.go` (`-evidence-token`, `-challenge-secret`) | **P2** | `ps aux` pe token dikhta hai; code khud SENTRY_DSN ke liye ye reasoning likhta hai par evidence/challenge secret ko flag se leta hai. Fix: env-first, flags deprecate; ya sirf env. |
| S5 | **Wildcard CORS authenticated endpoints pe** | `api/middleware.go` `RequireAuth` | **P3** | `Access-Control-Allow-Origin: *` with `Authorization` allowed. Bearer-token auth hone ki wajah se directly exploitable nahi (cookies nahi), par unnecessary. Fix: configured frontend origin allowlist. |
| S6 | **Honeypot trap path unauth 404 spoofable — but recording dead** | guard.go | **P0** (2.1) | — |
| S7 | **Challenge verify endpoint pe rate limit nahi** | `challenge.handleVerify` | **P2** | Unauth POST spam → har call SHA-256 + map scan. Body 64KB capped hai, par call-rate capped nahi. Fix: per-IP throttle. |
| S8 | **No `iss`/`kid` confusion guard beyond kid map** | `auth/jwt.go` | **P3** | Kid negative-cache acche se likha hai (bounded, TTL) — ye actually **good** hai. Sirf S3 ka complement. |
| S9 | **`Host` header pe hi tenant identity** | `core/identity.go` `requestHost` | **P3** | Host spoof se tenant A ko Tenant B ke stats dekhna possible nahi (host routing key hai), par Host-based tenant identity ko ownership verification (S2) ke saath hi safe mana ja sakta hai. |
| S10 | **`curl`/wget/uptime-monitor UA = score 100 block, par spoofed Googlebot = score 0 allow** | `signals/useragent.go` `scriptingMarkers`, `crawlerMarkers` | **P2** | Asymmetry: honest scripting tools hard-blocked ho jaate hain, par "Googlebot" likh dene wala (non-verified) crawler exemption se bach jaata hai (`claimsBrowser=false`). Business/FP risk. Fix: verified-bot ko allow, unverified crawler-claim ko scoring mein le aao. |

---

## 4. Cloud Cost / Waste Findings

| # | Finding | File | Impact @1M / 10M / 100M | Severity |
|---|---|---|---|---|
| C1 | **Uncached Postgres query per unknown Host** | `tenant.GetByHost` | 50k / 500k / 5M avoidable queries | **P0** |
| C2 | **3–4 Redis round trips per request, bina batching** | `velocity.go` (2 pipelines), `pattern.go` (1 pipeline: PFADD+EXPIRE+PFCOUNT) | Har request pe ~2–3 RTT. 100M req → ~250M+ Redis ops. Poore site ke page views pe HLL **write** hota hai | **P2** |
| C3 | **`crawl_pattern` har browser navigation pe HLL write** | `signals/pattern.go` | ~1 PFADD + 1 PFCOUNT + 1 EXPIRE **per page view**. 100M pageviews → 300M Redis ops sirf is signal ke liye | **P2** |
| C4 | **Good-bot DNS verification request path pe (2 lookups, 2s timeout)** | `signals/goodbots.go` `verifyDNS` | Spoofed "Googlebot" + rotating IPs → external DNS QPS spike + latency; har lookup 2s tak goroutine slot hold | **P2** |
| C5 | **`DefaultOriginTransport` mein `MaxConnsPerHost` unset** | `core/proxy.go` | Slow origin pe unbounded per-host connections → origin abuse + memory | **P2** |
| C6 | **Sentry `Flush(2s)` panic path pe per-panic** | `observability/sentry.go` | Panic storm mein har goroutine 2s block | **P3** |
| C7 | **Duplicate external calls nahi hain, but `ListDomains` har dashboard hit pe** | `api/handlers.go`, `dashboard_extra.go` | Dashboard traffic chhoti hai, par per-hit full list query; cacheable | **P3** |
| C8 | **`ja4db` background poll har 30s per node** | `signals/ja4db.go` | HGetAll + SMembers cheap hai; 3 nodes → 3x. Acceptable, par note-worthy | **P3** |

**Cost ka sabse bada multiplier C1 hai** — ye attacker-controlled hai. C2/C3 fixed, traffic-proportional hai (linear, not multiplicative), isliye P2.

---

## 5. Database Problems

| # | Finding | Detail | Severity |
|---|---|---|---|
| D1 | **Per-request uncached lookup** (same as C1) | `tenant.GetByHost` → `db.GetTenant` per unknown host | **P0** |
| D2 | **`users` table dead schema** | `password_hash VARCHAR(255) NOT NULL` — auth Supabase pe move ho gaya; table+hash approach abandon ho chuka hai par schema abhi bana hua hai (`initSchema` in `pkg/db/db.go`). Dead + misleading | **P3** |
| D3 | **Schema management code mein, versioned migrations nahi** | `initSchema` startup pe DDL chalata hai (`ALTER TABLE ... ADD COLUMN IF NOT EXISTS`). Ek node ka DDL doosre ke saath race kar sakta hai; rollback nahi | **P2** |
| D4 | **`CreateDomain` mein host validation nahi** | Koi host format/IP check nahi; `status` bhi `pending_verification` set karta hai par reader ignore karta hai (S2) | **P1** |
| D5 | **Indexes** | `tenants.host` UNIQUE (indexed), `owner_user_id` indexed, `mitigation_rules.owner_user_id` indexed, `protection_settings` PK. **Koi missing index nahi mila** — specific queries ke liye sahi indexes hain. Ye ek positive finding hai | ✅ |
| D6 | **N+1 / query-in-loop** | Kahin nahi mila. `ListDomains` single query; handlers loop mein query nahi karte | ✅ |
| D7 | **Connection pool explicit config nahi** | `pgxpool.New` default max conns (≈max(4, CPU)). Hosted multi-tenant pe explicit `MaxConns` + health set karna better | **P3** |
| D8 | **Rules/settings stored but never enforced** | `mitigation_rules` aur `protection_settings` real CRUD hain par `pkg/signals/score.go` inhe read nahi karta. User ko dashboard "protection on" dikhata hai jo asli nahi — product-trust issue (docs mein acknowledged) | **P1 (product)** |

---

## 6. Performance & Scalability

**10K / 100K / 1M / 10M / 100M request lens:**

* **10K–100K:** Sab theek. JA4 parse ~14µs, `Evaluate` ~0.9µs-ish (benchmarks claim). Koi visible issue nahi — **except** agar `ja4:browsers` empty hai to pehla popular browser build already ~50 rps pe cap maar dega (2.2).
* **1M:** Host-header DB amplification (C1) pool saturation; JA4 false-positive (2.2) visible.
* **10M:** JA4 mass-challenge guaranteed; Redis RTTs (C2/C3) significant; DNS amplification (C4).
* **100M:** C1 DB saturate, C2/C3 Redis cost, per-node in-memory state (honeypot 50k, botCache 100k, challenge 50k) ka memory footprint har node pe.

| # | Finding | File | Severity |
|---|---|---|---|
| P1 | **`botCache` eviction full hone pe O(n) map scan, write lock ke andar, request path pe** | `signals/goodbots.go` `cacheBotResult` | **P1** — jab cache 100k tak bhar jaata hai aur koi entry expired nahi, tab **har** new lookup poora 100k map iterate karta hai mutex hold karke → sab bot-claiming requests serialize |
| P2 | **`challenge.consume` har verify pe poora `used` map scan karta hai mutex ke andar** | `challenge/challenge.go` `consume` | **P1** — 50k entries tak, unauth endpoint pe; lock contention + CPU |
| P3 | **Redis RTTs per request** | `velocity.go`, `pattern.go` | **P2** |
| P4 | **`maxNavPerWindow=20/s` NAT/corporate IP pe FP risk** | `velocity.go` | **P2** (docs acknowledged) |
| P5 | **No `ReadTimeout`/`WriteTimeout` on `http.Server`** | `main.go` | **P2** — slow-loris body / slow origin pe connection budget |
| P6 | **`MaxConnsPerHost` unset** | `core/proxy.go` | **P2** |
| P7 | **Health check shallow — Redis/DB health reflect nahi karta** | `guard.go` healthz | **P1** — dead DB wala node "healthy" dikhata hai |
| P8 | **Redis outage ek hi circuit se teeno signals band** | `redis_circuit.go` | **P3** (by design, docs acknowledged) |
| P9 | **`rateLimitMs` var hai const nahi** | `velocity.go` | **P3** style |

---

## 7. QA / Test Gaps

Tests **kaafi acche** hain — 173 test/benchmark functions, table-driven cases, failure paths, concurrency (`-race`), token forgery, malformed input. Ye genuinely strong hai.

Lekin serious gaps:

1. **Integration tests `main.go` ka mux bypass karte hain (P0).** Isi wajah se 2.1 ka route-shadowing bug pass ho gaya. `TestGuardHealthzEndpoint` aur `TestHoneypotTrapEndToEnd` direct handler call karte hain. **Fix:** ek test jo real `http.NewServeMux()` wahi patterns register karke `/__hakaishield/healthz` pe 200 aur `/__hakaishield/trap` pe honeypot recording assert kare.
2. **`isCommonBrowserJA4` empty-list behavior ka test nahi (P0).** `velocity_test.go` `AddCommonBrowserPrefix` call karke happy path test karta hai — matlab production ke default (empty) state untested hai. Mutation gap.
3. **`db` package mein `[no test files]`** — `GetTenant`, `ListDomains`, `CreateDomain` ka koi direct test nahi. SQL-level regressions pakde nahi jaayenge.
4. **`rules` / `settings` stores ke SQL paths untested** — sirf encode/decode aur nil-pool "+0 tests". `Upsert`/`List` real DB ke bina verify nahi.
5. **Dashboard frontend ka koi test nahi** (`package.json` mein test script hi nahi). CLAUDE.md Section 27 kehta hai dashboard production code hai — 19 pages, zero tests. `npm audit` ke 2 moderate CVEs bhi open hain.
6. **No load test / p99 numbers** — docs khud acknowledge karte hain. 100M req claim ke liye koi evidence nahi.
7. **Shadow-mode honeypot behavior untested** — trap shadow mode mein bhi 404 deta hai (forward nahi karta), jo "shadow forwards everything" promise ko todta hai.
8. **Mutation checks documented hain** (acche), par specifically naye route layer pe nahi.

---

## 8. Code Quality Issues

Overall quality **high** hai. Chhoti cheezein:

| # | Issue | File | Severity |
|---|---|---|---|
| Q1 | **Duplicate top-offenders implementations** — `TopOffendersHandler` (wired) vs `DashboardTopOffendersHandler` (unwired) — do alag aggregation logic same data pe | `api/dashboard_extra.go` vs `api/report.go` | **P2→P3** |
| Q2 | **`requestClientIP` dead** — sirf test mein use hota hai | `core/identity.go` | **P3** |
| Q3 | **`dashboard_extra.go` aur `report.go` dono mein offender aggregation ka duplication** | — | **P3** |
| Q4 | **`go.mod`: `sentry-go` aur `golang-jwt/jwt` direct import hain par `// indirect` marked** | `go.mod` | **P3** (`go mod tidy`) |
| Q5 | **Inconsistent error helper use** — kuch handlers `http.Error` use karte hain, kuch `writeError` envelope | `api/*.go` | **P3** |
| Q6 | **`loadDotEnv` hand-rolled** — acceptable hai, comment mein reasoning diya hai | `main.go` | ✅ |
| Q7 | **Healthz JSON manually string-concat se bana** — bura nahi, par `json.Marshal` safer | `guard.go` | **P3** |

Over-engineering ya dead abstraction bahut kam hai — ye positive hai. `deception.lastIndexFold`, `redisCircuit`, `honeypot` sab justified hain.

---

## 9. Architecture / File Structure Issues

* **Positive:** Package boundaries (`signals` pure detection, `core` request-path orchestration, `tenant` isolation, `api` dashboard) saaf hain. Dependency direction theek hai, circular deps nahi. `main.go` thin hai. `ProcyFactory` callback se `tenant → core` import cycle avoid kiya — smart.
* **A1 (P2): Docs stale paths.** `docs/ARCHITECTURE.md` references `proxy/proxy.go`, `cmd/hakaishield`, `proxy/guard.go`, `proxy/evidence.go` — lekin actual code `backend/pkg/core/`, `backend/pkg/evidence/`, aur `backend/main.go` hai. Naya agent isse confuse hoga.
* **A2 (P2): README vs code mismatch.** README kehta hai `GET /api/v1/dashboard/stats` "needs no token"; code mein `DashboardStatsHandler` ab `RequireAuth` ke under hai (`main.go`). Doc galat.
* **A3 (P2): Env var naming mismatch.** `main.go` `HAKAISHIELD_CHALLENGE_SECRET` padhta hai; `PROGRESS.md`/`ROADMAP.md` `BOTSHIELD_CHALLENGE_SECRET` likhte hain. Ek dikkat: multi-node cluster mein agar operator docs dekh ke BOTSHIELD set karega, to node random secret use karega → nodes ke tokens ek dusre ko reject karenge.
* **A4 (P2): `signals` package 15+ files mein split hai** — mostly fine, par `score.go` (checks table), `velocity.go`, `pattern.go`, `ja4db.go`, `goodbots.go` sab request-path external-dependency touch karte hain. Ek "external-state" interface/group banane se testing aur reasoning aasan hota.
* **A5 (P2): Dashboard rules/settings UI jo kuch nahi karta** — architecturally ye "CRUD that lies" hai. Ya wire karo ya UI mein "not enforced yet" clearly dikhao (docs kehta hai kuch jagah dikhaya hai, par rules page pe nahi).
* **A6 (P3): DDL `pkg/db` ke andar** — migrations ka sahi ghar nahi.

**Architecture ke bare mein honest baat:** Ye ek single-binary, single-region, in-memory-state-per-node design hai jo **1 node ke liye sahi** hai. 100M requests / multi-region ke liye per-node in-memory state (Stats, Trail, honeypot, botCache, challenge nonce, challenge secret) **shared store** mein jaana padega — warna node A ke stats node B pe nahi dikhenge aur challenge cookie fail hogi. Ye roadmap item 20 ka core hai, par code abhi (rightly) single-node hai — isko "100M ready" na maano.

---

## 10. Dead / Redundant Code

| Item | File | Status |
|---|---|---|
| `DashboardTopOffendersHandler` | `api/report.go` | **Unwired** — main.go ise register nahi karta; sirf test use karta hai |
| `DashboardExportHandler` (CSV) | `api/report.go` | **Unwired** — koi route nahi |
| `requestClientIP` | `core/identity.go` | Sirf test |
| `AddKnownScraperJA4` / `AddCommonBrowserPrefix` | `signals/ja4db.go` | Production mein kabhi call nahi (tests only) — isi se 2.2 paida hota hai |
| `users` table | `pkg/db/db.go` | Dead schema (Supabase auth ke baad) |
| `WithJA4` | `core/capture.go` | Test-only override; theek hai par note-worthy |

**Dhyan:** `report.go` delete karne se pehle `report_test.go` bhi update karna hoga. `CLAUDE.md` kehta hai "certainty impossible ho to owner se poocho" — ye cases clear hain (main.go mein reference nahi), par export feature roadmap mein aa sakta hai, to confirm karke hatao.

---

## 11. Quick Wins (kam effort, achha payoff)

1. **Route shadowing fix (2.1)** — 3 lines + ek integration test. Honeypot live + healthz sahi.
2. **JA4 aggregate cap ko empty-prefix pe fail-open karo (2.2)** — chhota guard + startup warning.
3. **Negative cache for unknown Host (2.3)** — bounded map + TTL, ~30 lines.
4. **`go mod tidy`** — direct deps ko `// indirect` se hatao.
5. **README/ARCHITECTURE ke stale paths aur stats-auth claim fix** — docs consistency.
6. **Env var naming align (A3)** — `BOTSHIELD_*` ko `HAKAISHIELD_*` pe docs update ya code mein backward-compatible read.
7. **`MaxConnsPerHost` set karo** transport pe (~200).

---

## 12. High-Priority Fixes (production se pehle zaroori)

1. **P0:** Route shadowing (honeypot + healthz) — 2.1
2. **P0:** JA4 velocity false-positive — 2.2
3. **P0:** Host-header DB amplification — 2.3
4. **P0:** SSRF origin validation — 2.4
5. **P1:** Domain ownership verification + `status` enforcement — S2/D4
6. **P1:** Cross-tenant rate-limit key scoping — `velocity.go` keys mein tenant prefix add karo (`vel:ip:<tenant>:...`), warna tenant A ka traffic tenant B ko affect karega (isolation violation, CLAUDE.md §16)
7. **P1:** `botCache` eviction O(n) fix — full hone pe sweep ko amortize karo / background
8. **P1:** `challenge.consume` O(n) fix — periodic sweep, ya nonce ko token se bind karke `used` map hatao (stateless replay guard with timestamp window)
9. **P1:** Health check ko Redis/DB-aware banao (readiness vs liveness)
10. **P1:** Rules/settings ko enforce karo ya UI mein clearly "not active" dikhao

---

## 13. Long-Term Improvements

1. **Shared multi-node state:** Stats, Trail, honeypot, botCache, challenge nonce → Redis/Postgres. Roadmap item 20. Bina iske node-level scale-out galat data dikhayega.
2. **Versioned DB migrations** (golang-migrate / goose) — startup DDL hatao.
3. **Observability:** Prometheus metrics (latency per layer, decision breakdown, Redis circuit state, DB query count) — roadmap item 15. Abhi p99 numbers hi nahi hain.
4. **Load test + soak test** (roadmap item 16) — 1M/10M/100M claims ko evidence do.
5. **JA4 browser-fingerprint database pipeline** (roadmap item 19) — jo 2.2 ko safe tareeke se enable karega.
6. **Dashboard tests + `npm audit` CVEs** fix.
7. **Per-tenant bandwidth/request caps** (roadmap item 22) — hosted model mein ye margin-protecting hai; abhi koi cap nahi.

---

## Prioritized Action Plan

### P0 — Turant fix
- [ ] 2.1 Route shadowing: honeypot (`/__hakaishield/trap`) aur healthz (`/__hakaishield/healthz`) production mein dead → integration test ke saath fix
- [ ] 2.2 `isCommonBrowserJA4` empty-list pe aggregate JA4 cap fail-open karo (warna real browsers challenge loop)
- [ ] 2.3 Unknown Host pe uncached Postgres query (negative cache + bounded)
- [ ] 2.4 SSRF: origin IP-range validation + dial-time re-check

### P1 — Production/high traffic se pehle
- [ ] S2/D4 Domain ownership verification + `status='active'` gate
- [ ] Cross-tenant Redis key scoping (velocity/crawl/ja4)
- [ ] `botCache` O(n) eviction under lock
- [ ] `challenge.consume` O(n) scan under mutex
- [ ] Health/readiness endpoint Redis/DB aware
- [ ] Rules/settings enforce ya UI honest karo
- [ ] Dashboard frontend tests + CVEs

### P2 — Jaldi improve
- [ ] JWT `aud`/`iss` validation
- [ ] Redis RTTs batch karo (velocity+ja4+crawl ek pipeline)
- [ ] `crawl_pattern` write amplification (sampling / local count)
- [ ] Good-bot DNS: bounded + negative cache, request path se offload
- [ ] `MaxConnsPerHost` + server `ReadTimeout`/`WriteTimeout`
- [ ] Versioned migrations
- [ ] Docs consistency (paths, stats auth, env var names)
- [ ] `crawlerMarkers` exemption ko verified-bot pe tie karo (S10)
- [ ] Verify endpoint rate limit

### P3 — Nice to have
- [ ] Dead code hatao (`report.go` unwired handlers, `requestClientIP`, `users` table) — owner confirm ke baad
- [ ] `go mod tidy`
- [ ] Error response style consistent karo
- [ ] `rateLimitMs` const banao
- [ ] pgxpool explicit limits
- [ ] Wildcard CORS → allowlist

---

## Appendix — Verification jo maine actually chalayi

```text
cd backend
go build ./...   → clean (no output)
go vet ./...     → clean (no output)
go test ./...    → 16 packages, sab "ok", koi FAIL nahi
```

Aur grep/read se confirm kiya:
- `AddCommonBrowserPrefix` / `AddCommonBrowserPrefix` sirf tests mein → `ja4:browsers` production mein empty (find 2.2 confirm).
- `ja4:browsers` / `ja4:scrapers` keys repo mein kahin `Set`/`HSet` nahi hote (sirf read) → blocklist aur browser exemption dono production mein effectively inert.
- `DashboardTopOffendersHandler` / `DashboardExportHandler` / `requestClientIP` ka `main.go` mein koi reference nahi → dead.
- `main.go` mux patterns (`/__hakaishield/` + `/`) → route shadowing (find 2.1 confirm).

**Jo maine verify nahi kiya (honest disclosure):** real Postgres/Redis ke against load test nahi chala (env nahi), multi-node behavior simulate nahi kiya, aur dashboard ko browser mein run nahi kiya. Jo findings maine diye wo **code reading + grep + build/test** se evidence-backed hain, speculation nahi.
