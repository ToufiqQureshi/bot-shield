// Package observability wires crash/error reporting so a panic in a
// request handler shows up as an alert instead of a line in a log file
// nobody is watching. Without this, Go's own net/http server already
// recovers a handler panic per-connection (the process doesn't crash),
// but it only logs to stderr — on a real deployment that's the same as
// the bug never having happened until a customer complains.
package observability

import (
	"log"
	"net/http"
	"time"

	"github.com/getsentry/sentry-go"
)

// Init connects error reporting to Sentry. dsn empty is a deliberate,
// supported no-op (CLAUDE.md Section 29 — the product must still run
// without every optional integration configured); Middleware then just
// logs panics instead of also reporting them.
func Init(dsn string) error {
	if dsn == "" {
		log.Print("hakaishield: SENTRY_DSN not set, crash reporting disabled (panics still logged)")
		return nil
	}
	return sentry.Init(sentry.ClientOptions{
		Dsn: dsn,
		// The proxy path runs on every request; a panic here is a code
		// bug worth the full stack trace, not a sampled sample of them.
		AttachStacktrace: true,
	})
}

// Middleware recovers a panicking handler, reports it (if Init was
// given a DSN) and returns 500 instead of the client just seeing the
// connection drop, then lets the request finish instead of taking the
// whole process down with it.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				sentry.CurrentHub().Recover(err)
				sentry.Flush(2 * time.Second)
				log.Printf("hakaishield: recovered panic serving %s %s: %v", r.Method, r.URL.Path, err)
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
