package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestUpstreamTimeout(t *testing.T) {
	canceled := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		close(canceled)
	}))
	defer upstream.Close()
	handler, err := newHandler(upstream.URL, defaultRate, defaultBurst, 100*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/slow", nil)
	req.Header.Set("X-API-Key", "test-client")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Code != http.StatusGatewayTimeout || response.Body.String() != "Gateway Timeout\n" {
		t.Fatalf("unexpected timeout response: %d %s", response.Code, response.Body)
	}
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("upstream request was not canceled")
	}
}

func TestClientCancellationReachesUpstream(t *testing.T) {
	started := make(chan struct{})
	canceled := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
		close(canceled)
	}))
	defer upstream.Close()
	handler, err := newHandler(upstream.URL, defaultRate, defaultBurst, defaultUpstreamTimeout)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/slow", nil).WithContext(ctx)
	req.Header.Set("X-API-Key", "test-client")
	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(httptest.NewRecorder(), req)
		close(done)
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("upstream request did not start")
	}
	cancel()
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("client cancellation did not reach upstream")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("proxy did not return after cancellation")
	}
}

func TestProxyForwardsRequestAndResponse(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		if r.Method != http.MethodPost || r.URL.EscapedPath() != "/base/items/a%2Fb" || r.URL.RawQuery != "page=2" || string(body) != "payload" {
			t.Errorf("unexpected upstream request: %s %s body=%q", r.Method, r.URL, body)
		}
		if r.Header.Get("X-Test") != "forwarded" {
			t.Error("request header was not forwarded")
		}
		if strings.Contains(r.Header.Get("X-Forwarded-For"), "spoofed") {
			t.Error("untrusted forwarded header was preserved")
		}
		w.Header().Set("X-Upstream", "received")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("created"))
	}))
	defer upstream.Close()

	handler, err := newHandler(upstream.URL+"/base", defaultRate, defaultBurst, defaultUpstreamTimeout)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/items/a%2Fb?page=2", strings.NewReader("payload"))
	req.Header.Set("X-Test", "forwarded")
	req.Header.Set("X-API-Key", "test-client")
	req.Header.Set("X-Forwarded-For", "spoofed")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)

	if response.Code != http.StatusCreated || response.Body.String() != "created" || response.Header().Get("X-Upstream") != "received" {
		t.Fatalf("unexpected proxy response: %d %s %v", response.Code, response.Body, response.Header())
	}
}

func TestHealthDoesNotCallUpstream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("health request reached upstream")
	}))
	defer upstream.Close()
	handler, err := newHandler(upstream.URL, defaultRate, defaultBurst, defaultUpstreamTimeout)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health", nil))
	if response.Code != http.StatusOK || response.Body.String() != "{\"status\":\"ok\"}\n" {
		t.Fatalf("unexpected health response: %d %s", response.Code, response.Body)
	}
}

func TestUnavailableUpstream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	upstream.Close()
	handler, err := newHandler(upstream.URL, defaultRate, defaultBurst, defaultUpstreamTimeout)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/items", nil)
	req.Header.Set("X-API-Key", "test-client")
	handler.ServeHTTP(response, req)
	if response.Code != http.StatusBadGateway {
		t.Fatalf("got status %d, want 502", response.Code)
	}
}

func TestInvalidUpstream(t *testing.T) {
	for _, raw := range []string{"", "/relative", "localhost:9000", "ftp://localhost", "http://", "http://%", "http://user:pass@localhost", "http://localhost/#fragment"} {
		t.Run(raw, func(t *testing.T) {
			if _, err := newHandler(raw, defaultRate, defaultBurst, defaultUpstreamTimeout); err == nil {
				t.Fatalf("accepted invalid upstream %q", raw)
			}
		})
	}
}
