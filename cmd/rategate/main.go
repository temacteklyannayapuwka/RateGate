package main

import (
	"log"
	"net/http"
	"os"
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

	log.Printf("RateGate listening on %s", server.Addr)
	log.Fatal(server.ListenAndServe())
}
