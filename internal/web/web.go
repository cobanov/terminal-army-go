// Package web mounts the small HTML surface for signup, login, and alliance
// management. The game itself is played from the TUI; the web side stays
// intentionally minimal.
//
// The handler set is deliberately small:
//
//	GET  /                    landing page
//	GET  /signup              signup form
//	POST /signup              create account, set session cookie
//	GET  /login               login form
//	POST /login               verify credentials, set session cookie
//	POST /logout              clear cookie + delete device session
//	GET  /alliance            list + create form (members area)
//	POST /alliance            create a new alliance
//	GET  /alliance/{id}       alliance detail with join/leave
//	POST /alliance/{id}/join  join the alliance
//	POST /alliance/{id}/leave leave the alliance
//	GET  /admin               admin dashboard (admin role only)
//	GET  /admin/users         paginated user table
//	POST /admin/users         promote / demote a user
//
// Every form embeds a CSRF token; every session cookie is HttpOnly,
// SameSite=Lax, and Secure when the request arrives over HTTPS.
package web

import (
	"io/fs"
	"net/http"

	"github.com/cobanov/terminal-army-go/internal/svc"
	"github.com/go-chi/chi/v5"
)

// Mount attaches the web routes under /. Auth is opportunistic: the session
// middleware reads the cookie on every request, but only specific routes
// require a logged-in user. POST endpoints that touch credentials are
// rate-limited per source IP.
func Mount(r chi.Router, app *svc.App) {
	// 10-attempt burst, 1 request per 5 seconds sustained. Same shape as the
	// API limiter so users hit similar walls on either surface.
	authThrottle := newWebRateLimiter(10, 0.2)

	r.Handle("/static/*", staticFileServer())
	r.HandleFunc("/install.sh", installScriptHandler)

	r.Group(func(r chi.Router) {
		r.Use(withSession(app))

		r.Get("/", indexHandler(app))

		r.Get("/signup", signupHandler(app))
		r.With(authThrottle.limitPOST).Post("/signup", signupHandler(app))

		r.Get("/login", loginHandler(app))
		r.With(authThrottle.limitPOST).Post("/login", loginHandler(app))

		r.Post("/logout", logoutHandler(app))

		// Explicit, CSRF-protected device-code approval. Both methods share a
		// handler: GET shows the confirmation, POST performs the bind.
		r.Get("/device/approve", deviceApproveHandler(app))
		r.With(authThrottle.limitPOST).Post("/device/approve", deviceApproveHandler(app))

		r.Route("/alliance", func(r chi.Router) {
			r.Use(requireLogin)
			r.Get("/", allianceListHandler(app))
			r.Post("/", allianceListHandler(app))
			r.Get("/{id}", allianceDetailHandler(app))
			r.Post("/{id}/join", allianceJoinHandler(app))
			r.Post("/{id}/leave", allianceLeaveHandler(app))
		})

		// Admin surface. Gated by role; non-admin sessions get 403 rather
		// than a redirect so the URL behaviour is honest.
		r.Route("/admin", func(r chi.Router) {
			r.Use(requireAdmin)
			r.Get("/", adminDashboardHandler(app))
			r.Get("/users", adminUsersHandler(app))
			r.Post("/users", adminUsersHandler(app))
		})
	})
}

func staticFileServer() http.Handler {
	staticRoot, err := fs.Sub(templateFS, "static")
	if err != nil {
		panic(err)
	}
	return http.StripPrefix("/static/", http.FileServer(http.FS(staticRoot)))
}

func installScriptHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := templateFS.ReadFile("static/install.sh")
	if err != nil {
		http.Error(w, "installer unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = w.Write(body)
}
