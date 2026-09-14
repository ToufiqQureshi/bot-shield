# PROGRESS.md — What Was Done, When, and Why

A chronological work log. Every session (yours or another agent's)
appends an entry here **before stopping** — even a single small
function change, even a rename, even a "tried X, reverted it."

This is different from `docs/DECISIONS.md` (which is *why we chose
an approach*, rarely updated) and `docs/ROADMAP.md`'s Done list (which
is *what feature exists now*). This file is the blow-by-blow: what
actually happened in each work session, in order, so nothing has to
be reconstructed from git log or guessed later.

**Why this matters more than usual for this project:** code has been
handed off between sessions as a zip file, not always a git push — so
git history itself cannot be assumed to have survived. Treat this
file as the source of truth for history, not git log.

---

## Format for new entries

```text
## <date> — <short title>
Changed:
  - file/function: what changed
  - file/function: what changed
Why:
Tested how:
Known gaps / follow-up:
```

Keep entries factual and specific — "changed X because Y", not "made
improvements." A vague entry is as bad as no entry.

---

## 2026-09-14 — Backfilled RESEARCH.md and DECISIONS.md
Changed:
  - `docs/RESEARCH.md`: created — Akamai/DataDome/Cloudflare detection
    technique list, Patchright/Scrapling/obscura-scraper evasion
    summary, Bright Data-class proxy network strategy, open-source
    tools identified (fingerproxy, BotD, SafeLine, open-appsec).
  - `docs/DECISIONS.md`: added "assemble open-source, don't reinvent"
    and "large-proxy-network: fingerprint not IP" entries.
  - `CLAUDE.md`: added `docs/RESEARCH.md` to the Documentation Map
    and the mandatory pre-stop checklist.
Why: earlier conversation covered real research (competitor detection
methods, stealth-tool analysis, Bright Data threat strategy) that
existed only in chat history, not in any repo file — risked being
lost the moment a new session started with no memory of that chat.
Tested how: n/a (docs only).
Known gaps / follow-up: this file (`PROGRESS.md`) itself didn't exist
yet when this entry was written — added in the next entry, same day.

---

## 2026-09-14 — Reverse proxy skeleton (ROADMAP P0 item 1)
Changed:
  - `go.mod`: created, module `github.com/ToufiqQureshi/bot-shield`.
  - `proxy/proxy.go`: added `New(target string) (*httputil.ReverseProxy, error)`
    — wraps stdlib `httputil.NewSingleHostReverseProxy`, validates the
    target URL has a scheme+host before returning.
  - `proxy/errors.go`: added `errInvalidTarget`.
  - `proxy/proxy_test.go`: added `TestPassthrough` (request/response
    unchanged through the proxy) and `TestNewRejectsBadTarget`.
  - `cmd/botshield/main.go`: added the `botshield` binary — `-addr`/
    `-target` flags, starts the proxy, graceful shutdown on
    SIGINT/SIGTERM with a 10s drain timeout.
  - `.gitignore`: added (`/bin/`, `*.log`, `.env`).
  - `docs/ROADMAP.md`: moved item 1 from P0 list to Done, with the
    known gap noted inline.
Why: this is the first ROADMAP item — the integration point every
future detection layer (fingerprint, score, challenge) will plug
into. Built test-first per `CLAUDE.md` Section 7: wrote
`proxy_test.go` against a not-yet-existing `New()`, confirmed it
failed for the right reason (`undefined: New`), then implemented.
Tested how: `go test ./... -race` (unit tests pass); also ran a real
end-to-end smoke test — a local Python HTTP server as the origin, the
`botshield` binary in front of it, `curl` through the proxy — to
confirm actual traffic passthrough, not just mocked behavior.
`gofmt -l .` and `go vet ./...` both clean.
Known gaps / follow-up: no timeout/retry handling yet if the origin
is slow or down — a dead origin currently returns Go's default 502
with no retry. Deliberately deferred to ROADMAP item 2 (HTTP
fetcher)/retry work rather than building it ahead of the roadmap
order (see `docs/DECISIONS.md` "MVP scope" entry for why P0 stays
narrow). Also: code was committed locally but could not be pushed —
this session's GitHub access was scoped to a different repo
(`goScraper`), not `bot-shield`. Delivered as a zip instead; the
project owner pushes it manually. Whoever picks this up next should
verify the zip's contents actually landed in the real repo before
building on top of it.

---

## 2026-09-14 — Zip landed in real repo; PR merged
Changed:
  - Extracted the handed-off zip (see previous entry) into the actual
    `bot-shield` repo on branch `claude/code-review-feedback-k1zukr`,
    replacing the stray `bot-shield.zip` blob that had been committed
    to `main` directly instead of the real files. Opened as a PR,
    reviewed, merged into `main`.
  - `CLAUDE.md`: added Section 3a — every non-trivial function needs a
    2-3 line comment covering what it does, why it exists, and what
    need made it necessary. This file didn't have that rule explicit
    yet even though `docs/AGENT.md` already expects beginner-readable
    code; added on request, PR merged separately.
Why: this session had correct `bot-shield` GitHub access (previous
session did not — see prior entry), so it could close that handoff
gap instead of leaving it to the project owner.
Tested how: `go build/vet/test ./...` clean before and after; no
detection logic touched, pure delivery + docs.
Known gaps / follow-up: none — this was a delivery/recovery task, not
a feature. ROADMAP item 2 (fingerprinting) is next.

---

## 2026-09-14 — JA4 fingerprint computation (ROADMAP P0 item 2, partial)
Changed:
  - `go.mod`: added `github.com/wi1dcard/fingerproxy` as a dependency.
    Only its `pkg/ja4` package is imported (self-contained, stdlib +
    `utls` only) — deliberately did not import `pkg/fingerprint` or
    `pkg/proxyserver`, which would have pulled in Prometheus metrics
    and gopacket-based JA3 parsing we don't need yet (see
    `docs/DECISIONS.md` "minimal fingerproxy surface" entry).
  - `proxy/fingerprint.go`: added `ja4Fingerprint(clientHello []byte)
    (string, error)` — turns a raw TLS ClientHello record into its
    JA4 hash.
  - `proxy/fingerprint_test.go`: added `TestJA4Fingerprint` using two
    real, known-good ClientHello-to-JA4 vectors taken from
    fingerproxy's own test suite (curl 8.6.0, and a PSK-extension
    handshake) — not invented test data. Added
    `TestJA4FingerprintRejectsGarbage` for malformed input (this runs
    against adversarial traffic; garbage input is the normal case,
    not an edge case, per `CLAUDE.md` Section 9).
Why: ROADMAP item 2's first slice. TLS/JA4 fingerprinting needs a
correct hash function before it needs live-capture wiring — built and
tested that piece first rather than guessing at both at once.
Tested how: `go build ./...`, `go vet ./...`, `gofmt -l .` all clean.
`go test ./... -v` — both known-vector cases pass, garbage-input case
returns an error instead of a wrong/blank fingerprint. Confirmed via
`go list -deps` that Prometheus and gopacket are NOT compiled into the
binary (only `utls` and two small `quic-go` internal subpackages
`utls` itself needs).
Known gaps / follow-up (real, not deferred without reason — see
`CLAUDE.md` Section 17): **bot-shield does not terminate TLS at all
today** — `cmd/botshield` proxies plain HTTP, so nothing currently
captures a live ClientHello to feed this function. This is deliberately
not built in this same pass: it means rewriting the accept loop
(TLS-terminating listener + HTTP/1.1 vs HTTP/2 branching, following
fingerproxy's own `pkg/hack.HijackClientHelloConn` + `pkg/proxyserver`
pattern), deciding fail-open behavior for a failed/timed-out capture
(`CLAUDE.md` Section 9), and end-to-end testing against a real browser
and a real scripted client — a large piece of its own that would ship
half-done if rushed into this same commit. This is the next work item,
not "done."

---

## 2026-09-14 — Comments simplified; CLAUDE.md comment rule tightened
Changed:
  - `proxy/fingerprint.go`, `proxy/fingerprint_test.go`: comments were
    too long/technical (multi-doc citations, jargon) — cut to 1-2
    plain-English lines each.
  - `CLAUDE.md` Section 3a: added a concrete good/bad example so
    future comments stay short and plain instead of drifting long.
Why: project owner flagged the earlier comments as too dense for a
solo/beginner-friendly project.
Tested how: `go build/vet/test ./...` clean (comment-only change).
Known gaps / follow-up: none.

---

## 2026-09-14 — TLS termination + live JA4 capture (ROADMAP P0 item 2, done)
Changed:
  - `proxy/capture.go`: added `NewCaptureListener` — wraps a plain TCP
    listener so every connection gets TLS-handshaked and its JA4
    fingerprint read before the HTTP server sees it. One goroutine per
    connection does the handshake concurrently (capped at 1000
    in-flight via a semaphore, so a connection flood can't spawn
    unlimited goroutines — `CLAUDE.md` Section 9), then hands the
    finished connection to `fingerproxy/pkg/hack.ChannelListener` for
    the HTTP server to pick up. Added `ConnContext`/`JA4FromContext`
    to carry the fingerprint from the connection into each request's
    context.
  - `proxy/proxy.go`: `New()`'s director now sets `X-BotShield-JA4` on
    the forwarded request when a fingerprint was captured; forwards
    normally (no header) otherwise — fingerprinting failing must never
    block real traffic (fail open, `CLAUDE.md` Section 9).
  - `cmd/botshield/main.go`: added `-tls-cert`/`-tls-key` flags. With
    them, botshield terminates TLS and wires the capture listener;
    without them, it proxies plain HTTP exactly as before (local dev
    still works without a cert).
  - `proxy/capture_test.go`: added a real end-to-end test — actual TLS
    handshake through the capture listener, actual HTTP request
    proxied to a real origin, asserting the origin received a
    non-empty JA4 header. Added a bad-handshake test (garbage bytes
    instead of a ClientHello) proving the listener drops the bad
    connection and keeps serving good ones afterward, instead of
    hanging.
Why: this is the piece the previous "partial" entry flagged as
missing — without it, `ja4Fingerprint` had no real ClientHello to run
against. Closes ROADMAP item 2's TLS/JA4 half (HTTP/2 fingerprinting
is a separate, still-open half — see `DECISIONS.md`).
Tested how: `go build/vet/test ./... -race` all clean, `gofmt -l .`
clean. Real end-to-end test (self-signed cert generated in-test, real
`tls.Dial`/`http.Client` round trip). Also ran the actual compiled
binary by hand: generated an `openssl` self-signed cert, started
`botshield -tls-cert ... -tls-key ...` in front of a local Python HTTP
server, hit it with `curl -k` over real TLS — response came back
correctly. Also re-ran the plain-HTTP path (no `-tls-cert` flags) by
hand to confirm it still works unchanged.
Known gaps / follow-up (not deferred without reason — see `CLAUDE.md`
Section 17):
  - HTTP/2 fingerprinting is not built. The capture listener forces
    `NextProtos = ["http/1.1"]`, so a browser that would otherwise use
    HTTP/2 falls back to HTTP/1.1 against bot-shield. This is a
    deliberate scope cut (see `DECISIONS.md`), not an oversight — it
    is its own roadmap item (item 2's "HTTP/2 fingerprint" half), and
    building it means hand-rolling HTTP/2 serving alongside the
    stdlib's own HTTP/1.1 path (the way `fingerproxy/pkg/proxyserver`
    does), which is a large enough piece to ship on its own.
  - The 1000-in-flight-handshake cap is a reasonable-guess default,
    not load-tested against real adversarial volume — that belongs
    with ROADMAP item 16 (soak testing), once there's real traffic to
    tune it against.
  - JA4 alone is not a block/allow decision — nothing reads
    `X-BotShield-JA4` yet except this proxy setting it. That's
    ROADMAP items 3 (UA consistency) and 5 (scoring engine), not this
    item.

