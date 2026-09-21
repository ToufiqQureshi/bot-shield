package api

import (
	"net/http"
	"sort"

	"github.com/ToufiqQureshi/hakaishield/pkg/auth"
	"github.com/ToufiqQureshi/hakaishield/pkg/db"
	"github.com/ToufiqQureshi/hakaishield/pkg/tenant"
)

// TopOffendersHandler serves GET /dashboard/top-offenders: the JA4
// fingerprints with the most recorded decisions against the caller's
// domain, worst first. It aggregates evidence.Trail in memory rather
// than adding a new store — the trail already holds exactly the data
// this needs (JA4, decision) and nothing here needs history older than
// the trail's own 24h/1000-entry window (docs/DECISIONS.md).
//
// Real IP is deliberately not in this response: evidence.Evidence
// never captured it (see its own doc comment), so there is nothing to
// leak here that visitor privacy didn't already exclude upstream.
func TopOffendersHandler(store *tenant.Store, verifier *auth.Verifier) http.HandlerFunc {
	return RequireAuth(verifier, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		domainID, ok := callerDomain(r)
		if !ok {
			writeJSON(w, http.StatusOK, []any{})
			return
		}
		ten, err := store.GetByID(domainID)
		if err != nil {
			writeJSON(w, http.StatusOK, []any{})
			return
		}

		type agg struct {
			JA4     string
			Hits    int
			Blocked int
		}
		byJA4 := map[string]*agg{}
		for _, e := range ten.Trail.Recent(0) {
			if e.JA4 == "" {
				continue
			}
			a, ok := byJA4[e.JA4]
			if !ok {
				a = &agg{JA4: e.JA4}
				byJA4[e.JA4] = a
			}
			a.Hits++
			if e.Decision == "block" {
				a.Blocked++
			}
		}

		out := make([]*agg, 0, len(byJA4))
		for _, a := range byJA4 {
			out = append(out, a)
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Hits > out[j].Hits })
		if len(out) > 20 {
			out = out[:20]
		}

		resp := make([]map[string]any, 0, len(out))
		for _, a := range out {
			resp = append(resp, map[string]any{
				"ja4":     a.JA4,
				"hits":    a.Hits,
				"blocked": a.Blocked,
			})
		}
		writeJSON(w, http.StatusOK, resp)
	})
}

// EvidenceLogsHandler serves GET /dashboard/evidence-logs: the same
// data as the pre-existing evidence-token-gated
// /api/v1/dashboard/evidence, but authorized by the caller's JWT and
// scoped to their own domain instead of a static per-deployment
// token. The two endpoints are intentionally separate (see
// backend/pkg/api/handlers.go's DashboardEvidenceHandler) rather than
// merged, since they answer to different trust boundaries: one static
// operator token for the whole deployment vs. one account's own data.
func EvidenceLogsHandler(store *tenant.Store, verifier *auth.Verifier) http.HandlerFunc {
	return RequireAuth(verifier, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		domainID, ok := callerDomain(r)
		if !ok {
			writeJSON(w, http.StatusOK, []any{})
			return
		}
		ten, err := store.GetByID(domainID)
		if err != nil {
			writeJSON(w, http.StatusOK, []any{})
			return
		}
		writeJSON(w, http.StatusOK, ten.Trail.Recent(0))
	})
}

// callerDomain resolves the authenticated user's first domain (by
// creation order) as the implicit scope for endpoints that don't
// take a domain ID. Multi-domain accounts querying a specific domain
// need db.ListDomains directly until these endpoints grow a
// ?domain= parameter — see docs/PROGRESS.md.
func callerDomain(r *http.Request) (string, bool) {
	domains, err := db.ListDomains(r.Context(), UserIDFromContext(r.Context()))
	if err != nil || len(domains) == 0 {
		return "", false
	}
	return domains[0].ID, true
}
