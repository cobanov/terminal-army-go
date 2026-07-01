package httpapi

import (
	"net"
	"net/http"
	"strings"
)

// maxRequestBody caps the size of any request body the API/web will read,
// preventing a single client from exhausting memory with a huge upload.
const maxRequestBody = 1 << 20 // 1 MiB

// realIPFromTrustedProxy rewrites RemoteAddr from X-Forwarded-For / X-Real-IP
// ONLY when the direct peer is a loopback, private, or link-local address —
// i.e. a co-located reverse proxy. A directly-connected public client cannot
// spoof these headers to obtain a fresh per-IP rate-limit bucket, while a real
// proxy (Caddy/nginx on the same host or private network) still yields correct
// client IPs. This replaces chi's middleware.RealIP, which trusts the headers
// unconditionally.
func realIPFromTrustedProxy(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil && isTrustedProxy(host) {
			if ip := clientIPFromHeaders(r); ip != "" {
				r.RemoteAddr = net.JoinHostPort(ip, "0")
			}
		}
		next.ServeHTTP(w, r)
	})
}

func isTrustedProxy(host string) bool {
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()
}

func clientIPFromHeaders(r *http.Request) string {
	if v := strings.TrimSpace(r.Header.Get("X-Real-IP")); v != "" {
		return v
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[:i]) // original client is first
		}
		return strings.TrimSpace(xff)
	}
	return ""
}

// limitBody caps the request body size. MaxBytesReader also signals the
// ResponseWriter to close the connection once the limit is exceeded.
func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
		}
		next.ServeHTTP(w, r)
	})
}
