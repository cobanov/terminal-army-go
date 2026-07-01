package web

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/cobanov/terminal-army-go/internal/svc"
	"github.com/cobanov/terminal-army-go/internal/version"
	"github.com/go-chi/chi/v5"
)

// displayBuildDate renders the ldflags-injected build timestamp for humans.
// Falls back to the raw value (or "unknown") when it isn't RFC3339.
func displayBuildDate(raw string) string {
	if raw == "" || raw == "unknown" {
		return "unknown"
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.Format("2006-01-02 15:04 UTC")
	}
	return raw
}

// baseView builds the layout envelope every handler shares: title, current
// user (if any), CSRF token, public URL, current time. Page-specific fields
// are populated by the caller.
func baseView(app *svc.App, r *http.Request, title string) viewData {
	v := viewData{
		Title:      title,
		CSRF:       csrfFromCtx(r.Context()),
		PublicURL:  app.Cfg.PublicURL,
		Now:        time.Now().UTC(),
		ShellClass: "dash-shell",
		Version:    version.Version,
		BuildDate:  displayBuildDate(version.Date),
	}
	if s := sessionFromCtx(r.Context()); s != nil {
		v.User = s.User
	}
	return v
}

// indexHandler is the landing page. Public; renders the same shell whether or
// not the user is logged in (the layout swaps the nav).
func indexHandler(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		view := baseView(app, r, "home")
		view.ShellClass = "landing-shell"
		if rows, err := app.Leaderboard.Top(r.Context(), 5); err == nil {
			view.Leaderboard = rows
		}
		writePage(w, "index", view)
	}
}

// signupHandler renders the signup form on GET and creates a user on POST.
// On success the JWT is set as a cookie and the user is redirected to the
// alliance lobby (the only meaningful web destination).
func signupHandler(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		view := baseView(app, r, "sign up")
		view.ShellClass = "login-shell"
		code := authCodeFromRequest(r)
		view.Form = map[string]string{"code": code}

		// Already logged in: never bind a device code silently on GET. Send
		// the user to the explicit, CSRF-protected approval page instead
		// (prevents one-click device-code phishing). Without a code, go to
		// the alliance lobby.
		if view.User != nil {
			if code != "" {
				http.Redirect(w, r, "/device/approve?code="+url.QueryEscape(code), http.StatusSeeOther)
				return
			}
			http.Redirect(w, r, "/alliance", http.StatusSeeOther)
			return
		}

		if r.Method == http.MethodGet {
			writePage(w, "signup", view)
			return
		}

		if !checkCSRF(r) {
			view.Error = "invalid form token, please retry"
			writePage(w, "signup", view)
			return
		}

		username := r.FormValue("username")
		email := r.FormValue("email")
		password := r.FormValue("password")
		view.Form = map[string]string{"username": username, "email": email, "code": code}

		res, err := app.Auth.Register(r.Context(), username, email, password)
		if err != nil {
			view.Error = friendlyAuthError(err)
			writePage(w, "signup", view)
			return
		}

		setSessionCookie(w, r, res.Token)
		// Confirm device-code linking on a dedicated CSRF-protected page rather
		// than binding as a side effect of signup, so a "?code=" planted by a
		// third party can never be linked without an explicit approval click.
		if code != "" {
			http.Redirect(w, r, "/device/approve?code="+url.QueryEscape(code), http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, safeRedirect(r, "/alliance"), http.StatusSeeOther)
	}
}

// loginHandler renders the login form and processes a submission. On success
// the JWT cookie is set and the user is redirected to /alliance (or to the
// ?next= return URL when present).
func loginHandler(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		view := baseView(app, r, "log in")
		view.ShellClass = "login-shell"
		code := authCodeFromRequest(r)
		view.Form = map[string]string{"code": code}

		if view.User != nil {
			if code != "" {
				http.Redirect(w, r, "/device/approve?code="+url.QueryEscape(code), http.StatusSeeOther)
				return
			}
			http.Redirect(w, r, safeRedirect(r, "/alliance"), http.StatusSeeOther)
			return
		}

		if r.Method == http.MethodGet {
			writePage(w, "login", view)
			return
		}

		if !checkCSRF(r) {
			view.Error = "invalid form token, please retry"
			writePage(w, "login", view)
			return
		}

		username := r.FormValue("username")
		password := r.FormValue("password")
		view.Form = map[string]string{"username": username, "code": code}

		res, err := app.Auth.Login(r.Context(), username, password)
		if err != nil {
			view.Error = friendlyAuthError(err)
			writePage(w, "login", view)
			return
		}

		setSessionCookie(w, r, res.Token)
		if code != "" {
			http.Redirect(w, r, "/device/approve?code="+url.QueryEscape(code), http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, safeRedirect(r, "/alliance"), http.StatusSeeOther)
	}
}

// deviceApproveHandler is the single, explicit chokepoint for linking a CLI
// device-auth code to the current account. GET renders a confirmation page
// showing the code; POST (CSRF-checked) performs the bind. Binding never
// happens as a side effect of a GET or of signup/login, which prevents a
// third-party-planted "?code=" from hijacking a session with one click.
func deviceApproveHandler(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		view := baseView(app, r, "link device")
		view.ShellClass = "login-shell"
		code := authCodeFromRequest(r)

		// Must be logged in to approve. Bounce through login, preserving the
		// code so the user lands back here afterwards.
		if view.User == nil {
			http.Redirect(w, r, "/login?code="+url.QueryEscape(code), http.StatusSeeOther)
			return
		}
		if code == "" {
			http.Redirect(w, r, "/alliance", http.StatusSeeOther)
			return
		}
		view.Form = map[string]string{"code": code}

		if r.Method == http.MethodGet {
			writePage(w, "device_approve", view)
			return
		}

		if !checkCSRF(r) {
			view.Error = "invalid form token, please retry"
			writePage(w, "device_approve", view)
			return
		}
		tok, err := sessionTokenFromRequest(r)
		if err != nil || !app.Auth.BindDeviceAuth(r.Context(), code, tok, view.User.ID) {
			view.Error = "auth code invalid or expired; restart tarmy in terminal"
			writePage(w, "device_approve", view)
			return
		}
		view.Flash = "authentication complete"
		writePage(w, "terminal_success", view)
	}
}

// logoutHandler clears the session cookie and (best-effort) deletes the
// matching device session row. POST-only so a stray GET cannot log a user out.
func logoutHandler(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !checkCSRF(r) {
			http.Error(w, "invalid form token", http.StatusForbidden)
			return
		}
		if tok, err := sessionTokenFromRequest(r); err == nil {
			_ = app.Auth.Logout(r.Context(), tok)
		}
		clearSessionCookie(w, r)
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

// allianceListHandler renders the alliance lobby: every alliance with member
// counts, the user's current alliance (if any), and a "found an alliance"
// form. POST on the same path creates a new alliance.
func allianceListHandler(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		view := baseView(app, r, "alliances")
		user := view.User
		if user == nil {
			http.Redirect(w, r, "/login?next=%2Falliance", http.StatusSeeOther)
			return
		}

		list, err := app.Alliance.List(r.Context())
		if err != nil {
			view.Error = "could not load alliances"
			writePage(w, "alliance_list", view)
			return
		}
		view.Alliances = list
		view.Current = lookupCurrentAlliance(r, app, user.ID)

		if r.Method == http.MethodGet {
			writePage(w, "alliance_list", view)
			return
		}

		if !checkCSRF(r) {
			view.Error = "invalid form token, please retry"
			writePage(w, "alliance_list", view)
			return
		}

		tag := r.FormValue("tag")
		name := r.FormValue("name")
		desc := r.FormValue("description")
		view.Form = map[string]string{"tag": tag, "name": name, "description": desc}

		created, err := app.Alliance.Create(r.Context(), user.ID, tag, name, desc)
		if err != nil {
			view.Error = "could not create alliance; check the tag and name and try again"
			writePage(w, "alliance_list", view)
			return
		}
		http.Redirect(w, r, "/alliance/"+strconv.FormatInt(created.ID, 10), http.StatusSeeOther)
	}
}

// allianceDetailHandler renders a single alliance with a join or leave button
// depending on the user's membership state.
func allianceDetailHandler(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		view := baseView(app, r, "alliance")
		user := view.User
		if user == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		a, err := app.Alliance.Get(r.Context(), id)
		if err != nil {
			if errors.Is(err, svc.ErrNotFound) {
				http.NotFound(w, r)
				return
			}
			view.Error = "could not load alliance"
			writePage(w, "alliance_detail", view)
			return
		}
		view.Alliance = a
		view.Current = lookupCurrentAlliance(r, app, user.ID)
		view.IsMember = view.Current != nil && view.Current.ID == a.ID
		view.IsFounder = a.OwnerUserID == user.ID

		writePage(w, "alliance_detail", view)
	}
}

// allianceJoinHandler joins the current user to the alliance keyed by URL id.
func allianceJoinHandler(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := sessionFromCtx(r.Context())
		if user == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		if !checkCSRF(r) {
			http.Error(w, "invalid form token", http.StatusForbidden)
			return
		}
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if err := app.Alliance.Join(r.Context(), user.UserID, id); err != nil {
			renderError(w, r, app, "alliance_detail", "could not join alliance", id)
			return
		}
		http.Redirect(w, r, "/alliance/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
	}
}

// allianceLeaveHandler removes the current user from the alliance keyed by URL id.
func allianceLeaveHandler(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := sessionFromCtx(r.Context())
		if user == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		if !checkCSRF(r) {
			http.Error(w, "invalid form token", http.StatusForbidden)
			return
		}
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if err := app.Alliance.Leave(r.Context(), user.UserID, id); err != nil {
			renderError(w, r, app, "alliance_detail", "could not leave alliance", id)
			return
		}
		http.Redirect(w, r, "/alliance", http.StatusSeeOther)
	}
}

// lookupCurrentAlliance returns the user's current alliance (or nil). We hit
// the store directly because AllianceService does not expose the "by user"
// lookup; the alliance list view needs it to highlight the user's membership.
func lookupCurrentAlliance(r *http.Request, app *svc.App, uid int64) *svc.Alliance {
	row, err := app.Queries.GetUserAlliance(r.Context(), uid)
	if err != nil || row == nil {
		return nil
	}
	count, _ := app.Queries.CountAllianceMembers(r.Context(), row.ID)
	return &svc.Alliance{
		ID:          row.ID,
		Tag:         row.Tag,
		Name:        row.Name,
		Description: row.Description,
		OwnerUserID: row.FounderID,
		MemberCount: count,
		CreatedAt:   row.CreatedAt,
	}
}

func authCodeFromRequest(r *http.Request) string {
	if c := r.URL.Query().Get("code"); c != "" {
		return c
	}
	return r.FormValue("code")
}

// friendlyAuthError translates the AuthService sentinel errors to short
// user-facing strings. Unknown errors are surfaced as-is.
func friendlyAuthError(err error) string {
	switch {
	case errors.Is(err, svc.ErrUsernameTaken):
		return "that username is taken"
	case errors.Is(err, svc.ErrEmailTaken):
		return "that email is already registered"
	case errors.Is(err, svc.ErrInvalidUsername):
		return "username must be 3-32 chars: letters, digits, underscore, or dash"
	case errors.Is(err, svc.ErrInvalidEmail):
		return "that email address looks invalid"
	case errors.Is(err, svc.ErrPasswordTooShort):
		return "password must be at least 8 characters"
	case errors.Is(err, svc.ErrInvalidLogin):
		return "wrong username or password"
	case errors.Is(err, svc.ErrSessionExpired):
		return "session expired, please log in again"
	default:
		return "could not complete that request, please try again"
	}
}

// renderError re-renders the alliance detail page with an error banner. Used
// when a state-changing POST fails so the user lands back on the same page.
func renderError(w http.ResponseWriter, r *http.Request, app *svc.App, page string, msg string, allianceID int64) {
	view := baseView(app, r, "alliance")
	view.Error = msg
	if allianceID > 0 {
		if a, err := app.Alliance.Get(r.Context(), allianceID); err == nil {
			view.Alliance = a
			if view.User != nil {
				view.Current = lookupCurrentAlliance(r, app, view.User.ID)
				view.IsMember = view.Current != nil && view.Current.ID == a.ID
				view.IsFounder = a.OwnerUserID == view.User.ID
			}
		}
	}
	writePage(w, page, view)
}

// writePage is the one place that renders a page and translates errors into
// a 500 with the rendered banner. Keeps every handler one line of render.
func writePage(w http.ResponseWriter, page string, data viewData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := render(w, page, data); err != nil {
		slog.Error("template render failed", "page", page, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}
