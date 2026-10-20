package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		log.Fatal(err)
	}
	mux, err := newHandler(cfg.upstream, cfg.rate, cfg.burst, cfg.upstreamTimeout)
	if err != nil {
		log.Fatal(err)
	}

	server := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      cfg.upstreamTimeout + 5*time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("RateGate listening on %s", server.Addr)
	if err := serve(ctx, server, listener, cfg.upstreamTimeout+5*time.Second); err != nil {
		log.Fatal(err)
	}
	log.Print("RateGate stopped")
}
