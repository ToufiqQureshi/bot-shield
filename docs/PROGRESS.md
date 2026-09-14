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
