package observability

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCountersSnapshotAndHandler(t *testing.T) {
	resetCountersForTest()
	Inc("jwks_refresh_total")
	Inc("jwks_refresh_total")
	Inc("redis_circuit_open_total")

	snapshot := Snapshot()
	if snapshot["jwks_refresh_total"] != 2 {
		t.Fatalf("jwks_refresh_total = %d, want 2", snapshot["jwks_refresh_total"])
	}
	if snapshot["redis_circuit_open_total"] != 1 {
		t.Fatalf("redis_circuit_open_total = %d, want 1", snapshot["redis_circuit_open_total"])
	}

	handler := CountersHandler("ops-token")
	missing := httptest.NewRecorder()
	handler.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/__hakaishield/observability", nil))
	if missing.Code != http.StatusUnauthorized {
		t.Fatalf("missing token status = %d, want 401", missing.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/__hakaishield/observability", nil)
	req.Header.Set("Authorization", "Bearer ops-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]int64
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode counters: %v", err)
	}
	if body["jwks_refresh_total"] != 2 {
		t.Fatalf("handler jwks_refresh_total = %d, want 2", body["jwks_refresh_total"])
	}
}
