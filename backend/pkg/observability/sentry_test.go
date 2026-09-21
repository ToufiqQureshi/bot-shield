package observability_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ToufiqQureshi/hakaishield/pkg/observability"
)

func TestInitEmptyDSNIsNoop(t *testing.T) {
	if err := observability.Init(""); err != nil {
		t.Fatalf("Init(\"\") must be a no-op, got error: %v", err)
	}
}

func TestMiddlewareRecoversPanic(t *testing.T) {
	panicking := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	// The point of this test: if Middleware did not recover, this call
	// itself would panic and fail the test with an unrecovered panic
	// instead of a normal Fatalf, proving the wrapped handler surviving
	// a panic isn't accidental.
	observability.Middleware(panicking).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500 after a recovered panic, got %d", rec.Code)
	}
}

func TestMiddlewarePassesThroughNormalRequests(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("fine"))
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	observability.Middleware(ok).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("want 200 for a non-panicking handler, got %d", rec.Code)
	}
	if rec.Body.String() != "fine" {
		t.Errorf("want body %q, got %q", "fine", rec.Body.String())
	}
}
