package main

import (
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	mux, err := newHandler(os.Getenv("UPSTREAM_URL"))
	if err != nil {
		log.Fatal(err)
	}

	server := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("RateGate listening on %s", server.Addr)
	log.Fatal(server.ListenAndServe())
}
