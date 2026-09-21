package api

import (
	"errors"
	"net/http"

	"github.com/ToufiqQureshi/hakaishield/pkg/auth"
	"github.com/ToufiqQureshi/hakaishield/pkg/settings"
)

// ProtectionSettingsHandler serves GET/PUT /settings/protection.
func ProtectionSettingsHandler(store *settings.Store, issuer *auth.Issuer) http.HandlerFunc {
	return RequireAuth(issuer, func(w http.ResponseWriter, r *http.Request) {
		userID := UserIDFromContext(r.Context())

		switch r.Method {
		case http.MethodGet:
			p, err := store.Get(r.Context(), userID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "could not load settings")
				return
			}
			writeJSON(w, http.StatusOK, p)

		case http.MethodPut:
			var p settings.Protection
			if err := decodeJSON(r, &p); err != nil {
				writeError(w, http.StatusBadRequest, "malformed request body")
				return
			}
			err := store.Upsert(r.Context(), userID, p)
			if errors.Is(err, settings.ErrInvalid) {
				writeError(w, http.StatusBadRequest, "block threshold must be greater than challenge threshold")
				return
			}
			if err != nil {
				writeError(w, http.StatusInternalServerError, "could not save settings")
				return
			}
			writeMessage(w, http.StatusOK, "settings updated")

		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})
}
