package httpapi

import (
	"net/http"
	"time"

	"github.com/cobanov/terminal-army-go/internal/svc"
	"github.com/cobanov/terminal-army-go/internal/ws"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func newRouter(app *svc.App, hub *ws.Hub) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(securityHeaders)

	r.Get("/healthz", healthHandler)
	r.Get("/readyz", readyHandler(app))

	mount(r, app, hub)
	return r
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func readyHandler(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := app.Pool.Ping(r.Context()); err != nil {
			http.Error(w, "db unreachable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	}
}

// securityHeaders applies a strict baseline of HTTP security headers on every
// response. The web surface is intentionally tiny so the CSP can be locked
// down hard: no scripts at all, styles only inline (the single embedded
// stylesheet), and no remote fetches. HSTS is only set on HTTPS requests so
// development over plain HTTP still works.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		h.Set("Content-Security-Policy",
			"default-src 'none'; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data:; "+
				"font-src 'self'; "+
				"form-action 'self'; "+
				"frame-ancestors 'none'; "+
				"base-uri 'none'")
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

// mount is implemented in routes.go so the route table is easy to scan.
var mount = func(r chi.Router, app *svc.App, hub *ws.Hub) {}
