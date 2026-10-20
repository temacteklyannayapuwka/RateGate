package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"
)

type trackedListener struct {
	net.Listener
	closed chan struct{}
	once   sync.Once
}

func (l *trackedListener) Close() error {
	err := l.Listener.Close()
	l.once.Do(func() { close(l.closed) })
	return err
}

func TestShutdownDrainsActiveRequest(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	tracked := &trackedListener{Listener: listener, closed: make(chan struct{})}
	started := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	finish := func() { releaseOnce.Do(func() { close(release) }) }
	defer finish()
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		w.Write([]byte("completed"))
	})}
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stopped := make(chan error, 1)
	go func() { stopped <- serve(ctx, server, tracked, 2*time.Second) }()
	response := make(chan error, 1)
	go func() {
		client := &http.Client{Timeout: 3 * time.Second}
		r, err := client.Get("http://" + listener.Addr().String())
		if err != nil {
			response <- err
			return
		}
		defer r.Body.Close()
		body, err := io.ReadAll(r.Body)
		if err == nil && (r.StatusCode != http.StatusOK || string(body) != "completed") {
			err = fmt.Errorf("unexpected response: %d %q", r.StatusCode, body)
		}
		response <- err
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("request did not start")
	}
	cancel()
	select {
	case <-tracked.closed:
	case <-time.After(time.Second):
		t.Fatal("listener was not closed")
	}
	select {
	case err := <-stopped:
		t.Fatalf("stopped before active request finished: %v", err)
	default:
	}
	conn, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
	if err == nil {
		conn.Close()
		t.Fatal("accepted a new connection during shutdown")
	}
	finish()
	select {
	case err := <-response:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("active request did not complete")
	}
	select {
	case err := <-stopped:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not stop")
	}
}

func TestShutdownDeadlineClosesActiveConnection(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	canceled := make(chan struct{})
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
		close(canceled)
	})}
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stopped := make(chan error, 1)
	go func() { stopped <- serve(ctx, server, listener, 50*time.Millisecond) }()
	clientDone := make(chan struct{})
	go func() {
		defer close(clientDone)
		client := &http.Client{Timeout: 3 * time.Second}
		if r, err := client.Get("http://" + listener.Addr().String()); err == nil {
			r.Body.Close()
		}
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("request did not start")
	}
	cancel()
	select {
	case err := <-stopped:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("got %v, want shutdown deadline error", err)
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown did not respect deadline")
	}
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("active request was not canceled")
	}
	select {
	case <-clientDone:
	case <-time.After(time.Second):
		t.Fatal("client connection was not closed")
	}
}

func TestServeReportsListenerError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	listener.Close()
	if err := serve(context.Background(), &http.Server{}, listener, time.Second); err == nil {
		t.Fatal("listener error was not returned")
	}
}
