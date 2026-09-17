package api

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/ToufiqQureshi/bot-shield/pkg/signals"
	"github.com/ToufiqQureshi/bot-shield/pkg/tenant"
)

// OffenderStats aggregates traffic data for a single JA4 fingerprint.
type OffenderStats struct {
	JA4        string   `json:"ja4"`
	Total      int      `json:"total"`
	Passed     int      `json:"passed"`
	Challenged int      `json:"challenged"`
	Blocked    int      `json:"blocked"`
	Signals    []string `json:"signals"`
}

// DashboardTopOffendersHandler returns the top JA4 fingerprints that have triggered
// the most requests, along with a breakdown of the decisions (passed/challenged/blocked).
func DashboardTopOffendersHandler(store *tenant.Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.URL.Query().Get("tenant")
		if tenantID == "" {
			tenantID = "default"
		}

		tenant, err := store.GetByID(tenantID)
		if err != nil {
			http.Error(w, "tenant not found", http.StatusNotFound)
			return
		}

		if !authorized(r, tenant.Config.EvidenceToken) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		limit := 10
		if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 {
			limit = l
		}

		evidence := tenant.Trail.Recent(0)
		counts := make(map[string]*OffenderStats)

		for _, e := range evidence {
			if e.JA4 == "" {
				continue
			}
			if counts[e.JA4] == nil {
				counts[e.JA4] = &OffenderStats{JA4: e.JA4}
			}
			stats := counts[e.JA4]
			stats.Total++

			switch e.Decision {
			case signals.DecisionAllow.String():
				stats.Passed++
			case signals.DecisionChallenge.String():
				stats.Challenged++
			case signals.DecisionBlock.String():
				stats.Blocked++
			}

			for _, sig := range e.Signals {
				found := false
				for _, exist := range stats.Signals {
					if exist == sig {
						found = true
						break
					}
				}
				if !found {
					stats.Signals = append(stats.Signals, sig)
				}
			}
		}

		var list []*OffenderStats
		for _, s := range counts {
			list = append(list, s)
		}

		sort.Slice(list, func(i, j int) bool {
			return list[i].Total > list[j].Total
		})

		if len(list) > limit {
			list = list[:limit]
		}
		// If list is nil, initialize it to an empty slice so JSON is [] instead of null.
		if list == nil {
			list = make([]*OffenderStats, 0)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		// Enable CORS so the separate dashboard dev server can call it.
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(list)
	})
}

// DashboardExportHandler returns the evidence trail formatted as a CSV file.
func DashboardExportHandler(store *tenant.Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.URL.Query().Get("tenant")
		if tenantID == "" {
			tenantID = "default"
		}

		tenant, err := store.GetByID(tenantID)
		if err != nil {
			http.Error(w, "tenant not found", http.StatusNotFound)
			return
		}

		if !authorized(r, tenant.Config.EvidenceToken) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		evidence := tenant.Trail.Recent(0)

		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", `attachment; filename="bot-shield-traffic.csv"`)
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		writer := csv.NewWriter(w)
		_ = writer.Write([]string{"Time", "JA4", "signals.Score", "signals.Decision", "Enforced", "Signals"})
		for _, e := range evidence {
			_ = writer.Write([]string{
				e.Time.Format("2006-01-02T15:04:05Z07:00"),
				e.JA4,
				strconv.Itoa(e.Score),
				e.Decision,
				strconv.FormatBool(e.Enforced),
				strings.Join(e.Signals, "|"),
			})
		}
		writer.Flush()
	})
}
