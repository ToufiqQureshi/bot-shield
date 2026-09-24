package api

import (
	"net/http"

	"github.com/ToufiqQureshi/hakaishield/pkg/auth"
	"github.com/ToufiqQureshi/hakaishield/pkg/db"
)

var listDomains = db.ListDomains
var dashboardOwnershipConfigured = func() bool { return db.DB != nil }

func domainJSON(d db.Domain) map[string]any {
	return map[string]any{
		"id":     d.ID,
		"domain": d.Host,
		"origin": d.Target,
		"name":   d.Name,
		"status": d.Status,
	}
}

// DomainsHandler lists domains owned by the authenticated dashboard user.
// Creating a host from a user-supplied string is disabled for the managed
// pilot: domain ownership, DNS cutover and TLS must be verified by the
// operator before a host is allowed to reserve a routing row.
func DomainsHandler(verifier *auth.Verifier) http.HandlerFunc {
	return RequireAuth(verifier, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			domains, err := listDomains(r.Context(), UserIDFromContext(r.Context()))
			if err != nil {
				writeError(w, http.StatusInternalServerError, "could not list domains")
				return
			}
			out := make([]map[string]any, 0, len(domains))
			for _, d := range domains {
				out = append(out, domainJSON(d))
			}
			writeJSON(w, http.StatusOK, out)
		case http.MethodPost:
			writeError(w, http.StatusServiceUnavailable, "domain setup is managed during the pilot; contact the operator")
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})
}
