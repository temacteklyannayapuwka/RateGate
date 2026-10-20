package main

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type bucket struct {
	tokens  float64
	updated time.Time
}

type limiter struct {
	mu          sync.Mutex
	buckets     map[string]bucket
	now         func() time.Time
	lastCleanup time.Time
}

func newLimiter() *limiter {
	return &limiter{buckets: make(map[string]bucket), now: time.Now}
}

func (l *limiter) allow(key string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	if now.Sub(l.lastCleanup) >= time.Minute {
		for key, b := range l.buckets {
			if now.Sub(b.updated) >= time.Minute {
				delete(l.buckets, key)
			}
		}
		l.lastCleanup = now
	}

	b, ok := l.buckets[key]
	if !ok {
		b = bucket{tokens: 10, updated: now}
	}
	b.tokens = math.Min(10, b.tokens+now.Sub(b.updated).Seconds()*10)
	b.updated = now
	var retry time.Duration
	if b.tokens >= 1 {
		b.tokens--
	} else {
		retry = time.Duration(math.Ceil((1 - b.tokens) / 10 * float64(time.Second)))
	}
	l.buckets[key] = b
	return retry
}

func (l *limiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimSpace(r.Header.Get("X-API-Key"))
		if key == "" || len(key) > 256 {
			http.Error(w, "X-API-Key must contain 1 to 256 bytes", http.StatusBadRequest)
			return
		}
		if retry := l.allow(key); retry > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(retry.Seconds()))))
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
