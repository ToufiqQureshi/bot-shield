package core_test

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/ToufiqQureshi/hakaishield/pkg/challenge"
	"github.com/ToufiqQureshi/hakaishield/pkg/config"
	"github.com/ToufiqQureshi/hakaishield/pkg/core"
	"github.com/ToufiqQureshi/hakaishield/pkg/policy"
	"github.com/ToufiqQureshi/hakaishield/pkg/signals"
	"github.com/ToufiqQureshi/hakaishield/pkg/tenant"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestGuardAppliesOnlyActivatedTenantRouteLabels(t *testing.T) {
	mini := miniredis.RunT(t)
	signals.InitRedis(redis.NewClient(&redis.Options{Addr: mini.Addr()}))
	t.Cleanup(func() { signals.InitRedis(nil) })
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	defer origin.Close()
	proxy, err := core.NewOriginProxy(origin.URL)
	if err != nil {
		t.Fatal(err)
	}
	store := tenant.NewStore()
	for _, id := range []string{"one", "two"} {
		if err := store.Add(id, tenant.TenantConfig{Mode: config.ModeShadow, Policy: config.PolicyBalanced}, []string{id + ".example"}, proxy); err != nil {
			t.Fatal(err)
		}
	}
	c, err := challenge.NewChallenge([]byte("test-secret-1234567890123456789012"), "")
	if err != nil {
		t.Fatal(err)
	}
	guard := core.NewGuard(store, c).WithPolicyProvider(func(id string) *policy.Policy {
		if id == "one" {
			return &policy.Policy{Mode: "enforce", RouteClasses: map[string]string{"/account/signin": policy.ClassLogin}}
		}
		return &policy.Policy{Mode: "shadow", RouteClasses: map[string]string{"/account/signin": policy.ClassLogin}}
	})
	for _, id := range []string{"one", "two"} {
		for i := 0; i < 12; i++ {
			r := httptest.NewRequest("POST", "http://"+id+".example/account/signin", nil)
			r.RemoteAddr = "192.0.2.15:30000"
			w := httptest.NewRecorder()
			guard.ServeHTTP(w, r)
			if w.Code != http.StatusNoContent {
				t.Fatalf("tenant %s request %d status=%d", id, i, w.Code)
			}
		}
	}
	one, _ := store.GetByHost("one.example")
	two, _ := store.GetByHost("two.example")
	spiked := false
	for _, e := range one.Trail.Recent(0) {
		spiked = spiked || slices.Contains(e.Signals, "velocity_spike")
	}
	if !spiked {
		t.Fatal("activated custom login never fired velocity_spike")
	}
	for _, e := range two.Trail.Recent(0) {
		if slices.Contains(e.Signals, "velocity_spike") {
			t.Fatal("shadow draft or other tenant affected live velocity")
		}
	}
}
