package observability

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

var (
	countersMu sync.RWMutex
	counters   = map[string]*atomic.Int64{}
)

// Inc records one occurrence of a bounded internal event. Counter names are
// code-owned constants; callers must not include visitor-controlled values.
func Inc(name string) {
	Add(name, 1)
}

// Add records n occurrences at once, for callers that handle events in
// batches and would otherwise loop over Inc.
func Add(name string, n int) {
	if name == "" || n == 0 {
		return
	}
	countersMu.RLock()
	counter := counters[name]
	countersMu.RUnlock()
	if counter == nil {
		countersMu.Lock()
		counter = counters[name]
		if counter == nil {
			counter = &atomic.Int64{}
			counters[name] = counter
		}
		countersMu.Unlock()
	}
	counter.Add(int64(n))
}

// Snapshot returns a stable copy of the current aggregate counters.
func Snapshot() map[string]int64 {
	countersMu.RLock()
	names := make([]string, 0, len(counters))
	for name := range counters {
		names = append(names, name)
	}
	countersMu.RUnlock()
	sort.Strings(names)

	out := make(map[string]int64, len(names))
	countersMu.RLock()
	defer countersMu.RUnlock()
	for _, name := range names {
		if counter := counters[name]; counter != nil {
			out[name] = counter.Load()
		}
	}
	return out
}

// CountersHandler exposes aggregate operational counters for authenticated
// operators. It never includes raw tokens, IP addresses, hostnames, or other
// visitor-controlled values.
func CountersHandler(token string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if token == "" || strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ") != token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(Snapshot())
	})
}

func resetCountersForTest() {
	countersMu.Lock()
	counters = map[string]*atomic.Int64{}
	countersMu.Unlock()
}
