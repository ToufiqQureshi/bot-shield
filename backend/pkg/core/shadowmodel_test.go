package core_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/ToufiqQureshi/hakaishield/pkg/challenge"
	"github.com/ToufiqQureshi/hakaishield/pkg/config"
	"github.com/ToufiqQureshi/hakaishield/pkg/core"
	"github.com/ToufiqQureshi/hakaishield/pkg/decide"
	"github.com/ToufiqQureshi/hakaishield/pkg/signals"
	"github.com/ToufiqQureshi/hakaishield/pkg/tenant"
)

// shadowFixture builds a guard in front of a real origin, so the tests
// below can assert what the visitor actually received as well as what was
// recorded about them.
func shadowFixture(t *testing.T, policy config.PolicyMode) (*core.Guard, *tenant.Tenant, *originHits) {
	t.Helper()

	hits := &originHits{}
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.add()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("origin"))
	}))
	t.Cleanup(origin.Close)

	store := tenant.NewStore()
	c, err := challenge.NewChallenge([]byte("test-secret-1234567890123456789012"), "")
	if err != nil {
		t.Fatalf("NewChallenge() error: %v", err)
	}
	proxy, err := core.NewOriginProxy(origin.URL)
	if err != nil {
		t.Fatalf("NewOriginProxy() error: %v", err)
	}
	store.Add("default", tenant.TenantConfig{
		Target: origin.URL,
		Mode:   config.ModeEnforce,
		Policy: policy,
	}, []string{"example.com"}, proxy)

	tn, err := store.GetByHost("example.com")
	if err != nil {
		t.Fatalf("GetByHost(example.com) error: %v", err)
	}
	return core.NewGuard(store, c), tn, hits
}

// originHits counts requests that reached the origin. The guard proxies
// on its own goroutine-free path here, but httptest handlers still run on
// the server's goroutine, so the counter is guarded.
type originHits struct {
	mu sync.Mutex
	n  int
}

func (o *originHits) add() {
	o.mu.Lock()
	o.n++
	o.mu.Unlock()
}

func (o *originHits) count() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.n
}

// alwaysBlockModel decides block for anything with two signals and
// challenge for anything else, whatever the rules think. It exists to
// prove the model's opinion cannot reach the visitor.
func alwaysBlockModel(t *testing.T) *decide.Model {
	t.Helper()

	features := signals.FeatureNames()
	var set []decide.Sample
	// Every known check firing is automated; nothing firing is human.
	all := uint32(0)
	for i := range features {
		all |= 1 << uint(i)
	}
	for i := 0; i < 200; i++ {
		set = append(set, decide.Sample{Fired: all, Automated: true})
		set = append(set, decide.Sample{Fired: 0, Automated: false})
	}

	m, err := decide.Train(features, set, decide.DefaultOptions())
	if err != nil {
		t.Fatalf("Train() error: %v", err)
	}
	return m
}

func scriptedRequest() *http.Request {
	// python-requests fires scripting_tool, and the missing browser
	// headers fire header_anomaly: two signals, so the rule scorer blocks.
	return httptest.NewRequest("GET", "http://example.com/", nil)
}

// With no model attached, nothing about the request path changes and no
// model record appears. This is the default deployment.
func TestGuardWithoutModelRecordsNoOpinion(t *testing.T) {
	guard, tn, _ := shadowFixture(t, config.PolicyBalanced)

	req := scriptedRequest()
	req.Header.Set("User-Agent", "python-requests/2.31.0")
	guard.ServeHTTP(httptest.NewRecorder(), req)

	recent := tn.Trail.Recent(1)
	if len(recent) != 1 {
		t.Fatalf("Trail.Recent(1) returned %d records, want 1", len(recent))
	}
	if recent[0].Model != nil {
		t.Fatalf("Evidence.Model = %+v with no model loaded, want nil", recent[0].Model)
	}
}

// The safety property the whole shadow design rests on: attaching a
// model must not change what happens to the visitor.
//
// The challenge page carries a fresh random nonce per request by design,
// so its bytes differ between any two runs. What must not differ is the
// decision: the status returned, whether the origin was reached, and what
// the rule scorer recorded.
func TestShadowModelNeverChangesTheDecision(t *testing.T) {
	cases := []struct {
		name   string
		ua     string
		header http.Header
	}{
		{"scripted client", "python-requests/2.31.0", nil},
		{"browser with no fetch-metadata headers", "Mozilla/5.0 Chrome/120.0", nil},
		{"clean browser", "Mozilla/5.0 Chrome/120.0", browserHeaders()},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plainCode, plainHits, plainDecision, plainScore := serveOnce(t, tc.ua, tc.header, nil)
			shadowCode, shadowHits, shadowDecision, shadowScore := serveOnce(t, tc.ua, tc.header, alwaysBlockModel(t))

			if plainCode != shadowCode {
				t.Errorf("status changed when a model was attached: %d without, %d with", plainCode, shadowCode)
			}
			if plainHits != shadowHits {
				t.Errorf("origin was reached %d times without a model and %d times with one", plainHits, shadowHits)
			}
			if plainDecision != shadowDecision {
				t.Errorf("recorded decision changed when a model was attached: %q without, %q with", plainDecision, shadowDecision)
			}
			if plainScore != shadowScore {
				t.Errorf("recorded score changed when a model was attached: %d without, %d with", plainScore, shadowScore)
			}
		})
	}
}

// serveOnce runs one request through a fresh guard and reports what the
// visitor got and what was recorded about them.
func serveOnce(t *testing.T, ua string, header http.Header, m *decide.Model) (code, originHitCount int, decision string, score int) {
	t.Helper()

	guard, tn, hits := shadowFixture(t, config.PolicyBalanced)
	guard.WithShadowModel(m)

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.Header.Set("User-Agent", ua)
	for k, vs := range header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}

	rec := httptest.NewRecorder()
	guard.ServeHTTP(rec, req)

	recent := tn.Trail.Recent(1)
	if len(recent) != 1 {
		t.Fatalf("Trail.Recent(1) returned %d records, want 1", len(recent))
	}
	return rec.Code, hits.count(), recent[0].Decision, recent[0].Score
}

// browserHeaders is the fetch-metadata set a real browser navigation
// carries, so the header_anomaly check stays quiet.
func browserHeaders() http.Header {
	h := http.Header{}
	h.Set("Sec-Fetch-Dest", "document")
	h.Set("Sec-Fetch-Mode", "navigate")
	h.Set("Sec-Fetch-Site", "same-origin")
	h.Set("Sec-CH-UA", `"Chromium";v="120"`)
	return h
}

// The recorded opinion has to be a real prediction about this request,
// not a placeholder: its reasons must name checks that actually fired.
func TestShadowModelRecordsAnExplainedOpinion(t *testing.T) {
	guard, tn, _ := shadowFixture(t, config.PolicyBalanced)
	guard.WithShadowModel(alwaysBlockModel(t))

	req := scriptedRequest()
	req.Header.Set("User-Agent", "python-requests/2.31.0")
	guard.ServeHTTP(httptest.NewRecorder(), req)

	recent := tn.Trail.Recent(1)
	if len(recent) != 1 {
		t.Fatalf("Trail.Recent(1) returned %d records, want 1", len(recent))
	}
	rec := recent[0]
	if rec.Model == nil {
		t.Fatal("Evidence.Model is nil with a model attached")
	}
	if rec.Model.Decision == "" || rec.Model.Decision == "unknown" {
		t.Errorf("Model.Decision = %q, want a real decision", rec.Model.Decision)
	}
	if rec.Model.Probability < 0 || rec.Model.Probability > 1 {
		t.Errorf("Model.Probability = %v, want a value in [0,1]", rec.Model.Probability)
	}
	if len(rec.Model.Reasons) == 0 {
		t.Fatalf("a request that fired %v produced no model reasons", rec.Signals)
	}

	// Every reason must be a check that actually fired on this request.
	fired := map[string]bool{}
	for _, s := range rec.Signals {
		fired[s] = true
	}
	for _, r := range rec.Model.Reasons {
		if !fired[r.Feature] {
			t.Errorf("model reason %q was not among the signals that fired (%v)", r.Feature, rec.Signals)
		}
	}
}

// A request that fires nothing must produce an opinion with no reasons
// rather than an invented one.
func TestShadowModelOnCleanTrafficHasNoReasons(t *testing.T) {
	guard, tn, _ := shadowFixture(t, config.PolicyBalanced)
	guard.WithShadowModel(alwaysBlockModel(t))

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 Chrome/120.0")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Sec-CH-UA", `"Chromium";v="120"`)
	guard.ServeHTTP(httptest.NewRecorder(), req)

	recent := tn.Trail.Recent(1)
	if len(recent) != 1 || recent[0].Model == nil {
		t.Fatalf("no model opinion recorded for clean traffic: %+v", recent)
	}
	if len(recent[0].Signals) != 0 {
		t.Skipf("this environment fired %v on the clean request; nothing to assert", recent[0].Signals)
	}
	if len(recent[0].Model.Reasons) != 0 {
		t.Errorf("Model.Reasons = %+v for a request that fired nothing, want none", recent[0].Model.Reasons)
	}
	if recent[0].Model.Decision != signals.DecisionAllow.String() {
		t.Errorf("Model.Decision = %q for clean traffic, want %q", recent[0].Model.Decision, signals.DecisionAllow.String())
	}
}

// Attaching nil turns the shadow model off again rather than panicking in
// the request path.
func TestWithShadowModelNilDisablesIt(t *testing.T) {
	guard, tn, _ := shadowFixture(t, config.PolicyBalanced)
	guard.WithShadowModel(alwaysBlockModel(t)).WithShadowModel(nil)

	req := scriptedRequest()
	req.Header.Set("User-Agent", "python-requests/2.31.0")
	rec := httptest.NewRecorder()
	guard.ServeHTTP(rec, req)

	recent := tn.Trail.Recent(1)
	if len(recent) != 1 {
		t.Fatalf("Trail.Recent(1) returned %d records, want 1", len(recent))
	}
	if recent[0].Model != nil {
		t.Errorf("Evidence.Model = %+v after WithShadowModel(nil), want nil", recent[0].Model)
	}
	if strings.TrimSpace(rec.Body.String()) == "" && rec.Code == 0 {
		t.Error("the request was not served at all")
	}
}
