package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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

	handler, err := newHandler(upstream.URL + "/base")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/items/a%2Fb?page=2", strings.NewReader("payload"))
	req.Header.Set("X-Test", "forwarded")
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
	handler, err := newHandler(upstream.URL)
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
	handler, err := newHandler(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/items", nil))
	if response.Code != http.StatusBadGateway {
		t.Fatalf("got status %d, want 502", response.Code)
	}
}

func TestInvalidUpstream(t *testing.T) {
	for _, raw := range []string{"", "/relative", "localhost:9000", "ftp://localhost", "http://", "http://%", "http://user:pass@localhost", "http://localhost/#fragment"} {
		t.Run(raw, func(t *testing.T) {
			if _, err := newHandler(raw); err == nil {
				t.Fatalf("accepted invalid upstream %q", raw)
			}
		})
	}
}
