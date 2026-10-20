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
	rate        float64
	burst       float64
}

func newLimiter(rate, burst int) *limiter {
	return &limiter{buckets: make(map[string]bucket), now: time.Now, rate: float64(rate), burst: float64(burst)}
}

func (l *limiter) allow(key string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	if now.Sub(l.lastCleanup) >= time.Minute {
		for key, b := range l.buckets {
			idle := now.Sub(b.updated)
			if idle >= time.Minute && b.tokens+idle.Seconds()*l.rate >= l.burst {
				delete(l.buckets, key)
			}
		}
		l.lastCleanup = now
	}

	b, ok := l.buckets[key]
	if !ok {
		b = bucket{tokens: l.burst, updated: now}
	}
	b.tokens = math.Min(l.burst, b.tokens+now.Sub(b.updated).Seconds()*l.rate)
	b.updated = now
	var retry time.Duration
	if b.tokens >= 1 {
		b.tokens--
	} else {
		retry = time.Duration(math.Ceil((1 - b.tokens) / l.rate * float64(time.Second)))
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
