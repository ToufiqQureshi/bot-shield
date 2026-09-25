package core_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ToufiqQureshi/hakaishield/pkg/challenge"
	"github.com/ToufiqQureshi/hakaishield/pkg/config"
	"github.com/ToufiqQureshi/hakaishield/pkg/core"
	"github.com/ToufiqQureshi/hakaishield/pkg/tenant"
)

func newMeasuredStack(t *testing.T, body string) (*tenant.Store, *challenge.Challenge) {
	t.Helper()
	store := tenant.NewStore()
	c, _ := challenge.NewChallenge([]byte("test-secret-1234567890123456789012"), "")

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(target.Close)

	proxy, err := core.NewOriginProxy(target.URL)
	if err != nil {
		t.Fatalf("NewOriginProxy: %v", err)
	}
	if err := store.Add("default", tenant.TenantConfig{
		Target: target.URL,
		Mode:   config.ModeEnforce,
	}, []string{"example.com"}, proxy); err != nil {
		t.Fatalf("store.Add: %v", err)
	}
	return store, c
}

// P1 measurement: the egress number on the dashboard must be the bytes
// actually sent to visitors, so it is taken from the response writer
// after proxying, not from a count of requests.
func TestGuardCountsEgressBytes(t *testing.T) {
	store, c := newMeasuredStack(t, "0123456789") // 10 bytes
	guard := core.NewGuard(store, c)

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.Host = "example.com"
	rec := httptest.NewRecorder()
	guard.ServeHTTP(rec, req)

	ten, err := store.GetByID("default")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got := ten.Stats.EgressBytes(); got != 10 {
		t.Fatalf("EgressBytes = %d, want the 10 body bytes actually served", got)
	}
	if got := ten.Stats.Total(); got != 1 {
		t.Errorf("Total = %d, want 1", got)
	}
}

// The counting writer must not change what the visitor receives.
func TestMeasureWriterPassesThrough(t *testing.T) {
	rec := httptest.NewRecorder()
	mw := core.NewMeasureWriter(rec)

	mw.WriteHeader(http.StatusTeapot)
	if _, err := mw.Write([]byte("hi")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if mw.BytesWritten() != 2 {
		t.Errorf("BytesWritten = %d, want 2", mw.BytesWritten())
	}
	if rec.Code != http.StatusTeapot || rec.Body.String() != "hi" {
		t.Errorf("measure writer changed the response: code=%d body=%q", rec.Code, rec.Body.String())
	}
	if mw.Header() == nil {
		t.Error("Header must delegate to the underlying writer")
	}
}

// A request that never reaches the origin (challenge under strict
// policy) serves its own small page; it must count as egress too, not
// vanish from the cost number.
func TestGuardCountsEgressForChallengeResponse(t *testing.T) {
	store := tenant.NewStore()
	c, _ := challenge.NewChallenge([]byte("test-secret-1234567890123456789012"), "")

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("origin must not be reached for a challenged request")
	}))
	defer target.Close()

	proxy, err := core.NewOriginProxy(target.URL)
	if err != nil {
		t.Fatalf("NewOriginProxy: %v", err)
	}
	if err := store.Add("default", tenant.TenantConfig{
		Target: target.URL,
		Mode:   config.ModeEnforce,
		Policy: config.PolicyStrict, // every request is challenged
	}, []string{"example.com"}, proxy); err != nil {
		t.Fatalf("store.Add: %v", err)
	}
	guard := core.NewGuard(store, c)

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.Host = "example.com"
	rec := httptest.NewRecorder()
	guard.ServeHTTP(rec, req)

	ten, err := store.GetByID("default")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got := ten.Stats.EgressBytes(); got <= 0 {
		t.Fatalf("EgressBytes = %d, want the challenge page bytes", got)
	}
}
