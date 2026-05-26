package web

import (
	"net/http"
	"strconv"
	"time"

	"github.com/cobanov/terminal-army-go/internal/svc"
)

// adminPageSize is the default number of users shown on /admin/users. Kept
// modest so the page renders fast even when the user table grows past a few
// thousand rows; pagination handles the rest.
const adminPageSize = 50

// adminDashboardHandler renders the admin landing page: server-wide counters,
// active session count, and the list of universes (so the operator can see at
// a glance whether seed-universe needs to run).
func adminDashboardHandler(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		view := baseView(app, r, "admin")

		stats, err := app.Stats.Overview(r.Context())
		if err != nil {
			view.Error = "failed to load stats: " + err.Error()
			writePage(w, "admin_dashboard", view)
			return
		}
		view.Stats = stats

		sessions, err := app.Queries.CountActiveSessions(r.Context())
		if err == nil {
			view.ActiveSessions = sessions
		}

		universes, err := app.Universe.List(r.Context())
		if err == nil {
			view.Universes = universes
		}

		writePage(w, "admin_dashboard", view)
	}
}

// adminUsersHandler renders the paginated user list on GET. POST handles role
// changes (promote / demote). We embed the action in a hidden form field so a
// single form-action can flip a user either direction.
func adminUsersHandler(app *svc.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		view := baseView(app, r, "admin: users")

		if r.Method == http.MethodPost {
			if !checkCSRF(r) {
				view.Error = "invalid form token, please retry"
				renderAdminUsers(w, r, app, view, 0)
				return
			}
			view.Error = handleAdminUserPost(r, app)
		}

		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		if offset < 0 {
			offset = 0
		}
		renderAdminUsers(w, r, app, view, offset)
	}
}

// renderAdminUsers loads one page of users and emits the template. Split out
// so both GET and the POST-success path use the same view-loading code.
func renderAdminUsers(w http.ResponseWriter, r *http.Request, app *svc.App, view viewData, offset int) {
	limit := adminPageSize
	users, err := app.Queries.ListUsers(r.Context(), limit+1, offset)
	if err != nil {
		view.Error = "failed to load users: " + err.Error()
		writePage(w, "admin_users", view)
		return
	}

	hasNext := len(users) > limit
	if hasNext {
		users = users[:limit]
	}

	rows := make([]adminUserRow, 0, len(users))
	selfID := int64(0)
	if view.User != nil {
		selfID = view.User.ID
	}
	for _, u := range users {
		last := "-"
		if u.LastSeenAt != nil {
			last = u.LastSeenAt.UTC().Format(time.RFC3339)
		}
		rows = append(rows, adminUserRow{
			ID:       u.ID,
			Username: u.Username,
			Email:    u.Email,
			Role:     u.Role,
			Joined:   u.CreatedAt.UTC().Format("2006-01-02"),
			LastSeen: last,
			IsSelf:   u.ID == selfID,
		})
	}

	prevOffset := offset - limit
	if prevOffset < 0 {
		prevOffset = 0
	}

	view.Users = rows
	view.UserPage = adminUserPage{
		Limit:      limit,
		Offset:     offset,
		HasPrev:    offset > 0,
		HasNext:    hasNext,
		PrevOffset: prevOffset,
		NextOffset: offset + limit,
	}
	writePage(w, "admin_users", view)
}

// handleAdminUserPost applies a promote or demote action and returns a
// human-readable error string (empty on success). Self-demotion is refused so
// an admin cannot accidentally lock themselves out via the web UI.
func handleAdminUserPost(r *http.Request, app *svc.App) string {
	action := r.FormValue("action")
	userIDStr := r.FormValue("user_id")
	uid, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil || uid <= 0 {
		return "invalid user id"
	}

	sess := sessionFromCtx(r.Context())
	var role string
	switch action {
	case "promote":
		role = "admin"
	case "demote":
		role = "player"
		if sess != nil && sess.User != nil && sess.User.ID == uid {
			return "refusing to demote yourself: use the CLI if you really mean it"
		}
	default:
		return "unknown action"
	}

	if err := app.Queries.SetUserRole(r.Context(), uid, role); err != nil {
		return "role update failed: " + err.Error()
	}
	return ""
}
