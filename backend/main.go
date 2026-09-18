// Command botshield runs the bot-shield reverse proxy.
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

	"github.com/ToufiqQureshi/bot-shield/pkg/api"
	"github.com/ToufiqQureshi/bot-shield/pkg/challenge"
	"github.com/ToufiqQureshi/bot-shield/pkg/config"
	"github.com/ToufiqQureshi/bot-shield/pkg/core"
	"github.com/ToufiqQureshi/bot-shield/pkg/db"
	"github.com/ToufiqQureshi/bot-shield/pkg/observability"
	"github.com/ToufiqQureshi/bot-shield/pkg/signals"
	"github.com/ToufiqQureshi/bot-shield/pkg/tenant"

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
	deceptionFlag := flag.Bool("deception", false, "enable deception mode (forwards high-confidence bots to origin with X-BotShield-Decision: deceive instead of 403)")
	redisURL := flag.String("redis-url", "redis://localhost:6379", "Redis connection URL for distributed rate limiting")
	dbURL := flag.String("db-url", "", "PostgreSQL URL for Supabase integration (e.g. postgres://user:pass@host:5432/db)")
	flag.Parse()

	mode, err := config.ParseMode(*modeFlag)
	if err != nil {
		log.Fatalf("botshield: %v", err)
	}

	// SENTRY_DSN is an env var, not a flag: flags show up in `ps aux`
	// output on shared hosts, which a DSN (while not a secret that
	// grants access to customer data) still has no reason to leak into.
	if err := observability.Init(os.Getenv("SENTRY_DSN")); err != nil {
		log.Printf("botshield: warning: sentry init failed: %v", err)
	}
	defer sentry.Flush(2 * time.Second)

	if *target == "" {
		log.Fatal("botshield: -target is required")
	}

	secret := []byte(*challengeSecret)
	if len(secret) == 0 {
		// Fallback to random if not provided, sufficient for single-node.
		secret = make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			log.Fatalf("botshield: generating random secret: %v", err)
		}
	}

	challengeHandler, err := challenge.NewChallenge(secret)
	if err != nil {
		log.Fatalf("botshield: %v", err)
	}

	// Initialize Redis for global rate limiting
	opt, err := redis.ParseURL(*redisURL)
	if err != nil {
		log.Fatalf("botshield: invalid redis url: %v", err)
	}
	rdb := redis.NewClient(opt)
	ctxRdb, cancelRdb := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelRdb()
	if err := rdb.Ping(ctxRdb).Err(); err != nil {
		log.Printf("botshield: warning: could not connect to redis at %s: %v (falling back to open)", *redisURL, err)
	} else {
		log.Printf("botshield: connected to redis at %s", *redisURL)
	}
	signals.InitRedis(rdb)

	// Start dynamic JA4 synchronization from Redis
	signals.StartJA4Sync(context.Background(), rdb)

	if *dbURL != "" {
		if err := db.Init(*dbURL); err != nil {
			log.Fatalf("botshield: initializing postgres db: %v", err)
		}
		log.Printf("botshield: connected to postgres at %s", *dbURL)
	}

	store := tenant.NewStore()
	store.ProxyFactory = core.NewOriginProxy // Wire up proxy creation for lazy-loading tenants

	originProxy, err := core.NewOriginProxy(*target)
	if err != nil {
		log.Fatalf("botshield: creating origin proxy: %v", err)
	}

	err = store.Add("default", tenant.TenantConfig{
		Target:        *target,
		Mode:          mode,
		EvidenceToken: *evidenceToken,
		Deception:     *deceptionFlag,
	}, []string{"*"}, originProxy)

	if err != nil {
		log.Fatalf("botshield: provisioning default tenant: %v", err)
	}

	guard := core.NewGuard(store, challengeHandler)

	mux := http.NewServeMux()
	mux.Handle("/__botshield/", challengeHandler.Handler())
	mux.Handle("/api/v1/dashboard/stats", api.DashboardStatsHandler(store))
	mux.Handle("/", guard)

	if *evidenceToken != "" {
		mux.Handle("/api/v1/dashboard/evidence", api.DashboardEvidenceHandler(store))
		// mux.Handle("/api/v1/dashboard/top-offenders", api.DashboardTopOffendersHandler(store))
		// mux.Handle("/api/v1/dashboard/export", api.DashboardExportHandler(store))
	} else {
		log.Print("botshield: -evidence-token not set, evidence endpoint disabled")
	}

	srv := &http.Server{
		Handler:           observability.Middleware(mux),
		ConnContext:       core.ConnContext,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("botshield: %v", err)
	}

	if *certFile != "" {
		cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
		if err != nil {
			log.Fatalf("botshield: loading TLS cert/key: %v", err)
		}
		tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}}
		ln = core.NewCaptureListener(ln, tlsConfig)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("botshield: listening on %s, protecting %s", *addr, *target)
		if mode == config.ModeShadow {
			log.Print("botshield: SHADOW MODE - scoring and recording only, NOTHING will be blocked or challenged")
		}
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Fatalf("botshield: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("botshield: shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("botshield: shutdown error: %v", err)
	}
}
