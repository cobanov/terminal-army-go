package web

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cobanov/terminal-army-go/internal/svc"
)

// Cookie names and constants. We keep two cookies:
//   - tarmy_session: the JWT, server resolves it to a Session on each request
//   - tarmy_csrf:    a random per-session token, double-submit pattern
//
// Both are HttpOnly except csrf which must be readable by the form-rendering
// path (it is also HttpOnly because the token is read server side and echoed
// into a hidden field). Server enforces SameSite=Lax + Secure when behind TLS.
const (
	sessionCookie = "tarmy_session"
	csrfCookie    = "tarmy_csrf"
	sessionMaxAge = 7 * 24 * 60 * 60 // 7 days, same as default JWT TTL
)

// ctxKey is a private context key type so we never collide with other packages.
type ctxKey int

const (
	ctxSession ctxKey = iota
	ctxCSRF
)

// sessionFromCtx fetches the resolved session from the request context. Returns
// nil when the user is not logged in.
func sessionFromCtx(ctx context.Context) *svc.Session {
	v, _ := ctx.Value(ctxSession).(*svc.Session)
	return v
}

// csrfFromCtx fetches the per-request CSRF token (the one to embed in forms).
func csrfFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(ctxCSRF).(string)
	return v
}

// withSession reads the session cookie, resolves it through AuthService, and
// stashes the result in the request context. Failed lookups are silently
// ignored so the public pages still render. Also ensures every request has a
// CSRF token cookie (issuing one if absent).
func withSession(app *svc.App) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
				sess, err := app.Auth.ResolveSession(ctx, c.Value)
				if err == nil && sess != nil {
					ctx = context.WithValue(ctx, ctxSession, sess)
				} else {
					// Stale or invalid token. Clear the cookie so the user is
					// not stuck in a half-logged-in loop.
					clearSessionCookie(w, r)
				}
			}

			csrf := ensureCSRF(w, r)
			ctx = context.WithValue(ctx, ctxCSRF, csrf)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// requireLogin redirects anonymous users to /login with a return URL.
func requireLogin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if sessionFromCtx(r.Context()) == nil {
			next := url.QueryEscape(r.URL.Path)
			http.Redirect(w, r, "/login?next="+next, http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// requireAdmin gates a handler chain on the admin role. Anonymous users get
// the standard login redirect; non-admin users get a 403 (we deliberately do
// not redirect them - they should not see the admin URL existed at all, but
// a non-200 is more honest than a silent 404).
func requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := sessionFromCtx(r.Context())
		if sess == nil || sess.User == nil {
			next := url.QueryEscape(r.URL.Path)
			http.Redirect(w, r, "/login?next="+next, http.StatusSeeOther)
			return
		}
		if sess.User.Role != "admin" {
			http.Error(w, "admin access required", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// checkCSRF compares the form value against the cookie value using constant
// time. Called from POST handlers. Returns true when the request is valid.
func checkCSRF(r *http.Request) bool {
	form := r.FormValue("csrf")
	if form == "" {
		return false
	}
	c, err := r.Cookie(csrfCookie)
	if err != nil || c.Value == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(form), []byte(c.Value)) == 1
}

// ensureCSRF returns the current CSRF token, generating and setting a cookie
// if none is present. Token is 32 hex chars (16 bytes random).
func ensureCSRF(w http.ResponseWriter, r *http.Request) string {
	if c, err := r.Cookie(csrfCookie); err == nil && len(c.Value) == 32 {
		return c.Value
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	tok := hex.EncodeToString(buf)
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookie,
		Value:    tok,
		Path:     "/",
		MaxAge:   sessionMaxAge,
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
	return tok
}

// setSessionCookie issues the JWT cookie after a successful login or signup.
func setSessionCookie(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   sessionMaxAge,
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
}

// clearSessionCookie expires the session cookie. Used by logout and stale
// session cleanup.
func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0),
	})
}

// isHTTPS detects HTTPS even when a reverse proxy is in front. We check the
// request itself plus the standard X-Forwarded-Proto header.
func isHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		return true
	}
	return false
}

// safeRedirect picks a safe in-app path from the ?next= query, defaulting to
// dflt if the value is missing, external, or otherwise unsafe.
func safeRedirect(r *http.Request, dflt string) string {
	next := r.URL.Query().Get("next")
	if next == "" {
		next = r.FormValue("next")
	}
	if next == "" {
		return dflt
	}
	// Must be a path that starts with / and is not a protocol-relative URL.
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") {
		return dflt
	}
	if _, err := url.Parse(next); err != nil {
		return dflt
	}
	return next
}

// sessionTokenFromRequest returns the raw session cookie value, used by logout
// to call AuthService.Logout against the JWT.
func sessionTokenFromRequest(r *http.Request) (string, error) {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return "", err
	}
	if c.Value == "" {
		return "", errors.New("empty session cookie")
	}
	return c.Value, nil
}
