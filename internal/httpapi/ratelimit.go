package httpapi

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// rateLimiter is a tiny per-IP token bucket suitable for protecting auth
// endpoints from credential stuffing and brute force. We deliberately keep
// it in-process: a single tarmy serve handles hundreds of players, and a
// distributed limiter (redis) is overkill for the MVP. The bucket parameters
// are passed in so the middleware can be tuned per route.
//
// State lifecycle:
//   - one bucket per source IP, lazily created on first hit
//   - background sweep every 5 minutes removes buckets idle > 30 minutes
//
// The 429 response includes a Retry-After hint computed from the refill rate.
type rateLimiter struct {
	capacity   float64
	refillRate float64 // tokens per second
	mu         sync.Mutex
	buckets    map[string]*bucket
	stop       chan struct{}
}

type bucket struct {
	tokens   float64
	lastSeen time.Time
}

func newRateLimiter(capacity int, refillPerSecond float64) *rateLimiter {
	rl := &rateLimiter{
		capacity:   float64(capacity),
		refillRate: refillPerSecond,
		buckets:    make(map[string]*bucket),
		stop:       make(chan struct{}),
	}
	go rl.gcLoop()
	return rl
}

// allow consumes one token for the given key. Returns true when the request
// is permitted, false (with the seconds to wait) when throttled.
func (rl *rateLimiter) allow(key string) (bool, time.Duration) {
	now := time.Now()
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b := rl.buckets[key]
	if b == nil {
		b = &bucket{tokens: rl.capacity, lastSeen: now}
		rl.buckets[key] = b
	} else {
		// Refill since last access. Capped at bucket capacity.
		elapsed := now.Sub(b.lastSeen).Seconds()
		b.tokens += elapsed * rl.refillRate
		if b.tokens > rl.capacity {
			b.tokens = rl.capacity
		}
		b.lastSeen = now
	}
	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}
	// How long until we have 1 token again?
	wait := time.Duration((1.0 - b.tokens) / rl.refillRate * float64(time.Second))
	if wait < time.Second {
		wait = time.Second
	}
	return false, wait
}

// gcLoop periodically removes idle buckets so memory does not grow without
// bound on long-running servers.
func (rl *rateLimiter) gcLoop() {
	t := time.NewTicker(5 * time.Minute)
	defer t.Stop()
	for {
		select {
		case <-rl.stop:
			return
		case now := <-t.C:
			rl.mu.Lock()
			for k, b := range rl.buckets {
				if now.Sub(b.lastSeen) > 30*time.Minute {
					delete(rl.buckets, k)
				}
			}
			rl.mu.Unlock()
		}
	}
}

// middleware returns chi-compatible middleware that gates each request by
// client IP. Use a small bucket (e.g. capacity=10, refill=0.2/s) for login /
// signup endpoints; about 10 attempts upfront then 1 per 5 seconds sustained.
func (rl *rateLimiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := clientIP(r)
		ok, wait := rl.allow(key)
		if !ok {
			w.Header().Set("Retry-After", formatSeconds(wait))
			http.Error(w, "rate limit exceeded; slow down", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP picks the source address for limiting. middleware.RealIP already
// rewrote RemoteAddr from X-Forwarded-For / X-Real-IP when those headers were
// trusted, so reading RemoteAddr here is correct. We strip any :port.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// formatSeconds renders a Retry-After header value in whole seconds (the spec
// also allows an HTTP-date, but seconds is simpler and universally supported).
func formatSeconds(d time.Duration) string {
	secs := int(d.Seconds())
	if secs < 1 {
		secs = 1
	}
	return strings.TrimSpace(itoa(secs))
}

// itoa avoids an strconv import just for the Retry-After header.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
