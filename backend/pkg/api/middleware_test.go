package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ToufiqQureshi/hakaishield/pkg/auth"
)

func testIssuer(t *testing.T) *auth.Issuer {
	t.Helper()
	iss, err := auth.NewIssuer([]byte("test-secret-for-middleware-tests"))
	if err != nil {
		t.Fatalf("auth.NewIssuer: %v", err)
	}
	return iss
}

func TestRequireAuth_RejectsMissingToken(t *testing.T) {
	iss := testIssuer(t)
	handlerRan := false
	h := RequireAuth(iss, func(w http.ResponseWriter, r *http.Request) { handlerRan = true })

	req := httptest.NewRequest(http.MethodGet, "/anything", nil)
	rec := httptest.NewRecorder()
	h(rec, req)

	if handlerRan {
		t.Fatal("wrapped handler ran without a token")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRequireAuth_RejectsInvalidToken(t *testing.T) {
	iss := testIssuer(t)
	handlerRan := false
	h := RequireAuth(iss, func(w http.ResponseWriter, r *http.Request) { handlerRan = true })

	req := httptest.NewRequest(http.MethodGet, "/anything", nil)
	req.Header.Set("Authorization", "Bearer not-a-real-token")
	rec := httptest.NewRecorder()
	h(rec, req)

	if handlerRan {
		t.Fatal("wrapped handler ran with a forged token")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRequireAuth_AcceptsValidTokenAndSetsUserID(t *testing.T) {
	iss := testIssuer(t)
	token, err := iss.Issue("usr_real_user")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	var gotUserID string
	h := RequireAuth(iss, func(w http.ResponseWriter, r *http.Request) {
		gotUserID = UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/anything", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if gotUserID != "usr_real_user" {
		t.Fatalf("UserIDFromContext = %q, want %q", gotUserID, "usr_real_user")
	}
}

func TestRequireAuth_TokenFromDifferentSecretRejected(t *testing.T) {
	iss := testIssuer(t)
	otherIssuer, err := auth.NewIssuer([]byte("a-completely-different-secret"))
	if err != nil {
		t.Fatalf("auth.NewIssuer: %v", err)
	}
	token, err := otherIssuer.Issue("usr_attacker")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	handlerRan := false
	h := RequireAuth(iss, func(w http.ResponseWriter, r *http.Request) { handlerRan = true })

	req := httptest.NewRequest(http.MethodGet, "/anything", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	h(rec, req)

	if handlerRan {
		t.Fatal("wrapped handler ran with a token signed by a different secret")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRequireAuth_OptionsPassesWithoutToken(t *testing.T) {
	iss := testIssuer(t)
	handlerRan := false
	h := RequireAuth(iss, func(w http.ResponseWriter, r *http.Request) { handlerRan = true })

	req := httptest.NewRequest(http.MethodOptions, "/anything", nil)
	rec := httptest.NewRecorder()
	h(rec, req)

	if handlerRan {
		t.Fatal("wrapped handler should not run for an OPTIONS preflight")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("OPTIONS status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Fatal("OPTIONS response missing CORS headers")
	}
}
