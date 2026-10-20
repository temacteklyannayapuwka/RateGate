package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

func newHandler(upstream string, rate, burst int, timeout time.Duration) (http.Handler, error) {
	target, err := url.Parse(upstream)
	if err != nil || target.Hostname() == "" || (target.Scheme != "http" && target.Scheme != "https") || target.Fragment != "" || target.User != nil {
		return nil, fmt.Errorf("UPSTREAM_URL must be an absolute HTTP or HTTPS URL without credentials or a fragment")
	}

	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(target)
			r.SetXForwarded()
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			if errors.Is(r.Context().Err(), context.Canceled) {
				return
			}
			log.Printf("upstream request failed: %v", err)
			var netErr net.Error
			if errors.Is(err, context.DeadlineExceeded) || errors.As(err, &netErr) && netErr.Timeout() {
				http.Error(w, "Gateway Timeout", http.StatusGatewayTimeout)
				return
			}
			http.Error(w, "Bad Gateway", http.StatusBadGateway)
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{\"status\":\"ok\"}\n"))
	})
	timedProxy := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		proxy.ServeHTTP(w, r.WithContext(ctx))
	})
	mux.Handle("/", newLimiter(rate, burst).middleware(timedProxy))
	return mux, nil
}
