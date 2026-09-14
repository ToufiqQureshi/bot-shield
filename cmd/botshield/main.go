// Command botshield runs the bot-shield reverse proxy.
package main

import (
	"context"
	"flag"
	"log"
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
	flag.Parse()

	if *target == "" {
		log.Fatal("botshield: -target is required")
	}

	p, err := proxy.New(*target)
	if err != nil {
		log.Fatalf("botshield: %v", err)
	}

	srv := &http.Server{Addr: *addr, Handler: p}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("botshield: listening on %s, protecting %s", *addr, *target)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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
