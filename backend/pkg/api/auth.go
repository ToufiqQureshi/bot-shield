package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/ToufiqQureshi/hakaishield/pkg/account"
	"github.com/ToufiqQureshi/hakaishield/pkg/auth"
)

// signupRequest/signinRequest field names match dashboard/BACKEND_WIRING_DOCS.md
// exactly (name, email, password, company) so the frontend's existing
// fetch bodies work unchanged.
type signupRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Company  string `json:"company"`
}

type signinRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func decodeJSON(r *http.Request, v any) error {
	return json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20)).Decode(v)
}

// SignupHandler creates an account. It does not send a verification
// email (no email service is wired up — docs/PROGRESS.md) and marks
// new accounts usable immediately rather than gating them behind a
// verification step the product can't currently complete.
func SignupHandler(store *account.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		if r.Method == http.MethodOptions {
			return
		}
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var req signupRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "malformed request body")
			return
		}

		u, err := store.Signup(r.Context(), req.Email, req.Password, req.Name, req.Company)
		switch {
		case errors.Is(err, account.ErrEmailTaken):
			writeError(w, http.StatusConflict, "an account with that email already exists")
			return
		case errors.Is(err, account.ErrInvalidInput):
			writeError(w, http.StatusBadRequest, "enter a valid email and a password of at least 8 characters")
			return
		case err != nil:
			writeError(w, http.StatusInternalServerError, "could not create account")
			return
		}

		writeJSON(w, http.StatusCreated, map[string]any{
			"userId": u.ID,
			"email":  u.Email,
		})
	}
}

// SigninHandler authenticates and returns a session JWT.
func SigninHandler(store *account.Store, issuer *auth.Issuer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		if r.Method == http.MethodOptions {
			return
		}
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var req signinRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "malformed request body")
			return
		}

		u, err := store.Signin(r.Context(), req.Email, req.Password)
		if err != nil {
			// Same message and status for "no such user" and "wrong
			// password" — account.Signin already collapsed both into
			// ErrInvalidCredentials; repeating that distinction here
			// would undo the point of it.
			writeError(w, http.StatusUnauthorized, "invalid email or password")
			return
		}

		token, err := issuer.Issue(u.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not create session")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"token": token,
			"user": map[string]any{
				"id":                 u.ID,
				"email":              u.Email,
				"name":               u.FullName,
				"onboardingComplete": u.OnboardingComplete,
			},
		})
	}
}

// MeHandler returns the authenticated account's own data.
func MeHandler(store *account.Store, issuer *auth.Issuer) http.HandlerFunc {
	return RequireAuth(issuer, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		u, err := store.GetByID(r.Context(), UserIDFromContext(r.Context()))
		if errors.Is(err, account.ErrNotFound) {
			writeError(w, http.StatusNotFound, "account not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not load account")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"id":                 u.ID,
			"email":              u.Email,
			"name":               u.FullName,
			"company":            u.Company,
			"onboardingComplete": u.OnboardingComplete,
		})
	})
}

// OnboardingCompleteHandler marks the authenticated account as having
// finished onboarding. It does not persist the questionnaire content
// itself (plan/goals/team size) — no table for that exists yet, and
// the frontend's own onboarding steps are informational rather than
// gating anything downstream today.
func OnboardingCompleteHandler(store *account.Store, issuer *auth.Issuer) http.HandlerFunc {
	return RequireAuth(issuer, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if err := store.CompleteOnboarding(r.Context(), UserIDFromContext(r.Context())); err != nil {
			writeError(w, http.StatusInternalServerError, "could not save onboarding")
			return
		}
		writeMessage(w, http.StatusOK, "onboarding complete")
	})
}
