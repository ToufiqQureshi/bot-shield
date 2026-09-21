package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/ToufiqQureshi/hakaishield/pkg/auth"
)

type ctxKeyUserID struct{}

// RequireAuth wraps a handler so it only runs for a request carrying a
// valid dashboard session JWT, making the authenticated user's ID
// available to it via UserIDFromContext. CORS headers are set here
// rather than per-handler because every authenticated dashboard route
// needs the same ones, including on the OPTIONS preflight this
// function answers before the wrapped handler ever runs.
func RequireAuth(issuer *auth.Issuer, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
		if r.Method == http.MethodOptions {
			return
		}

		tokenStr := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if tokenStr == "" {
			writeError(w, http.StatusUnauthorized, "missing bearer token")
			return
		}
		userID, err := issuer.Verify(tokenStr)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		next(w, r.WithContext(context.WithValue(r.Context(), ctxKeyUserID{}, userID)))
	}
}

// UserIDFromContext returns the authenticated user's ID, set by
// RequireAuth. Empty if called outside a RequireAuth-wrapped handler.
func UserIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(ctxKeyUserID{}).(string)
	return id
}
