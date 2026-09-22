# What hakaishield actually is, today

**Who this is for:** the owner, or anyone who has to explain this product to a
customer, an investor, or a new hire — without overclaiming.

**The rule for this document:** if something is not finished, it says so. A
demo that promises more than the code does is a problem you find out about in
front of the person you were trying to impress.

Last checked against the code: **2026-09-22**.

---

## 1. Say it out loud in thirty seconds

> hakaishield sits in front of a website as a reverse proxy. Every request
> that comes in gets examined by nine independent checks — the TLS handshake,
> the browser's claims, its headers, its speed, its browsing pattern. Each
> check that fires adds to a risk score. Based on that score we let the visitor
> through, make them solve a JavaScript puzzle, feed them decoy data, or block
> them.
>
> The part that matters commercially is not the blocking. It's that for every
> single decision we can show the customer exactly which checks fired and why.
> Most competitors give you a score and a shrug.

If they ask what makes it different, the honest answer is **explainable
request-level evidence** — not price, not block rate.

---

## 2. What is actually running

All of this is built, tested and works today.

### The proxy
Terminates TLS itself (it has to, to see the handshake), forwards everything to
the customer's origin server. Built on Go's standard `httputil.ReverseProxy`.
Graceful shutdown, panic recovery, optional Sentry.

### The nine checks

| Check | What it catches |
|---|---|
| `fragmented_handshake` | ClientHello split across TLS records — a known fingerprint-evasion trick no real browser does |
| `ua_mismatch` | Claims to be Chrome, but the TLS handshake says otherwise |
| `header_anomaly` | Claims a browser, sends none of the headers browsers send |
| `ja4_blocklist` | TLS fingerprint verified to belong to a scraping tool |
| `scripting_tool` | User agent openly says `python-requests`, `curl`, etc. |
| `velocity_spike` | This IP is requesting faster than a person reads |
| `ja4_velocity_spike` | This TLS fingerprint is, across many IPs |
| `crawl_pattern` | Walking many distinct URLs, fetching no images or CSS |
| `honeypot_trap` | Followed an invisible link no human can see |

They are different kinds of evidence on purpose. Faking a user agent is
trivial; faking a TLS fingerprint takes effort; *actually browsing like a
person* means being slow, which defeats the point of scraping.

### The JavaScript challenge
A proof-of-work puzzle plus a canvas render plus automation-framework
detection. It catches Selenium, Puppeteer and Playwright even when the browser
underneath is genuinely real — `navigator.webdriver`, `__pwInitScripts`, the
default 800×600 viewport, a Chromium-without-Chrome client-hints brand,
SwiftShader/llvmpipe headless renderers. The property names it looks for are
obfuscated in the page so a bot cannot simply grep for them.

A real person solves it in about a second and mostly does not notice.

### Deception mode
Instead of a 403, forward the request to the origin flagged as
`X-HakaiShield-Decision: deceive`, so the customer's own application can serve
decoy data. A scraper burns time and bandwidth on garbage instead of learning
it has been detected and rotating its fingerprint.

### The honeypot
An invisible `aria-hidden`, `nofollow` link injected into deceived responses.
Following it is strong evidence of DOM-walking automation. Deliberately scored
rather than auto-blocked — a screen reader can reach a hidden link too, and
those are real people.

### Good-bot protection
Googlebot, Bingbot and Applebot are verified by reverse-then-forward DNS and
forwarded with no friction at all. Faking the user agent does not work; the DNS
has to check out. **A customer's SEO is not collateral damage.**

### Shadow mode
Run the entire pipeline, record every decision, and act on none of them. This
is how a customer sees what hakaishield would do to their real traffic before
letting it do anything. Hard to miss that it is on: a startup log line,
`"mode":"shadow"` on every stats response, `"enforced":false` on every evidence
record, and the dashboard relabels its counters to "Would block" / "Would
challenge".

### The evidence trail
The last 1000 decisions, newest first: what fired, what score, what we did,
whether it was enforced. Behind a bearer token, never wildcard CORS — it
contains visitor fingerprints, and left open it would tell a bot whether its
own fingerprint is being flagged.

### Multi-tenant
One deployment serves many customers, routed by `Host` header. Tenant identity
comes from a validated boundary, never from anything a visitor sends. Every
piece of traffic-derived state — honeypot trips, rate counters, evidence — is
scoped per customer.

### The dashboard
React + Vite. Sign-up, sign-in, password reset and email verification run on
Supabase Auth. Domains, mitigation rules and protection settings are real pages
against a real API with real tenant-scoped persistence.

### Learned scoring (the new part)
The nine checks have hand-picked weights (50, 25, 100…). Those were sensible
guesses by someone who knows the domain, but guesses. `pkg/decide` learns them
from real traffic instead — logistic regression, pure Go, 14 nanoseconds per
request, and it still produces a per-check explanation rather than a black-box
number.

`pkg/labels` collects the training data automatically: a solved challenge is a
human label, a honeypot trip is an automated one. Both are independent of our
own scoring, which is what makes them worth training on.

**It decides nothing.** It runs in shadow and records what it would have done.
See §5.

---

## 3. One request, end to end

Useful in a demo, because it is the whole product in one walkthrough.

```text
1.  Request arrives, TLS handshake captured   ->  JA4 fingerprint
2.  Host header  ->  which customer is this?  (unknown host: rejected)
3.  Verified Googlebot?                       ->  straight through, no friction
4.  Already solved a challenge?               ->  through, but still rate-limited
5.  Run all nine checks                       ->  score + the list of what fired
6.  Score -> decision                         ->  allow / challenge / deceive / block
7.  Record the evidence                       ->  what fired, score, decision
8.  Shadow mode?                              ->  record only, forward anyway
9.  Act
```

Step 7 is the product. Steps 5 and 6 are what everyone else also does.

---

## 4. The numbers you can quote

Every one of these is measured, not estimated:

| | |
|---|---|
| Full request path through the guard | **~36 µs** |
| JA4 fingerprint extraction | **14.3 µs** |
| Learned model prediction | **14 ns**, zero allocations |
| Label collection | **102 ns**, zero allocations |
| Backend code | ~6,800 lines |
| Test code | ~6,600 lines — **roughly 1:1 with the code** |
| Lint | **0 issues** across the backend |

That last pair is worth saying out loud. This is not a prototype with tests
bolted on afterwards.

---

## 5. What is NOT built

Say these before someone finds them. It costs nothing when you say it first.

**Custom rules and protection settings do not do anything yet.** The dashboard
saves them, the database stores them, and live scoring ignores them. The pages
are real; the effect is not. This is the largest single gap.

**No billing.** No Stripe, no payments, no subscriptions. There was a fake
billing UI that showed invented invoices and collected card numbers under a
false "processed by Stripe" claim — it was found and removed. Those pages now
honestly say it is not set up.

**No browser fingerprint database.** `ua_mismatch` can tell you a handshake is
broken. It cannot yet tell you "this claims Chrome 120 but is not Chrome 120",
because that needs a maintained database of real browser fingerprints, refreshed
on a schedule. **This is the actual commercial moat** — everything else here is
code, and code gets rebuilt in a fortnight. Maintained fingerprint intelligence
does not.

**No domain ownership verification, no automatic certificates.** A customer
cannot self-serve onboard a domain end to end yet.

**HTTP/1.1 only.** No HTTP/2 fingerprinting. Browsers fall back, so it works,
but a signal is missing.

**The learned model has never enforced anything**, and should not until one
open question is answered — see below.

**Never run against real production traffic.** Everything is measured against
tests, benchmarks, and driven browsers. No customer has pointed a real domain
at this.

---

## 6. The one open decision that is yours

The learned model needs labelled traffic. It collects it automatically now.
But under the default policy, **only traffic that already looks suspicious gets
challenged** — so every "this was a human" label comes from a human who already
looked suspicious.

Like a doctor who only ever examines people in a hospital, then concludes most
people are ill.

That makes the data excellent for deciding *where the line goes* and wrong
about *how much traffic sits over it*. Three ways to handle it are written up in
`docs/LEARNED_SCORING.md` §3. Until one is chosen, no learned model should
enforce anything.

This is a product decision, not an engineering one. It is waiting on you.

---

## 7. If you have to demo it

```bash
cd backend

# Shadow mode against a real site — decides nothing, records everything
./hakaishield -target https://theircompany.com -mode shadow -evidence-token secret

# Then hit it with a scraper and show them the evidence endpoint
curl -H "Authorization: Bearer secret" localhost:8080/api/v1/dashboard/evidence
```

The evidence output is the pitch. Not the block — the explanation.

A good line when someone asks why they would not just use Cloudflare: *"Ask
them why a specific request was blocked. Then ask us."*

---

## 8. How this was built, if anyone asks

Written by AI agents (Claude Code and Codex) under an engineering standard
checked into the repo as `CLAUDE.md` — which mandates tests before
implementation, mutation verification (deliberately breaking the code to prove
the test catches it), an adversarial security pass, a performance and cost
review, and a written record of every meaningful decision.

That is why `docs/PROGRESS.md` is thousands of lines: every session recorded
what it changed, what it verified, what it broke on purpose to check the tests
were real, and what it left unfinished.

It is also why this document exists.

---

## Where to read more

| | |
|---|---|
| `README.md` | Running it, every flag |
| `docs/DEPLOYMENT.md` | Where to host it and why, and what bandwidth costs |
| `deploy/README.md` | The actual commands to put it on a box |
| `docs/ARCHITECTURE.md` | How the request path fits together |
| `docs/SCORING_EXPLAINED.md` | How scoring and the model work, from zero |
| `docs/LEARNED_SCORING.md` | The model's data pipeline and open questions |
| `docs/ROADMAP.md` | Built, in progress, and what is next |
| `docs/DECISIONS.md` | Why each significant choice was made |
| `docs/RESEARCH.md` | Threats, competitors, techniques studied |
| `docs/PROGRESS.md` | The full session-by-session record |
