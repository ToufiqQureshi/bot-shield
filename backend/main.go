// Command hakaishield runs the hakaishield reverse proxy.
package main

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ToufiqQureshi/hakaishield/pkg/api"
	"github.com/ToufiqQureshi/hakaishield/pkg/auth"
	"github.com/ToufiqQureshi/hakaishield/pkg/challenge"
	"github.com/ToufiqQureshi/hakaishield/pkg/config"
	"github.com/ToufiqQureshi/hakaishield/pkg/core"
	"github.com/ToufiqQureshi/hakaishield/pkg/db"
	"github.com/ToufiqQureshi/hakaishield/pkg/observability"
	"github.com/ToufiqQureshi/hakaishield/pkg/rules"
	"github.com/ToufiqQureshi/hakaishield/pkg/settings"
	"github.com/ToufiqQureshi/hakaishield/pkg/signals"
	"github.com/ToufiqQureshi/hakaishield/pkg/tenant"

	"github.com/getsentry/sentry-go"
	"github.com/redis/go-redis/v9"
)

func main() {
	addr := flag.String("addr", ":8080", "address to listen on")
	target := flag.String("target", "", "origin server to protect, e.g. https://example.com")
	certFile := flag.String("tls-cert", "", "TLS certificate file; enables TLS + JA4 fingerprinting")
	keyFile := flag.String("tls-key", "", "TLS private key file, required with -tls-cert")
	challengeSecret := flag.String("challenge-secret", "", "Shared secret for stateless JS challenges. If empty, a random one is generated.")
	evidenceToken := flag.String("evidence-token", "", "bearer token for the per-request evidence endpoint; unset leaves the endpoint off")
	modeFlag := flag.String("mode", "enforce", `"enforce" acts on scores; "shadow" only records what it would have done`)
	themeFlag := flag.String("theme", "ghost", `challenge page theme: "ghost", "branded", or "default"`)
	policyFlag := flag.String("policy", "balanced", `policy strategy: "balanced" (allow clean score 0, challenge suspicious) or "strict" (mandatory challenge)`)
	deceptionFlag := flag.Bool("deception", false, "enable deception mode (forwards high-confidence bots to origin with X-HakaiShield-Decision: deceive instead of 403)")
	redisURL := flag.String("redis-url", "redis://localhost:6379", "Redis connection URL for distributed rate limiting")
	dbURL := flag.String("db-url", "", "PostgreSQL URL for the Supabase project's database (Project Settings > Database in the Supabase dashboard)")
	supabaseURL := flag.String("supabase-url", "", "Supabase project URL (e.g. https://xxxx.supabase.co); used to verify dashboard session JWTs against the project's published JWKS. Required, with -db-url, to enable the domains/rules/settings API.")
	flag.Parse()

	mode, err := config.ParseMode(*modeFlag)
	if err != nil {
		log.Fatalf("hakaishield: %v", err)
	}

	policy, err := config.ParsePolicy(*policyFlag)
	if err != nil {
		log.Fatalf("hakaishield: %v", err)
	}

	// SENTRY_DSN is an env var, not a flag: flags show up in `ps aux`
	// output on shared hosts, which a DSN (while not a secret that
	// grants access to customer data) still has no reason to leak into.
	if err := observability.Init(os.Getenv("SENTRY_DSN")); err != nil {
		log.Printf("hakaishield: warning: sentry init failed: %v", err)
	}
	defer sentry.Flush(2 * time.Second)

	if *target == "" {
		log.Fatal("hakaishield: -target is required")
	}

	secretStr := *challengeSecret
	if secretStr == "" {
		secretStr = os.Getenv("HAKAISHIELD_CHALLENGE_SECRET")
	}
	secret := []byte(secretStr)
	if len(secret) == 0 {
		// Fallback to random if not provided, sufficient for single-node.
		secret = make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			log.Fatalf("hakaishield: generating random secret: %v", err)
		}
	}

	challengeHandler, err := challenge.NewChallenge(secret, *themeFlag)
	if err != nil {
		log.Fatalf("hakaishield: %v", err)
	}

	// Initialize Redis for global rate limiting
	opt, err := redis.ParseURL(*redisURL)
	if err != nil {
		log.Fatalf("hakaishield: invalid redis url: %v", err)
	}
	rdb := redis.NewClient(opt)
	ctxRdb, cancelRdb := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelRdb()
	if err := rdb.Ping(ctxRdb).Err(); err != nil {
		log.Printf("hakaishield: warning: could not connect to redis at %s: %v (falling back to open)", *redisURL, err)
	} else {
		log.Printf("hakaishield: connected to redis at %s", *redisURL)
	}
	signals.InitRedis(rdb)

	// Start dynamic JA4 synchronization from Redis
	signals.StartJA4Sync(context.Background(), rdb)

	if *dbURL != "" {
		if err := db.Init(*dbURL); err != nil {
			log.Fatalf("hakaishield: initializing postgres db: %v", err)
		}
		log.Printf("hakaishield: connected to postgres at %s", *dbURL)
	}

	store := tenant.NewStore()
	store.ProxyFactory = core.NewOriginProxy // Wire up proxy creation for lazy-loading tenants

	originProxy, err := core.NewOriginProxy(*target)
	if err != nil {
		log.Fatalf("hakaishield: creating origin proxy: %v", err)
	}

	err = store.Add("default", tenant.TenantConfig{
		Target:        *target,
		Mode:          mode,
		Policy:        policy,
		EvidenceToken: *evidenceToken,
		Deception:     *deceptionFlag,
	}, []string{"*"}, originProxy)

	if err != nil {
		log.Fatalf("hakaishield: provisioning default tenant: %v", err)
	}

	guard := core.NewGuard(store, challengeHandler)

	mux := http.NewServeMux()
	mux.Handle("/__hakaishield/", challengeHandler.Handler())
	mux.Handle("/api/v1/dashboard/stats", api.DashboardStatsHandler(store))
	mux.Handle("/", guard)

	if *evidenceToken != "" {
		mux.Handle("/api/v1/dashboard/evidence", api.DashboardEvidenceHandler(store))
	} else {
		log.Print("hakaishield: -evidence-token not set, evidence endpoint disabled")
	}

	// The domains/rules/settings dashboard API needs both a database
	// (the only durable store any of it has) and a Supabase project URL
	// (to verify session JWTs Supabase Auth issued, against that
	// project's published JWKS — see pkg/auth). Requiring both
	// explicitly means a misconfigured deployment fails loudly at
	// startup instead of silently accepting no sessions at all.
	if *dbURL != "" && *supabaseURL != "" {
		verifier, err := auth.NewVerifier(*supabaseURL)
		if err != nil {
			log.Fatalf("hakaishield: %v", err)
		}
		rulesStore := rules.NewStore(db.DB)
		settingsStore := settings.NewStore(db.DB)

		// No method prefix on any of these patterns: net/http's
		// ServeMux would reject a browser's CORS preflight OPTIONS
		// request at the routing layer before it ever reached a
		// handler's own OPTIONS short-circuit, breaking every one of
		// these from a browser (found via an end-to-end Playwright
		// run — see docs/PROGRESS.md). Every handler below already
		// checks r.Method itself (directly, or via RequireAuth), so
		// the mux doesn't need to gate on method too.
		//
		// Signup/signin/session-management are no longer this
		// backend's job — the frontend talks to Supabase Auth
		// directly (see dashboard/src/lib/supabaseClient.ts). This
		// backend only verifies the JWT Supabase already issued.
		mux.HandleFunc("/api/v1/domains", api.DomainsHandler(verifier))
		mux.HandleFunc("/api/v1/rules", api.RulesListHandler(rulesStore, verifier))
		mux.HandleFunc("/api/v1/rules/custom", api.CreateRuleHandler(rulesStore, verifier))
		mux.HandleFunc("/api/v1/rules/{id}/toggle", api.ToggleRuleHandler(rulesStore, verifier))
		mux.HandleFunc("/api/v1/settings/protection", api.ProtectionSettingsHandler(settingsStore, verifier))
		mux.HandleFunc("/api/v1/dashboard/top-offenders", api.TopOffendersHandler(store, verifier))
		mux.HandleFunc("/api/v1/dashboard/evidence-logs", api.EvidenceLogsHandler(store, verifier))
		log.Print("hakaishield: domains/rules/settings API enabled (Supabase-authenticated)")
	} else {
		log.Print("hakaishield: -db-url and/or -supabase-url not set, domains/rules/settings API disabled")
	}

	srv := &http.Server{
		Handler:           observability.Middleware(mux),
		ConnContext:       core.ConnContext,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("hakaishield: %v", err)
	}

	if *certFile != "" {
		cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
		if err != nil {
			log.Fatalf("hakaishield: loading TLS cert/key: %v", err)
		}
		// TLS 1.0/1.1 are deprecated and, for a product whose own
		// detection logic reads TLS version to spot automation
		// (UAMismatch in pkg/signals), accepting them here would also
		// undermine that signal for real visitors on old clients.
		tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
		ln = core.NewCaptureListener(ln, tlsConfig)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("hakaishield: listening on %s, protecting %s", *addr, *target)
		if mode == config.ModeShadow {
			log.Print("hakaishield: SHADOW MODE - scoring and recording only, NOTHING will be blocked or challenged")
		}
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Fatalf("hakaishield: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("hakaishield: shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("hakaishield: shutdown error: %v", err)
	}
}
