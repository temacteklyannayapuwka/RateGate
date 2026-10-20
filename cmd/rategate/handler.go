package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func newHandler(upstream string, rate, burst int) (http.Handler, error) {
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
			log.Printf("upstream request failed: %v", err)
			http.Error(w, "Bad Gateway", http.StatusBadGateway)
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{\"status\":\"ok\"}\n"))
	})
	mux.Handle("/", newLimiter(rate, burst).middleware(proxy))
	return mux, nil
}
