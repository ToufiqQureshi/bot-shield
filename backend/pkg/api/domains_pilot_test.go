package api_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ToufiqQureshi/hakaishield/pkg/api"
)

func TestPilotDomainCreationDoesNotReserveUnverifiedHost(t *testing.T) {
	auth := newTestAuth(t)
	handler := api.DomainsHandler(auth.verifier(t))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/domains", strings.NewReader(`{"domain":"victim.example","origin":"https://origin.example"}`))
	req.Header.Set("Authorization", "Bearer "+auth.sign(t, "pilot-user"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("unverified domain creation: status=%d, want 503", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "managed during the pilot") {
		t.Fatalf("response must explain the managed pilot setup: %s", rec.Body.String())
	}
}

func TestPilotDomainCreationStillRequiresAuth(t *testing.T) {
	auth := newTestAuth(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/domains", strings.NewReader(`{"domain":"victim.example"}`))
	rec := httptest.NewRecorder()
	api.DomainsHandler(auth.verifier(t)).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated domain creation: status=%d, want 401", rec.Code)
	}
}
