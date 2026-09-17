// Command botshield runs the bot-shield reverse proxy.
package main

import (
	"context"
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
	"github.com/ToufiqQureshi/bot-shield/pkg/tenant"
)

func main() {
	addr := flag.String("addr", ":8080", "address to listen on")
	target := flag.String("target", "", "origin server to protect, e.g. https://example.com")
	certFile := flag.String("tls-cert", "", "TLS certificate file; enables TLS + JA4 fingerprinting")
	keyFile := flag.String("tls-key", "", "TLS private key file, required with -tls-cert")
	evidenceToken := flag.String("evidence-token", "", "bearer token for the per-request evidence endpoint; unset leaves the endpoint off")
	modeFlag := flag.String("mode", "enforce", `"enforce" acts on scores; "shadow" only records what it would have done`)
	flag.Parse()

	mode, err := config.ParseMode(*modeFlag)
	if err != nil {
		log.Fatalf("botshield: %v", err)
	}

	if *target == "" {
		log.Fatal("botshield: -target is required")
	}

	challengeHandler, err := challenge.NewChallenge()
	if err != nil {
		log.Fatalf("botshield: %v", err)
	}

	store := tenant.NewStore()
	originProxy, err := core.NewOriginProxy(*target)
	if err != nil {
		log.Fatalf("botshield: creating origin proxy: %v", err)
	}

	err = store.Add("default", tenant.TenantConfig{
		Target:        *target,
		Mode:          mode,
		EvidenceToken: *evidenceToken,
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
		Handler:           mux,
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
