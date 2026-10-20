package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"
)

func serve(ctx context.Context, server *http.Server, listener net.Listener, grace time.Duration) error {
	result := make(chan error, 1)
	go func() {
		result <- server.Serve(listener)
	}()

	select {
	case err := <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}

	log.Print("RateGate shutting down; waiting for active requests")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), grace)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		server.Close()
		<-result
		return fmt.Errorf("server shutdown: %w", err)
	}
	if err := <-result; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
