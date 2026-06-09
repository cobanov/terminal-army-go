package web

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"sync"
	"time"

	"github.com/cobanov/terminal-army-go/internal/svc"
)

//go:embed templates/*.html
var templateFS embed.FS

// viewData is the canonical envelope passed to every template render. Keeping
// one shape lets the shared layout always find Title, User, CSRF, Flash, and
// Error without per-page glue.
type viewData struct {
	Title     string
	User      *svc.User
	CSRF      string
	Flash     string
	Error     string
	PublicURL string
	Now       time.Time

	// page-specific payloads
	Form      map[string]string
	Alliances []svc.Alliance
	Alliance  *svc.Alliance
	Current   *svc.Alliance
	IsMember  bool
	IsFounder bool

	// admin payloads
	Stats          *svc.StatsOverview
	ActiveSessions int
	Universes      []svc.Universe
	Users          []adminUserRow
	UserPage       adminUserPage
}

// adminUserRow is the per-row view-model passed to the admin users page.
// We carry the friendly strings (rendered times, role label) so the template
// stays trivial.
type adminUserRow struct {
	ID       int64
	Username string
	Email    string
	Role     string
	Joined   string
	LastSeen string
	IsSelf   bool
}

// adminUserPage carries pagination state for /admin/users.
type adminUserPage struct {
	Limit      int
	Offset     int
	HasPrev    bool
	HasNext    bool
	PrevOffset int
	NextOffset int
}

var (
	templatesOnce sync.Once
	templates     map[string]*template.Template
	templatesErr  error
)

// pageTemplates lists every concrete page that pairs with the shared layout.
// Adding a page is two lines: drop in a content template under templates/ and
// register it here.
var pageTemplates = map[string]string{
	"index":            "templates/index.html",
	"signup":           "templates/signup.html",
	"login":            "templates/login.html",
	"terminal_success": "templates/terminal_success.html",
	"alliance_list":    "templates/alliance_list.html",
	"alliance_detail":  "templates/alliance_detail.html",
	"admin_dashboard":  "templates/admin_dashboard.html",
	"admin_users":      "templates/admin_users.html",
}

// loadTemplates parses each page bundled with the shared layout. We do this
// once at startup; html/template panics on race so the sync.Once both
// initialises and guards.
func loadTemplates() (map[string]*template.Template, error) {
	templatesOnce.Do(func() {
		out := make(map[string]*template.Template, len(pageTemplates))
		for name, path := range pageTemplates {
			t, err := template.New("layout").ParseFS(templateFS, "templates/layout.html", path)
			if err != nil {
				templatesErr = fmt.Errorf("parse %s: %w", name, err)
				return
			}
			out[name] = t
		}
		templates = out
	})
	return templates, templatesErr
}

// render writes the named page using the layout template. The page's own
// content block is defined inside its .html file.
func render(w io.Writer, page string, data viewData) error {
	tmpls, err := loadTemplates()
	if err != nil {
		return err
	}
	t, ok := tmpls[page]
	if !ok {
		return fmt.Errorf("unknown template %q", page)
	}
	if data.Form == nil {
		data.Form = map[string]string{}
	}
	return t.ExecuteTemplate(w, "layout", data)
}
