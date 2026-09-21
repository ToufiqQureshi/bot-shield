package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/ToufiqQureshi/hakaishield/pkg/auth"
	"github.com/ToufiqQureshi/hakaishield/pkg/db"
)

type addDomainRequest struct {
	Domain string `json:"domain"`
	Origin string `json:"origin"`
}

func domainJSON(d db.Domain) map[string]any {
	return map[string]any{
		"id":     d.ID,
		"domain": d.Host,
		"origin": d.Target,
		"name":   d.Name,
		"status": d.Status,
	}
}

func newDomainID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating domain id: %w", err)
	}
	return "dom_" + hex.EncodeToString(b), nil
}

// normalizeOrigin accepts the "host:port" form dashboard/BACKEND_WIRING_DOCS.md
// itself documents as the example ("10.0.1.50:8080") as well as a full
// URL, and returns a URL core.NewOriginProxy can actually parse
// (scheme + host required). Without this, an origin submitted exactly
// as documented would be accepted here and only fail much later, as an
// opaque "tenant not found" from the dashboard/stats endpoint once
// something tries to build a reverse proxy for it.
func normalizeOrigin(origin string) (string, error) {
	if strings.Contains(origin, "://") {
		u, err := url.Parse(origin)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return "", fmt.Errorf("not a valid URL")
		}
		return origin, nil
	}
	withScheme := "http://" + origin
	u, err := url.Parse(withScheme)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("not a valid host:port")
	}
	return withScheme, nil
}

// DomainsHandler serves GET /domains (list) and POST /domains (add).
// A newly added domain is not yet taking live traffic — see
// db.CreateDomain's comment on why it needs the proxy process to pick
// the row up before enforcement actually starts for that host.
func DomainsHandler(issuer *auth.Issuer) http.HandlerFunc {
	return RequireAuth(issuer, func(w http.ResponseWriter, r *http.Request) {
		userID := UserIDFromContext(r.Context())

		switch r.Method {
		case http.MethodGet:
			domains, err := db.ListDomains(r.Context(), userID)
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
			var req addDomainRequest
			if err := decodeJSON(r, &req); err != nil {
				writeError(w, http.StatusBadRequest, "malformed request body")
				return
			}
			host := strings.TrimSpace(strings.ToLower(req.Domain))
			rawOrigin := strings.TrimSpace(req.Origin)
			if host == "" || rawOrigin == "" {
				writeError(w, http.StatusBadRequest, "domain and origin are required")
				return
			}
			origin, err := normalizeOrigin(rawOrigin)
			if err != nil {
				writeError(w, http.StatusBadRequest, "origin must be a host:port (e.g. 10.0.1.50:8080) or a full URL")
				return
			}

			id, err := newDomainID()
			if err != nil {
				writeError(w, http.StatusInternalServerError, "could not create domain")
				return
			}
			d, err := db.CreateDomain(r.Context(), id, userID, host, origin, host)
			if err != nil {
				// The tenants table's host column is UNIQUE; a second
				// account (or the same one twice) adding the same host
				// hits that constraint. Postgres doesn't hand back a
				// typed error through this call path yet, so this
				// reports the general failure rather than guessing.
				writeError(w, http.StatusConflict, "could not add domain (it may already be registered)")
				return
			}

			writeJSON(w, http.StatusCreated, domainJSON(*d))

		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})
}
