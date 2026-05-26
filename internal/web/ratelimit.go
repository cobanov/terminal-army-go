package web

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// webRateLimiter mirrors the API limiter in internal/httpapi but lives in
// this package so the web surface stays self-contained. We protect signup
// and login POST routes against credential-stuffing scripts that target the
// browser flow.
type webRateLimiter struct {
	capacity   float64
	refillRate float64
	mu         sync.Mutex
	buckets    map[string]*wbucket
}

type wbucket struct {
	tokens   float64
	lastSeen time.Time
}

func newWebRateLimiter(capacity int, refillPerSecond float64) *webRateLimiter {
	rl := &webRateLimiter{
		capacity:   float64(capacity),
		refillRate: refillPerSecond,
		buckets:    make(map[string]*wbucket),
	}
	go rl.gcLoop()
	return rl
}

func (rl *webRateLimiter) allow(key string) (bool, time.Duration) {
	now := time.Now()
	rl.mu.Lock()
	defer rl.mu.Unlock()
	b := rl.buckets[key]
	if b == nil {
		b = &wbucket{tokens: rl.capacity, lastSeen: now}
		rl.buckets[key] = b
	} else {
		b.tokens += now.Sub(b.lastSeen).Seconds() * rl.refillRate
		if b.tokens > rl.capacity {
			b.tokens = rl.capacity
		}
		b.lastSeen = now
	}
	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}
	wait := time.Duration((1.0 - b.tokens) / rl.refillRate * float64(time.Second))
	if wait < time.Second {
		wait = time.Second
	}
	return false, wait
}

func (rl *webRateLimiter) gcLoop() {
	t := time.NewTicker(5 * time.Minute)
	defer t.Stop()
	for now := range t.C {
		rl.mu.Lock()
		for k, b := range rl.buckets {
			if now.Sub(b.lastSeen) > 30*time.Minute {
				delete(rl.buckets, k)
			}
		}
		rl.mu.Unlock()
	}
}

// limitPOST applies the bucket only to mutating requests. GET pages stay
// unthrottled so a casual visitor browsing the signup form does not consume
// tokens.
func (rl *webRateLimiter) limitPOST(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			next.ServeHTTP(w, r)
			return
		}
		ok, wait := rl.allow(clientIP(r))
		if !ok {
			w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())))
			http.Error(w, "rate limit exceeded; slow down", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
