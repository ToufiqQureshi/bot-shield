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

func TestPassedSessionDoesNotBypassScriptingToolDetection(t *testing.T) {
	for _, mode := range []config.Mode{config.ModeEnforce, config.ModeShadow} {
		t.Run(mode.String(), func(t *testing.T) {
			originHits := 0
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				originHits++
				w.WriteHeader(http.StatusOK)
			}))
			defer origin.Close()
			proxy, err := core.NewOriginProxy(origin.URL)
			if err != nil {
				t.Fatal(err)
			}
			store := tenant.NewStore()
			if err := store.Add("pilot", tenant.TenantConfig{Target: origin.URL, Mode: mode, Policy: config.PolicyBalanced}, []string{"example.com"}, proxy); err != nil {
				t.Fatal(err)
			}
			c, err := challenge.NewChallenge([]byte("test-secret-1234567890123456789012"), "")
			if err != nil {
				t.Fatal(err)
			}
			cookie := solveChallenge(t, c, "example.com")
			req := httptest.NewRequest(http.MethodGet, "http://example.com/pricing", nil)
			req.Header.Set("User-Agent", "python-requests/2.32")
			req.AddCookie(cookie)
			rec := httptest.NewRecorder()
			core.NewGuard(store, c).ServeHTTP(rec, req)

			wantCode, wantHits := http.StatusForbidden, 0
			if mode == config.ModeShadow {
				wantCode, wantHits = http.StatusOK, 1
			}
			if rec.Code != wantCode || originHits != wantHits {
				t.Fatalf("mode %s: status=%d origin hits=%d, want %d/%d", mode, rec.Code, originHits, wantCode, wantHits)
			}
			pilot, err := store.GetByHost("example.com")
			if err != nil {
				t.Fatal(err)
			}
			rows := pilot.Trail.Recent(1)
			if len(rows) != 1 || rows[0].Decision != "block" || !slices.Contains(rows[0].Signals, "scripting_tool") {
				t.Fatalf("passed session lost detector evidence: %+v", rows)
			}
		})
	}
}

func TestPassedSessionDoesNotBypassKnownScraperJA4(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer origin.Close()
	proxy, err := core.NewOriginProxy(origin.URL)
	if err != nil {
		t.Fatal(err)
	}
	store := tenant.NewStore()
	if err := store.Add("pilot", tenant.TenantConfig{Target: origin.URL, Mode: config.ModeEnforce, Policy: config.PolicyBalanced}, []string{"example.com"}, proxy); err != nil {
		t.Fatal(err)
	}
	c, err := challenge.NewChallenge([]byte("test-secret-1234567890123456789012"), "")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "http://example.com/pricing", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.AddCookie(solveChallenge(t, c, "example.com"))
	req = req.WithContext(core.WithJA4(req.Context(), "t12d190800_4464c1bd5eb7_b3394627b738"))
	rec := httptest.NewRecorder()
	core.NewGuard(store, c).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("passed cookie hid a known scraper JA4: status=%d", rec.Code)
	}
	pilot, err := store.GetByHost("example.com")
	if err != nil {
		t.Fatal(err)
	}
	rows := pilot.Trail.Recent(1)
	if len(rows) != 1 || !slices.Contains(rows[0].Signals, "ja4_blocklist") {
		t.Fatalf("missing JA4 evidence after challenge solve: %+v", rows)
	}
}
