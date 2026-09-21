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

	"github.com/ToufiqQureshi/hakaishield/pkg/account"
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
	dbURL := flag.String("db-url", "", "PostgreSQL URL for Supabase integration (e.g. postgres://user:pass@host:5432/db)")
	jwtSecret := flag.String("jwt-secret", "", "HMAC secret for dashboard session JWTs; required to enable the account/domains/rules/settings API (unset disables it)")
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

	// The account/domains/rules/settings dashboard API needs both a
	// database (it's the only durable store any of it has) and a JWT
	// secret (without one, sessions can't be signed at all). Requiring
	// both explicitly rather than falling back to a random secret means
	// a misconfigured deployment fails loudly at startup instead of
	// silently minting sessions no restart can verify.
	if *dbURL != "" && *jwtSecret != "" {
		issuer, err := auth.NewIssuer([]byte(*jwtSecret))
		if err != nil {
			log.Fatalf("hakaishield: %v", err)
		}
		accountStore := account.NewStore(db.DB)
		rulesStore := rules.NewStore(db.DB)
		settingsStore := settings.NewStore(db.DB)

		mux.HandleFunc("POST /api/v1/auth/signup", api.SignupHandler(accountStore))
		mux.HandleFunc("POST /api/v1/auth/signin", api.SigninHandler(accountStore, issuer))
		mux.HandleFunc("GET /api/v1/auth/me", api.MeHandler(accountStore, issuer))
		mux.HandleFunc("POST /api/v1/onboarding/complete", api.OnboardingCompleteHandler(accountStore, issuer))
		mux.HandleFunc("/api/v1/domains", api.DomainsHandler(issuer))
		mux.HandleFunc("GET /api/v1/rules", api.RulesListHandler(rulesStore, issuer))
		mux.HandleFunc("POST /api/v1/rules/custom", api.CreateRuleHandler(rulesStore, issuer))
		mux.HandleFunc("PUT /api/v1/rules/{id}/toggle", api.ToggleRuleHandler(rulesStore, issuer))
		mux.HandleFunc("/api/v1/settings/protection", api.ProtectionSettingsHandler(settingsStore, issuer))
		mux.HandleFunc("GET /api/v1/dashboard/top-offenders", api.TopOffendersHandler(store, issuer))
		mux.HandleFunc("GET /api/v1/dashboard/evidence-logs", api.EvidenceLogsHandler(store, issuer))
		log.Print("hakaishield: account/domains/rules/settings API enabled")
	} else {
		log.Print("hakaishield: -db-url and/or -jwt-secret not set, account/domains/rules/settings API disabled")
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
