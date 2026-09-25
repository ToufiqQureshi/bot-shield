package api

import (
	"crypto/subtle"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/ToufiqQureshi/hakaishield/pkg/auth"
	"github.com/ToufiqQureshi/hakaishield/pkg/config"
	"github.com/ToufiqQureshi/hakaishield/pkg/tenant"
)

type statsResponse struct {
	TotalRequests     int64  `json:"total_requests"`
	Passed            int64  `json:"passed"`
	Challenged        int64  `json:"challenged"`
	Blocked           int64  `json:"blocked"`
	Deceived          int64  `json:"deceived"`
	RateLimited       int64  `json:"rateLimited"`
	EgressBytes       int64  `json:"egress_bytes"`
	ChallengeSolves   int64  `json:"challenge_solves"`
	ChallengeFailures int64  `json:"challenge_failures"`
	Mode              string `json:"mode"`
	Enforcing         bool   `json:"enforcing"`
}

func DashboardStatsHandler(store *tenant.Store, verifier *auth.Verifier) http.Handler {
	return RequireAuth(verifier, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		tenantID := r.URL.Query().Get("tenant")
		if tenantID == "" {
			tenantID = "default"
		}
		if dashboardOwnershipConfigured() {
			domains, err := listDomains(r.Context(), UserIDFromContext(r.Context()))
			if err != nil {
				http.Error(w, "could not verify tenant", http.StatusInternalServerError)
				return
			}
			owned := false
			for _, domain := range domains {
				if domain.ID == tenantID {
					owned = true
					break
				}
			}
			if !owned {
				http.Error(w, "tenant not found", http.StatusNotFound)
				return
			}
		}

		ten, err := store.GetByID(tenantID)
		if err != nil {
			http.Error(w, "tenant not found", http.StatusNotFound)
			return
		}

		s := ten.Stats
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if err := json.NewEncoder(w).Encode(statsResponse{
			TotalRequests:     s.Total(),
			Passed:            s.Passed(),
			Challenged:        s.Challenged(),
			Blocked:           s.Blocked(),
			Deceived:          s.Deceived(),
			RateLimited:       s.RateLimited(),
			EgressBytes:       s.EgressBytes(),
			ChallengeSolves:   s.ChallengeSolves(),
			ChallengeFailures: s.ChallengeFailures(),
			Mode:              s.Mode.String(),
			Enforcing:         s.Mode == config.ModeEnforce,
		}); err != nil {
			log.Printf("hakaishield: encoding dashboard stats response: %v", err)
		}
	})
}

func DashboardEvidenceHandler(store *tenant.Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.URL.Query().Get("tenant")
		if tenantID == "" {
			tenantID = "default"
		}

		ten, err := store.GetByID(tenantID)
		if err != nil {
			http.Error(w, "tenant not found", http.StatusNotFound)
			return
		}

		if !authorized(r, ten.Config.EvidenceToken) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if err := json.NewEncoder(w).Encode(ten.Trail.Recent(limit)); err != nil {
			log.Printf("hakaishield: encoding dashboard evidence response: %v", err)
		}
	})
}

func authorized(r *http.Request, token string) bool {
	if token == "" {
		return false
	}
	got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	return subtle.ConstantTimeCompare([]byte(got), []byte(token)) == 1
}
