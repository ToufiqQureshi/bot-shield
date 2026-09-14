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

---

## 2026-09-14 — Fixed 2 real production bugs in the capture listener
Changed:
  - `proxy/capture.go`: `handshakeAndCapture` now has `defer recover()`
    around the whole function. Without it, a crafted ClientHello that
    triggers a panic in JA4 parsing would crash the *entire process*
    (Go kills the whole program on an unrecovered panic in any
    goroutine, not just that connection) — every client behind
    bot-shield would go down over one bad handshake.
  - `proxy/capture.go`: TLS handshake now uses `HandshakeContext` with
    a 10s timeout (`handshakeTimeout`, stored via `sync/atomic` so
    tests can shrink it) instead of a bare `Handshake()` call that
    could block forever. Without this, ~1000 clients that open a
    connection and never finish handshaking would fill
    `maxHandshakes` and block every legitimate new connection
    (slowloris-style).
  - `cmd/botshield/main.go`: added `ReadHeaderTimeout`/`IdleTimeout` to
    the `http.Server` — the same slowloris risk, one layer up, for a
    client that completes the TLS handshake but then sends the HTTP
    request too slowly.
  - `proxy/capture_test.go`: added
    `TestCaptureListenerTimesOutSlowHandshake` — a client that
    connects and sends nothing must be dropped after the timeout, not
    held open. (No dedicated test added for the panic-recovery path:
    reliably forcing a panic inside the third-party `ja4` parser
    without depending on its internals wasn't worth the fragility —
    the fix itself is a single, obviously-correct `defer recover()`.)
Why: project owner asked directly whether the fingerprinting code was
production-grade. Re-reading it against `CLAUDE.md` Section 9
(fail-open, timeouts, bounded concurrency) surfaced both gaps — real
bugs, not style nits, found by re-auditing rather than by a report
from outside.
Tested how: `go build/vet/test ./... -race` clean, including the new
timeout test. Confirmed the fix by first reproducing the race in the
test itself (a shared package var without synchronization) and fixing
it with `sync/atomic` before trusting the result.
Known gaps / follow-up: the panic-recovery path is unverified by an
actual test (see above) — if a real crash from malformed input is ever
observed in the wild, add a regression test using that exact input at
that point, don't try to construct one speculatively now.

---

## 2026-09-14 — Re-audit found 2 more real bugs; added CLAUDE.md Section 22
Changed:
  - `proxy/proxy.go`: `New()`'s director now does `r.Header.Del(ja4Header)`
    unconditionally before maybe setting it. Before this, a visitor
    could set `X-BotShield-JA4` themselves and it would reach the
    origin untouched whenever real capture didn't produce a value
    (plain HTTP, or capture failing) — a spoofable "detection" signal.
  - `proxy/capture.go`: the background accept loop in
    `NewCaptureListener` now logs and retries (with backoff, capped at
    1s) on an Accept error, and only stops for real on
    `net.ErrClosed`. Before this, ANY Accept error (including a
    transient one like a brief fd-limit hit) silently killed the
    entire TLS accept loop forever — the process would look alive but
    quietly stop taking new TLS connections, with nothing logged.
  - `proxy/proxy_test.go`: added `TestNewStripsSpoofedJA4Header`.
  - `proxy/capture_test.go`: added
    `TestCaptureListenerSurvivesTransientAcceptError` (a fake listener
    that fails once, then works — proves the retry, not just that the
    code compiles).
  - `CLAUDE.md`: added Section 22, a concrete pre-push checklist
    (adversarial re-read, spoofable-input check, silent-failure check,
    "what's not tested" check) — this is now how these 2 bugs were
    actually found, made repeatable instead of one-off luck.
Why: project owner asked directly whether the fingerprinting work was
really production-grade, whether anything was skipped for speed, and
to re-check test coverage before moving on — a fair challenge, and
re-reading the diff adversarially (per the new Section 22, which this
session wrote and then immediately used) surfaced both bugs. Neither
was hypothetical: the header spoof directly undermines Section 6
(detection signals must not be trivially fakeable) before scoring
even exists to consume it, and the silent accept-loop death is exactly
the "silent failure" `docs/AGENT.md` calls out by name.
Tested how: `go build/vet/test ./... -race` clean. Both new tests
fail against the old code (verified by re-reading the diff, not just
trusting the fix) and pass against the fix. Re-ran the compiled
binary by hand: sent a request with a forged `X-BotShield-JA4` header
through a real TLS connection, confirmed the origin never saw it.
Known gaps / follow-up: this was a re-audit of already-shipped code,
not a new feature — a reminder that "tested and merged" isn't the
same as "no more bugs in it." The same adversarial re-read should
happen again before ROADMAP item 3 (UA consistency) is called done,
not just for new code but for whatever it touches in `proxy/`.

---

## 2026-09-14 — Mutation-tested our own tests; 2 of them were fake
Changed:
  - `proxy/capture_test.go`:
    `TestCaptureListenerTimesOutSlowHandshake` now checks *why* the
    connection ended — a server-side close, not the test's own read
    deadline — and that it happened near the handshake timeout. The
    old version set a 2s read deadline and only checked `err != nil`,
    so it passed whether the timeout worked or not.
  - `proxy/capture_test.go`: `TestCaptureListenerEndToEnd` now matches
    the header against a JA4 shape regex and fails if the origin was
    never reached. The old version only checked "not empty", which a
    garbage string also satisfies.
  - `proxy/proxy_test.go`: `TestNewStripsSpoofedJA4Header` now asserts
    the origin was actually reached. Its check lived inside the origin
    handler, so if the request had never arrived the test would have
    passed having proved nothing.
  - `proxy/fingerprint_test.go`: `TestJA4FingerprintRejectsGarbage`
    went from one 3-byte input to a table of 7 (nil, empty, random,
    wrong record type, truncated mid-handshake, header-only, and a
    record lying about its own length), built by chopping up a real
    curl ClientHello rather than inventing bytes. Also now asserts the
    returned fingerprint is empty, not just that an error came back.
  - `CLAUDE.md`: added Section 23 (a green suite proves nothing by
    itself) with the mandatory mutation check, the five ways a test
    lies, stale tests, "never write a test to make the suite green",
    and an explicit "do this without being asked". Linked it from the
    Section 19 and Section 22 checklists.
Why: project owner pushed back with the right question — the last two
rounds of bugs were only found because he asked. If a fake test is
sitting in the suite, nobody looks again, and it ships. So this round
audited the tests themselves instead of the production code.
Tested how: mutation testing, which is the only honest way to answer
"is this test real":
  - Deleted the handshake timeout from `capture.go` →
    `TestCaptureListenerTimesOutSlowHandshake` **passed** (took 2.00s,
    its own deadline, not the 0.20s timeout). Proof it was fake. After
    the fix, the same mutation **fails** with "read hit its own
    5.00s deadline: the server never closed the connection".
  - Replaced the real fingerprint with the literal
    `GARBAGE-NOT-A-FINGERPRINT` → `TestCaptureListenerEndToEnd`
    **passed**. Proof it was weak. After the fix, the same mutation
    **fails** naming the garbage value.
  - Removed `r.Header.Del(ja4Header)` from `proxy.go` →
    `TestNewStripsSpoofedJA4Header` correctly **failed**, so that one
    was genuinely testing its feature (the vacuous-pass guard was
    added anyway, since it was structurally possible).
  - Full suite `-race` green afterwards; production code restored
    byte-for-byte (verified with `git diff`), no behaviour changed
    this round.
Known gaps / follow-up: the remaining tests
(`TestCaptureListenerDropsBadHandshake`,
`TestCaptureListenerSurvivesTransientAcceptError`) fail by hanging
until the test timeout rather than by a clean assertion if their
feature breaks — they do catch it, but the failure message is poor.
Worth tightening with an explicit dial timeout next time either file
is touched. Also still true: the panic-recovery path has no test
(noted two entries above).




---

## 2026-09-14 — Audited against the standard library; deleted ~60 lines we shouldn't have written
Changed:
  - `proxy/capture.go`: deleted the hand-rolled TLS handshake timeout
    (`atomic.Int64` + `init()`), the accept-retry backoff loop, the
    per-connection panic recovery, the goroutine-per-connection and
    its 1000-slot semaphore, the `hack.ChannelListener` handoff, and
    the `capturedConn` type. `Accept` now returns a real `*tls.Conn`
    without handshaking it, and net/http does the handshake. The file
    went from 140 lines to 70.
  - `proxy/capture.go`: JA4 is now derived in `JA4FromContext` from
    `conn.NetConn()` at request time, instead of being computed in our
    own handshake goroutine and carried on a wrapper type.
  - `proxy/proxy.go`: migrated from `NewSingleHostReverseProxy` +
    `Director` to `httputil.ReverseProxy{Rewrite: ...}`, keeping the
    inbound `Host` and calling `SetXForwarded()`.
  - `cmd/botshield/main.go`: comment now records that
    `ReadHeaderTimeout` is also the TLS handshake deadline, so nobody
    removes it as "just a header thing".
  - `proxy/capture_test.go`: rewritten around a `startCapture` helper
    that runs the real wiring (capture listener + an `http.Server`
    configured like `cmd/botshield`), since the timeout behaviour now
    comes from that configuration.
  - `proxy/proxy_test.go`: added `TestNewStripsSpoofedForwardedFor`.
  - `CLAUDE.md`: added Section 24 (check the standard library before
    writing the code — five questions, answered by reading the source,
    not from memory), 24a (delete dead code first) and 24b (run
    `/code-review`, `/security-review`, `/simplify` on your own work).
    Linked from the Section 22 checklist.
Why: project owner asked whether existing code had unnecessary lines,
whether Go already provided any of it built-in, and whether we should
be using that instead. Reading `$GOROOT/src` rather than trusting
memory answered yes, four times over.
Tested how:
  - Read the stdlib source to confirm each claim before deleting
    anything: `Server.tlsHandshakeTimeout()` (server.go:934, takes the
    minimum of ReadHeaderTimeout/ReadTimeout/WriteTimeout and applies
    it around `HandshakeContext`), the `tempDelay` accept-retry loop
    (server.go:3420, 5ms doubling to a 1s cap, with logging),
    `conn.serve()`'s `defer recover()` (server.go:1944), and
    `tls.Conn.NetConn()` (conn.go:165).
  - Mutation checks on the new code: reverting `Rewrite` to `Director`
    makes `TestNewStripsSpoofedForwardedFor` fail with the origin
    seeing `"1.2.3.4, 127.0.0.1"` — the spoofed IP first, which is the
    value most code reads. Making `Accept` return a wrapper instead of
    a real `*tls.Conn` makes `TestCaptureListenerEndToEnd` fail.
  - Benchmarked the per-request fingerprint cost before deciding not
    to cache it: **14.3µs**, against `ARCHITECTURE.md`'s ~2ms budget.
  - `go build/vet/test ./... -race` and `gofmt -l .` clean.
  - Dead-code scan: every declared symbol in `proxy/` has live
    references; nothing removed silently.
Known gaps / follow-up:
  - One mutation check was informative in an uncomfortable way:
    `TestCaptureListenerTimesOutSlowHandshake` **passed** when `Accept`
    stopped returning a real `*tls.Conn`, because the connection was
    then treated as plain HTTP and closed by the same
    `ReadHeaderTimeout`. The test's comment claimed it proved more
    than it did; the comment now states its real scope and names the
    test that does catch that case. Worth remembering: a test can be
    honest about its result and still lie in its description.
  - HTTP/2 is still not offered, but this change makes adding it small
    — stdlib h2 negotiation works precisely because `Accept` now hands
    back a real `*tls.Conn`.
  - `proxy/errors.go` holds a single error value. Left alone rather
    than folded into `proxy.go`: `CLAUDE.md` Section 5 names
    `errors.go` as a wanted filename, and Section 13 says ask before
    removing something that merely looks surplus.
