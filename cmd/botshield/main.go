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

	"github.com/ToufiqQureshi/bot-shield/proxy"
)

func main() {
	addr := flag.String("addr", ":8080", "address to listen on")
	target := flag.String("target", "", "origin server to protect, e.g. https://example.com")
	certFile := flag.String("tls-cert", "", "TLS certificate file; enables TLS + JA4 fingerprinting")
	keyFile := flag.String("tls-key", "", "TLS private key file, required with -tls-cert")
	evidenceToken := flag.String("evidence-token", "", "bearer token for the per-request evidence endpoint; unset leaves the endpoint off")
	modeFlag := flag.String("mode", "enforce", `"enforce" acts on scores; "shadow" only records what it would have done`)
	flag.Parse()

	mode, err := proxy.ParseMode(*modeFlag)
	if err != nil {
		log.Fatalf("botshield: %v", err)
	}

	if *target == "" {
		log.Fatal("botshield: -target is required")
	}

	p, err := proxy.New(*target)
	if err != nil {
		log.Fatalf("botshield: %v", err)
	}

	// The challenge page/verify endpoints stay reachable directly too,
	// for manual testing.
	challenge, err := proxy.NewChallenge()
	if err != nil {
		log.Fatalf("botshield: %v", err)
	}

	// Guard is where scoring (ROADMAP item 5) actually acts: allow,
	// challenge, or block, instead of just labeling the request.
	stats := &proxy.Stats{Mode: mode}
	trail := proxy.NewTrail()
	guard := proxy.NewGuard(p, challenge, stats, trail, mode)

	mux := http.NewServeMux()
	mux.Handle("/__botshield/", challenge.Handler())
	mux.Handle("/api/v1/dashboard/stats", stats.Handler())
	mux.Handle("/", guard)

	// The evidence endpoint returns per-visitor fingerprints, so it
	// only exists once an operator has set a token for it. Left off, it
	// can't leak anything or tell a bot whether it's being flagged.
	if *evidenceToken != "" {
		mux.Handle("/api/v1/dashboard/evidence", trail.Handler(*evidenceToken))
	} else {
		log.Print("botshield: -evidence-token not set, evidence endpoint disabled")
	}

	// These timeouts stop a client that opens a connection and then
	// sends data slowly (or never) from holding it open forever.
	// ReadHeaderTimeout does double duty: net/http also uses it as the
	// TLS handshake deadline, so it covers a stalled handshake too.
	// Do not remove it thinking it is only about headers.
	srv := &http.Server{
		Handler:           mux,
		ConnContext:       proxy.ConnContext,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("botshield: %v", err)
	}

	// Fingerprinting a client's TLS handshake only works if bot-shield
	// itself terminates TLS. Without a cert, we still proxy plain HTTP
	// so local dev / testing keeps working without one.
	if *certFile != "" {
		cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
		if err != nil {
			log.Fatalf("botshield: loading TLS cert/key: %v", err)
		}
		tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}}
		ln = proxy.NewCaptureListener(ln, tlsConfig)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("botshield: listening on %s, protecting %s", *addr, *target)
		if mode == proxy.ModeShadow {
			log.Print("botshield: SHADOW MODE — scoring and recording only, NOTHING will be blocked or challenged")
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
