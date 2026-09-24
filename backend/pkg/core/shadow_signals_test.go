package core_test

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/ToufiqQureshi/hakaishield/pkg/challenge"
	"github.com/ToufiqQureshi/hakaishield/pkg/config"
	"github.com/ToufiqQureshi/hakaishield/pkg/core"
	"github.com/ToufiqQureshi/hakaishield/pkg/tenant"
)

func TestClientHintMismatchIsRecordedWithoutEnforcement(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("origin"))
	}))
	defer origin.Close()
	proxy, err := core.NewOriginProxy(origin.URL)
	if err != nil {
		t.Fatal(err)
	}
	store := tenant.NewStore()
	if err := store.Add("tenant-a", tenant.TenantConfig{Target: origin.URL, Mode: config.ModeEnforce, Policy: config.PolicyBalanced}, []string{"example.com"}, proxy); err != nil {
		t.Fatal(err)
	}
	c, err := challenge.NewChallenge([]byte("test-secret-1234567890123456789012"), "")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "http://example.com/pricing", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Sec-CH-UA", `"Chromium";v="119", "Google Chrome";v="119"`)
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	response := httptest.NewRecorder()
	core.NewGuard(store, c).ServeHTTP(response, req)
	if response.Code != http.StatusOK || response.Body.String() != "origin" {
		t.Fatalf("shadow signal changed origin response: status=%d body=%q", response.Code, response.Body.String())
	}
	tenantA, err := store.GetByHost("example.com")
	if err != nil {
		t.Fatal(err)
	}
	records := tenantA.Trail.Recent(1)
	if len(records) != 1 || records[0].Score != 0 || records[0].Decision != "allow" || !slices.Contains(records[0].ShadowSignals, "client_hint_major_mismatch") {
		t.Fatalf("want allow with a separate shadow signal, got %+v", records)
	}
}
