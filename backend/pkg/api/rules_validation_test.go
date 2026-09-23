package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ToufiqQureshi/hakaishield/pkg/rules"
)

func TestCreateRuleRejectsUnsupportedFieldWithBadRequest(t *testing.T) {
	auth := newIsolationAuth(t)
	handler := CreateRuleHandler(rules.NewStore(nil), auth.verifier(t))
	request := httptest.NewRequest(http.MethodPost, "/api/v1/rules/custom", strings.NewReader(`{
		"name":"ASN block", "action":"BLOCK",
		"conditions":[{"field":"ASN","operator":"EQUALS","value":"123"}]
	}`))
	request.Header.Set("Authorization", "Bearer "+auth.sign(t, "owner"))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "unknown condition field") {
		t.Fatalf("body = %s, want actionable validation error", response.Body.String())
	}
}
