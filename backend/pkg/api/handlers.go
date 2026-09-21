package api

import (
	"crypto/subtle"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/ToufiqQureshi/hakaishield/pkg/config"
	"github.com/ToufiqQureshi/hakaishield/pkg/tenant"
)

type statsResponse struct {
	TotalRequests int64  `json:"total_requests"`
	Passed        int64  `json:"passed"`
	Challenged    int64  `json:"challenged"`
	Blocked       int64  `json:"blocked"`
	Deceived      int64  `json:"deceived"`
	Mode          string `json:"mode"`
	Enforcing     bool   `json:"enforcing"`
}

func DashboardStatsHandler(store *tenant.Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		tenantID := r.URL.Query().Get("tenant")
		if tenantID == "" {
			tenantID = "default"
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
			TotalRequests: s.Total(),
			Passed:        s.Passed(),
			Challenged:    s.Challenged(),
			Blocked:       s.Blocked(),
			Deceived:      s.Deceived(),
			Mode:          s.Mode.String(),
			Enforcing:     s.Mode == config.ModeEnforce,
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
