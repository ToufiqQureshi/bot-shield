package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/ToufiqQureshi/hakaishield/pkg/auth"
	"github.com/ToufiqQureshi/hakaishield/pkg/policy"
	"github.com/ToufiqQureshi/hakaishield/pkg/policyprovider"
	"github.com/ToufiqQureshi/hakaishield/pkg/tenant"
	"github.com/ToufiqQureshi/hakaishield/pkg/tenantpolicy"
)

type policyWriteRequest struct {
	ExpectedVersion int                   `json:"expectedVersion"`
	Document        tenantpolicy.Document `json:"document"`
}
type policyRollbackRequest struct {
	ExpectedVersion int `json:"expectedVersion"`
	TargetVersion   int `json:"targetVersion"`
}
type policyPreviewRequest struct {
	Facts policy.Facts `json:"facts"`
}

func decodePolicyJSON(w http.ResponseWriter, r *http.Request, v any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("trailing JSON")
		}
		return err
	}
	return nil
}

func TenantPolicyHandler(store *tenantpolicy.Store, provider *policyprovider.Provider, verifier *auth.Verifier) http.HandlerFunc {
	return RequireAuth(verifier, func(w http.ResponseWriter, r *http.Request) {
		owner, id := UserIDFromContext(r.Context()), r.PathValue("id")
		switch r.Method {
		case http.MethodGet:
			rev, err := store.Latest(r.Context(), owner, id)
			if err != nil {
				policyError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, rev)
		case http.MethodPut:
			var req policyWriteRequest
			if decodePolicyJSON(w, r, &req) != nil {
				writeError(w, http.StatusBadRequest, "malformed request body")
				return
			}
			if req.Document.Mode != "shadow" {
				writeError(w, http.StatusBadRequest, "save a shadow policy, then activate after measured shadow traffic")
				return
			}
			rev, err := store.Save(r.Context(), owner, id, owner, req.ExpectedVersion, req.Document)
			if err != nil {
				policyError(w, err)
				return
			}
			provider.InvalidateTenant(id)
			writeJSON(w, http.StatusOK, rev)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})
}

func TenantPolicyHistoryHandler(store *tenantpolicy.Store, verifier *auth.Verifier) http.HandlerFunc {
	return RequireAuth(verifier, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		history, err := store.History(r.Context(), UserIDFromContext(r.Context()), r.PathValue("id"))
		if err != nil {
			policyError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, history)
	})
}

func TenantPolicyVersionHandler(store *tenantpolicy.Store, verifier *auth.Verifier) http.HandlerFunc {
	return RequireAuth(verifier, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		version, err := strconv.Atoi(r.PathValue("version"))
		if err != nil || version < 1 {
			writeError(w, http.StatusBadRequest, "invalid version")
			return
		}
		rev, err := store.GetVersion(r.Context(), UserIDFromContext(r.Context()), r.PathValue("id"), version)
		if err != nil {
			policyError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, rev)
	})
}

func TenantPolicyRollbackHandler(store *tenantpolicy.Store, provider *policyprovider.Provider, verifier *auth.Verifier) http.HandlerFunc {
	return RequireAuth(verifier, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		var req policyRollbackRequest
		if decodePolicyJSON(w, r, &req) != nil {
			writeError(w, http.StatusBadRequest, "malformed request body")
			return
		}
		id, owner := r.PathValue("id"), UserIDFromContext(r.Context())
		rev, err := store.Rollback(r.Context(), owner, id, owner, req.ExpectedVersion, req.TargetVersion)
		if err != nil {
			policyError(w, err)
			return
		}
		provider.InvalidateTenant(id)
		writeJSON(w, http.StatusOK, rev)
	})
}

// Preview is a hypothetical simulation; the caller supplies the facts and no
// visitor traffic is affected. Live evidence remains the rollout signal.
func TenantPolicyPreviewHandler(store *tenantpolicy.Store, verifier *auth.Verifier) http.HandlerFunc {
	return RequireAuth(verifier, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		var req policyPreviewRequest
		if decodePolicyJSON(w, r, &req) != nil {
			writeError(w, http.StatusBadRequest, "malformed request body")
			return
		}
		rev, err := store.Latest(r.Context(), UserIDFromContext(r.Context()), r.PathValue("id"))
		if err != nil {
			policyError(w, err)
			return
		}
		p, err := policy.Compile(&policy.Policy{Rules: rev.Document.Rules})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "policy unavailable")
			return
		}
		path, ok := policy.NormalizePath(req.Facts.Path)
		if !ok {
			writeError(w, http.StatusBadRequest, "invalid path")
			return
		}
		req.Facts.Path = path
		req.Facts.Class = policy.Classify(path, req.Facts.Method)
		writeJSON(w, http.StatusOK, map[string]any{"version": rev.Version, "mode": "preview", "result": policy.Evaluate(p, req.Facts)})
	})
}

type policyActivateRequest struct {
	ExpectedVersion int `json:"expectedVersion"`
}

// TenantPolicyShadowHandler exposes the measured differences an operator must
// inspect before activation. Bounded per-version aggregates survive evidence
// ring wraps but reset on process restart.
func TenantPolicyShadowHandler(store *tenantpolicy.Store, tenants *tenant.Store, verifier *auth.Verifier) http.HandlerFunc {
	return RequireAuth(verifier, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		id := r.PathValue("id")
		rev, err := store.Latest(r.Context(), UserIDFromContext(r.Context()), id)
		if err != nil {
			policyError(w, err)
			return
		}
		t, err := tenants.GetByID(id)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "tenant telemetry unavailable on this node")
			return
		}
		summary := t.PolicyShadow.Snapshot(rev.Version, time.Now())
		writeJSON(w, http.StatusOK, summary)
	})
}

// Activation requires at least 100 observed evaluations spanning 30 minutes
// for the current revision. A restart or another node cannot invent history;
// insufficient local evidence conservatively blocks activation.
func TenantPolicyActivateHandler(store *tenantpolicy.Store, provider *policyprovider.Provider, tenants *tenant.Store, verifier *auth.Verifier) http.HandlerFunc {
	return RequireAuth(verifier, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		var req policyActivateRequest
		if decodePolicyJSON(w, r, &req) != nil {
			writeError(w, http.StatusBadRequest, "malformed request body")
			return
		}
		id, owner := r.PathValue("id"), UserIDFromContext(r.Context())
		rev, err := store.Latest(r.Context(), owner, id)
		if err != nil {
			policyError(w, err)
			return
		}
		if req.ExpectedVersion != rev.Version {
			policyError(w, tenantpolicy.ErrConflict)
			return
		}
		if rev.Document.Mode != "shadow" {
			writeError(w, http.StatusConflict, "policy is already active")
			return
		}
		t, err := tenants.GetByID(id)
		if err != nil || !shadowReady(t, rev.Version, time.Now()) {
			writeError(w, http.StatusConflict, "need 100 shadow evaluations across 30 minutes on this node")
			return
		}
		doc := rev.Document
		doc.Mode = "enforce"
		active, err := store.Save(r.Context(), owner, id, owner, rev.Version, doc)
		if err != nil {
			policyError(w, err)
			return
		}
		provider.InvalidateTenant(id)
		writeJSON(w, http.StatusOK, active)
	})
}

func shadowReady(t *tenant.Tenant, version int, now time.Time) bool {
	if t == nil || t.PolicyShadow == nil {
		return false
	}
	return t.PolicyShadow.Snapshot(version, now).Ready
}

func policyError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, tenantpolicy.ErrNotFound):
		writeError(w, http.StatusNotFound, "policy or domain not found")
	case errors.Is(err, tenantpolicy.ErrConflict):
		writeError(w, http.StatusConflict, "policy version changed")
	case errors.Is(err, tenantpolicy.ErrInvalid):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "policy operation failed")
	}
}
