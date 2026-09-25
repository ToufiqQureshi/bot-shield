# HakaiShield dashboard

React/Vite dashboard for the managed single-client pilot. It displays real
domain status, request totals, evidence and saved rule drafts from the Go API.
It does not activate domains or process payments. Customers can save exact
login/checkout route tags as versioned shadow policy drafts in Settings; those
tags change live velocity buckets only after the policy activation gate.
Evidence rows distinguish scored signals from yellow "Observed" candidates.
The proxy's shadow mode does not serve challenge pages, so challenge-only
observations appear only for visitors who later solve an enforced challenge.

## Local development

Use Node 24. Copy `.env.example` to `.env.local` and set your Supabase project
URL and anon key. The API defaults to `http://localhost:8080/api/v1` only in
Vite development mode.

```bash
npm ci
npm run dev
npm run typecheck
npm test
npm run build
```

## Pilot deployment

Set these build-time variables on the static site host:

- `VITE_SUPABASE_URL`: the same project used by the Go backend.
- `VITE_SUPABASE_ANON_KEY`: its public anon key.
- `VITE_API_BASE_URL`: `https://<pilot-domain>/api/v1`.
- `VITE_PILOT_CONTACT_EMAIL`: a working operator inbox. Until it is set,
  the contact page shows no send action.

Build with `npm ci && npm run build`, publish `dist/`, and configure the host to
serve `index.html` for dashboard routes. Add the dashboard URL and password
reset redirect URL to the Supabase Auth allowlist. Set `DATABASE_URL` and
`SUPABASE_URL` in `deploy/.env` so the authenticated backend API is enabled.

The first client domain is provisioned by the operator after ownership, TLS,
origin and shadow-mode checks. See `../docs/CLIENT_PILOT_RELEASE.md` for the
gate and account-to-domain binding. Restart the proxy after binding the
default owner so owner-scoped route drafts can load. A pending row does not
protect traffic.
