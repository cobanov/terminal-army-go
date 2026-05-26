// Package auth provides JWT helpers and the HTTP middleware that resolves
// the bearer token into a session record. Session storage and password
// hashing live in internal/svc (AuthService) to keep this package small and
// free of cycles.
package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/cobanov/terminal-army-go/internal/svc"
)

type ctxKey string

const (
	sessionCtxKey ctxKey = "session"
)

// TokenFromRequest pulls a bearer token from the Authorization header.
// It also accepts ?token= for the websocket / TUI fall-back.
func TokenFromRequest(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
	}
	if t := r.URL.Query().Get("token"); t != "" {
		return t
	}
	return ""
}

// Middleware enforces a valid bearer token and stashes the resolved session
// in the request context. The lookup is provided by AuthService.
func Middleware(app *svc.App) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok := TokenFromRequest(r)
			if tok == "" {
				http.Error(w, `{"error":"missing token"}`, http.StatusUnauthorized)
				return
			}
			sess, err := app.Auth.ResolveSession(r.Context(), tok)
			if err != nil || sess == nil {
				http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
				return
			}
			if sess.User != nil {
				app.Presence.Touch(sess.User.ID)
			}
			ctx := context.WithValue(r.Context(), sessionCtxKey, sess)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// SessionFromContext returns the request's session, if any.
func SessionFromContext(ctx context.Context) *svc.Session {
	s, _ := ctx.Value(sessionCtxKey).(*svc.Session)
	return s
}

// UserFromContext returns the authenticated user record, or nil.
func UserFromContext(ctx context.Context) *svc.User {
	s := SessionFromContext(ctx)
	if s == nil {
		return nil
	}
	return s.User
}

// UserIDFromContext returns the authenticated user id, or 0.
func UserIDFromContext(ctx context.Context) int64 {
	s := SessionFromContext(ctx)
	if s == nil {
		return 0
	}
	return s.UserID
}
