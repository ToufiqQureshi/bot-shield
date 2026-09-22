package api

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ToufiqQureshi/hakaishield/pkg/auth"
	"github.com/ToufiqQureshi/hakaishield/pkg/config"
	"github.com/ToufiqQureshi/hakaishield/pkg/db"
	"github.com/ToufiqQureshi/hakaishield/pkg/evidence"
	"github.com/ToufiqQureshi/hakaishield/pkg/signals"
	"github.com/ToufiqQureshi/hakaishield/pkg/tenant"
	"github.com/golang-jwt/jwt/v5"
)

type isolationAuth struct {
	server *httptest.Server
	priv   *ecdsa.PrivateKey
	kid    string
}

func newIsolationAuth(t *testing.T) *isolationAuth {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating test key: %v", err)
	}
	ia := &isolationAuth{priv: priv, kid: "isolation-key"}
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/v1/.well-known/jwks.json", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]any{{
			"kid": ia.kid,
			"kty": "EC",
			"crv": "P-256",
			"x":   base64.RawURLEncoding.EncodeToString(ia.priv.X.Bytes()),
			"y":   base64.RawURLEncoding.EncodeToString(ia.priv.Y.Bytes()),
		}}})
	})
	ia.server = httptest.NewServer(mux)
	t.Cleanup(ia.server.Close)
	return ia
}

func (ia *isolationAuth) verifier(t *testing.T) *auth.Verifier {
	t.Helper()
	v, err := auth.NewVerifier(ia.server.URL)
	if err != nil {
		t.Fatalf("auth.NewVerifier: %v", err)
	}
	return v
}

func (ia *isolationAuth) sign(t *testing.T, subject string) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.RegisteredClaims{
		Subject: subject, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})
	tok.Header["kid"] = ia.kid
	signed, err := tok.SignedString(ia.priv)
	if err != nil {
		t.Fatalf("signing token: %v", err)
	}
	return signed
}

func TestDashboardAPIsRespectDomainOwnership(t *testing.T) {
	originalListDomains := listDomains
	originalOwnershipConfigured := dashboardOwnershipConfigured
	listDomains = func(_ context.Context, ownerUserID string) ([]db.Domain, error) {
		switch ownerUserID {
		case "user-a":
			return []db.Domain{{ID: "tenant-a", Host: "a.example.com", Status: "active"}}, nil
		case "user-b":
			return []db.Domain{{ID: "tenant-b", Host: "b.example.com", Status: "active"}}, nil
		default:
			return nil, nil
		}
	}
	dashboardOwnershipConfigured = func() bool { return true }
	t.Cleanup(func() {
		listDomains = originalListDomains
		dashboardOwnershipConfigured = originalOwnershipConfigured
	})

	store := tenant.NewStore()
	if err := store.Add("tenant-a", tenant.TenantConfig{Mode: config.ModeEnforce}, []string{"a.example.com"}, nil); err != nil {
		t.Fatalf("add tenant-a: %v", err)
	}
	if err := store.Add("tenant-b", tenant.TenantConfig{Mode: config.ModeEnforce}, []string{"b.example.com"}, nil); err != nil {
		t.Fatalf("add tenant-b: %v", err)
	}
	tenA, _ := store.GetByID("tenant-a")
	tenB, _ := store.GetByID("tenant-b")
	tenA.Stats.Record(signals.DecisionAllow)
	tenA.Trail.Record(evidence.Evidence{JA4: "ja4-a", Decision: signals.DecisionAllow.String()})
	tenB.Stats.Record(signals.DecisionBlock)
	tenB.Trail.Record(evidence.Evidence{JA4: "ja4-b", Decision: signals.DecisionBlock.String()})

	ia := newIsolationAuth(t)
	verifier := ia.verifier(t)
	userAToken := ia.sign(t, "user-a")

	t.Run("stats cannot read another tenant by guessed id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/stats?tenant=tenant-b", nil)
		req.Header.Set("Authorization", "Bearer "+userAToken)
		rec := httptest.NewRecorder()
		DashboardStatsHandler(store, verifier).ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rec.Code)
		}
	})

	t.Run("evidence logs use caller owned domain only", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/evidence-logs", nil)
		req.Header.Set("Authorization", "Bearer "+userAToken)
		rec := httptest.NewRecorder()
		EvidenceLogsHandler(store, verifier).ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		var body struct {
			Data []evidence.Evidence `json:"data"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("decode logs: %v", err)
		}
		if len(body.Data) != 1 || body.Data[0].JA4 != "ja4-a" {
			t.Fatalf("logs = %+v, want only tenant-a evidence", body.Data)
		}
	})

	t.Run("top offenders use caller owned domain only", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/top-offenders", nil)
		req.Header.Set("Authorization", "Bearer "+userAToken)
		rec := httptest.NewRecorder()
		TopOffendersHandler(store, verifier).ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		var body struct {
			Data []map[string]any `json:"data"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("decode offenders: %v", err)
		}
		if len(body.Data) != 1 || body.Data[0]["ja4"] != "ja4-a" {
			t.Fatalf("offenders = %+v, want only tenant-a offender", body.Data)
		}
	})
}
