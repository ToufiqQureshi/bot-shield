package signals

import (
	"slices"
	"testing"
)

func TestVelocityBucketClassifiesByEndpoint(t *testing.T) {
	cases := []struct {
		path, method string
		wantBucket   string
		wantLimit    int64
	}{
		{"/pricing", "GET", "nav", maxNavPerWindow},
		{"/login", "POST", "login", maxLoginPerWindow},
		{"/api/v1/users", "GET", "api", maxAPIPerWindow},
		{"/checkout", "POST", "checkout", maxCheckoutPerWindow},
		{"/static/app.js", "GET", "asset", maxAssetPerWindow},
	}
	for _, tc := range cases {
		key, limit := velocityBucket("t", "1.2.3.4", tc.path, tc.method, 0)
		if limit != tc.wantLimit {
			t.Errorf("velocityBucket(%q,%q) limit = %d, want %d", tc.path, tc.method, limit, tc.wantLimit)
		}
		if want := "vel:t:1:t:ip:1.2.3.4:" + tc.wantBucket + ":0"; key != want {
			t.Errorf("velocityBucket(%q,%q) key = %q, want %q", tc.path, tc.method, key, want)
		}
	}
}

// A credential-stuffing run hits /login over and over; a browser never
// posts 30 logins a second, but it does click around the site. The login
// counter must fill on its own without nav traffic and without tripping
// the nav limit early.
func TestLoginBucketFillsSeparatelyFromNav(t *testing.T) {
	newTestRedis(t)
	for i := 0; i < maxLoginPerWindow; i++ {
		if checkVelocitySpike("t", "1.2.3.4", "/login", "POST") {
			t.Fatalf("login %d: spiked before exceeding maxLoginPerWindow=%d", i, maxLoginPerWindow)
		}
	}
	// The login burst must not have touched the nav counter: the same IP
	// can still browse at normal speed.
	if checkVelocitySpike("t", "1.2.3.4", "/pricing", "GET") {
		t.Fatal("login burst leaked into the nav counter")
	}
	if !checkVelocitySpike("t", "1.2.3.4", "/login", "POST") {
		t.Fatal("login over maxLoginPerWindow must spike")
	}
}

func TestAPIAndCheckoutBucketsAreIndependent(t *testing.T) {
	newTestRedis(t)
	for i := 0; i < maxAPIPerWindow; i++ {
		checkVelocitySpike("t", "2.2.2.2", "/api/v1/list", "GET")
	}
	if !checkVelocitySpike("t", "2.2.2.2", "/api/v1/list", "GET") {
		t.Fatal("api over maxAPIPerWindow must spike")
	}
	if checkVelocitySpike("t", "2.2.2.2", "/checkout", "POST") {
		t.Fatal("api burst must not flag the first checkout request")
	}
}

// Classify is defensive about the method because r.Method is
// client-controlled input on the request line.
func TestVelocityBucketHandlesHostileMethod(t *testing.T) {
	key, limit := velocityBucket("t", "1.2.3.4", "/static/app.js", "get", 0)
	if limit != maxAssetPerWindow {
		t.Errorf("lowercase get must still classify as an asset fetch, got limit %d", limit)
	}
	if key == "" {
		t.Fatal("empty method must still produce a key, not an uncounted request")
	}
	_, limit = velocityBucket("t", "1.2.3.4", "/x", "PROPFIND", 0)
	if limit != maxNavPerWindow {
		t.Errorf("unknown methods must fall back to the nav limit, got %d", limit)
	}
}

// The full Evaluate path must carry Method through, so the live check
// sees the endpoint class and not just the path.
func TestVelocitySignalUsesMethodFromFacts(t *testing.T) {
	newTestRedis(t)
	var last Evaluation
	for i := 0; i < maxLoginPerWindow+1; i++ {
		last = Evaluate(RequestFacts{Tenant: "t", IP: "4.4.4.4", Path: "/login", Method: "POST"})
	}
	if !slices.Contains(last.Signals, "velocity_spike") {
		t.Fatal("a login flood must fire velocity_spike")
	}
}

// Endpoints stay tenant-scoped: one customer's login flood must not
// rate-limit another customer's.
func TestEndpointBucketsIsolatedPerTenant(t *testing.T) {
	newTestRedis(t)
	for i := 0; i < maxLoginPerWindow+1; i++ {
		checkVelocitySpike("tenant-a", "5.5.5.5", "/login", "POST")
	}
	if checkVelocitySpike("tenant-b", "5.5.5.5", "/login", "POST") {
		t.Fatal("tenant B inherited tenant A's login counter")
	}
}

func TestCustomLoginRouteUsesExistingVelocitySignal(t *testing.T) {
	newTestRedis(t)
	var last Evaluation
	for i := 0; i < maxLoginPerWindow+1; i++ {
		last = Evaluate(RequestFacts{Tenant: "custom", IP: "6.6.6.6", Path: "/account/signin", Method: "POST", RouteClass: ClassLogin})
	}
	if !slices.Contains(last.Signals, "velocity_spike") {
		t.Fatal("custom login route did not use the stricter existing velocity signal")
	}
	if other := Evaluate(RequestFacts{Tenant: "other", IP: "6.6.6.6", Path: "/account/signin", Method: "POST"}); slices.Contains(other.Signals, "velocity_spike") {
		t.Fatal("route label leaked across tenants")
	}
}

func TestRouteOverrideCannotLoosenBuiltInLogin(t *testing.T) {
	_, limit := velocityBucketForClass("t", "1.2.3.4", "/login", "POST", ClassCheckout, 0)
	if limit != maxLoginPerWindow {
		t.Fatalf("malformed policy loosened built-in login limit to %d", limit)
	}
}
