package core_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ToufiqQureshi/bot-shield/pkg/challenge"
	"github.com/ToufiqQureshi/bot-shield/pkg/config"
	"github.com/ToufiqQureshi/bot-shield/pkg/core"
	"github.com/ToufiqQureshi/bot-shield/pkg/tenant"
)

func TestGuardPassesTraffic(t *testing.T) {
	store := tenant.NewStore()
	c, _ := challenge.NewChallenge()

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("origin"))
	}))
	defer target.Close()

	proxy, _ := core.NewOriginProxy(target.URL)
	store.Add("default", tenant.TenantConfig{
		Target: target.URL,
		Mode:   config.ModeEnforce,
	}, []string{"example.com"}, proxy)

	guard := core.NewGuard(store, c)

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	rec := httptest.NewRecorder()

	guard.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", rec.Code)
	}
}
