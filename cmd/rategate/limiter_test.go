package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLimiterRefillAndIndependentKeys(t *testing.T) {
	now := time.Unix(1000, 0)
	l := newLimiter(defaultRate, defaultBurst)
	l.now = func() time.Time { return now }
	for i := 0; i < 10; i++ {
		if retry := l.allow("alice"); retry != 0 {
			t.Fatalf("request %d rejected", i+1)
		}
	}
	if retry := l.allow("alice"); retry != 100*time.Millisecond {
		t.Fatalf("retry = %v, want 100ms", retry)
	}
	if retry := l.allow("bob"); retry != 0 {
		t.Fatal("another key shared the exhausted budget")
	}
	now = now.Add(50 * time.Millisecond)
	if retry := l.allow("alice"); retry != 50*time.Millisecond {
		t.Fatalf("retry = %v, want 50ms", retry)
	}
	now = now.Add(50 * time.Millisecond)
	if retry := l.allow("alice"); retry != 0 {
		t.Fatal("refilled token was not available")
	}
	now = now.Add(time.Minute)
	l.allow("carol")
	if _, exists := l.buckets["alice"]; exists {
		t.Fatal("idle key was not removed")
	}
	for i := 0; i < 10; i++ {
		if retry := l.allow("alice"); retry != 0 {
			t.Fatal("idle bucket did not recover its burst")
		}
	}
	if l.allow("alice") == 0 {
		t.Fatal("bucket exceeded burst capacity")
	}
}

func TestLimiterConcurrentRequests(t *testing.T) {
	l := newLimiter(defaultRate, defaultBurst)
	l.now = func() time.Time { return time.Unix(1000, 0) }
	var accepted atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if l.allow("same-client") == 0 {
				accepted.Add(1)
			}
		}()
	}
	wg.Wait()
	if accepted.Load() != 10 {
		t.Fatalf("accepted %d simultaneous requests, want 10", accepted.Load())
	}
}

func TestCustomLimiterRateAndBurst(t *testing.T) {
	now := time.Unix(1000, 0)
	l := newLimiter(4, 3)
	l.now = func() time.Time { return now }
	for i := 0; i < 3; i++ {
		if l.allow("client") != 0 {
			t.Fatal("custom burst rejected too early")
		}
	}
	if retry := l.allow("client"); retry != 250*time.Millisecond {
		t.Fatalf("retry = %v, want 250ms", retry)
	}
	now = now.Add(125 * time.Millisecond)
	if retry := l.allow("client"); retry != 125*time.Millisecond {
		t.Fatalf("retry = %v, want 125ms", retry)
	}
	now = now.Add(125 * time.Millisecond)
	if l.allow("client") != 0 || l.allow("client") == 0 {
		t.Fatal("custom rate did not replenish exactly one token")
	}
}

func TestCleanupPreservesPartiallyRefilledBucket(t *testing.T) {
	now := time.Unix(1000, 0)
	l := newLimiter(1, 100)
	l.now = func() time.Time { return now }
	for i := 0; i < 100; i++ {
		l.allow("client")
	}
	now = now.Add(time.Minute)
	l.allow("another-client")
	for i := 0; i < 60; i++ {
		if l.allow("client") != 0 {
			t.Fatal("refilled token rejected")
		}
	}
	if l.allow("client") == 0 {
		t.Fatal("cleanup reset a partially refilled budget")
	}
}

func TestRateLimitedProxy(t *testing.T) {
	var reached atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()
	handler, err := newHandler(upstream.URL, defaultRate, defaultBurst, defaultUpstreamTimeout)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"", "   ", strings.Repeat("a", 257)} {
		req := httptest.NewRequest(http.MethodGet, "/items", nil)
		req.Header.Set("X-API-Key", key)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid key returned %d", response.Code)
		}
	}
	for i := 0; i < 15; i++ {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health", nil))
		if response.Code != http.StatusOK {
			t.Fatalf("health returned %d", response.Code)
		}
	}
	if reached.Load() != 0 {
		t.Fatal("invalid keys or health checks reached upstream")
	}
}

func TestLimiterMiddlewareRejectsExhaustedKey(t *testing.T) {
	l := newLimiter(defaultRate, defaultBurst)
	l.now = func() time.Time { return time.Unix(1000, 0) }
	var reached int
	handler := l.middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached++
		w.WriteHeader(http.StatusNoContent)
	}))
	for i := 0; i < 11; i++ {
		req := httptest.NewRequest(http.MethodGet, "/items", nil)
		req.Header.Set("X-API-Key", " alice ")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		if i < 10 && response.Code != http.StatusNoContent {
			t.Fatalf("allowed request returned %d", response.Code)
		}
		if i == 10 && (response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") != "1") {
			t.Fatalf("unexpected rejection: %d %v", response.Code, response.Header())
		}
	}
	if reached != 10 {
		t.Fatalf("upstream received %d requests, want 10", reached)
	}
}
