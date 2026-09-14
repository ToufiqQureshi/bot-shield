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
	flag.Parse()

	if *target == "" {
		log.Fatal("botshield: -target is required")
	}

	p, err := proxy.New(*target)
	if err != nil {
		log.Fatalf("botshield: %v", err)
	}

	srv := &http.Server{Handler: p, ConnContext: proxy.ConnContext}

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
