package api

import (
	"errors"
	"net/http"

	"github.com/ToufiqQureshi/hakaishield/pkg/auth"
	"github.com/ToufiqQureshi/hakaishield/pkg/rules"
)

type createRuleRequest struct {
	Name       string            `json:"name"`
	Conditions []rules.Condition `json:"conditions"`
	Action     string            `json:"action"`
}

type toggleRuleRequest struct {
	Enabled bool `json:"enabled"`
}

func customRuleJSON(r rules.CustomRule) map[string]any {
	return map[string]any{
		"id":         r.ID,
		"name":       r.Name,
		"conditions": r.Conditions,
		"action":     r.Action,
		"enabled":    r.Enabled,
		"createdAt":  r.CreatedAt,
	}
}

// RulesListHandler serves GET /rules: the fixed managed-rule catalogue
// plus the account's own custom rules. "exceptions" is always empty —
// no allowlist-by-visitor feature exists yet — returned as [] rather
// than omitted so the frontend's existing shape doesn't need a
// conditional for a key that might not be there.
func RulesListHandler(store *rules.Store, issuer *auth.Issuer) http.HandlerFunc {
	return RequireAuth(issuer, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		custom, err := store.List(r.Context(), UserIDFromContext(r.Context()))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not list rules")
			return
		}
		customJSON := make([]map[string]any, 0, len(custom))
		for _, c := range custom {
			customJSON = append(customJSON, customRuleJSON(c))
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"managedRules": rules.Managed(),
			"customRules":  customJSON,
			"exceptions":   []any{},
		})
	})
}

// CreateRuleHandler serves POST /rules/custom.
func CreateRuleHandler(store *rules.Store, issuer *auth.Issuer) http.HandlerFunc {
	return RequireAuth(issuer, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var req createRuleRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "malformed request body")
			return
		}
		if req.Name == "" || req.Action == "" || len(req.Conditions) == 0 {
			writeError(w, http.StatusBadRequest, "name, at least one condition, and an action are required")
			return
		}

		rule, err := store.Create(r.Context(), UserIDFromContext(r.Context()), req.Name, req.Conditions, req.Action)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not create rule")
			return
		}
		writeJSON(w, http.StatusCreated, customRuleJSON(*rule))
	})
}

// ToggleRuleHandler serves PUT /rules/{id}/toggle.
func ToggleRuleHandler(store *rules.Store, issuer *auth.Issuer) http.HandlerFunc {
	return RequireAuth(issuer, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var req toggleRuleRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "malformed request body")
			return
		}

		id := r.PathValue("id")
		err := store.SetEnabled(r.Context(), UserIDFromContext(r.Context()), id, req.Enabled)
		if errors.Is(err, rules.ErrNotFound) {
			writeError(w, http.StatusNotFound, "rule not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not update rule")
			return
		}
		writeMessage(w, http.StatusOK, "rule updated")
	})
}
