// Typed wrappers around the REST API. Each method is a one-line shim over
// Client.do so the TUI never builds URLs by hand. Methods return the same
// shapes the server emits (defined in internal/svc/types.go) so they can be
// rendered directly by Bubble Tea views.
package client

import (
	"context"
	"fmt"

	"github.com/cobanov/terminal-army-go/internal/svc"
)

// --- auth ---

// Register creates a new account and returns a token and the user record.
// The token is NOT installed automatically - the caller decides whether to
// keep the credentials.
func (c *Client) Register(ctx context.Context, username, email, password string) (*svc.AuthResult, error) {
	body := map[string]string{
		"username": username,
		"email":    email,
		"password": password,
	}
	out := &svc.AuthResult{}
	if err := c.do(ctx, "POST", "/api/v1/auth/register", body, out); err != nil {
		return nil, err
	}
	return out, nil
}

// Login authenticates and returns a token + user. Token is not installed
// automatically; caller chooses whether to call SetToken.
func (c *Client) Login(ctx context.Context, username, password string) (*svc.AuthResult, error) {
	body := map[string]string{
		"username": username,
		"password": password,
	}
	out := &svc.AuthResult{}
	if err := c.do(ctx, "POST", "/api/v1/auth/login", body, out); err != nil {
		return nil, err
	}
	return out, nil
}

// Logout invalidates the current bearer token server-side. The local token
// stays set; clear it via SetToken("") if you want a clean slate.
func (c *Client) Logout(ctx context.Context) error {
	return c.do(ctx, "POST", "/api/v1/auth/logout", nil, nil)
}

// StartDeviceAuth begins the browser auth flow used by the default CLI.
func (c *Client) StartDeviceAuth(ctx context.Context) (*svc.DeviceAuthStart, error) {
	out := &svc.DeviceAuthStart{}
	if err := c.do(ctx, "POST", "/auth/start", nil, out); err != nil {
		return nil, err
	}
	return out, nil
}

// PollDeviceAuth returns a token once the browser flow completes.
func (c *Client) PollDeviceAuth(ctx context.Context, code string) (*svc.DeviceAuthPoll, error) {
	body := map[string]string{"auth_code": code}
	out := &svc.DeviceAuthPoll{}
	if err := c.do(ctx, "POST", "/auth/poll", body, out); err != nil {
		return nil, err
	}
	if out.Token == "" {
		return nil, &APIError{Status: 202, Message: "pending"}
	}
	return out, nil
}

// AuthMe validates the token through the Python-compatible /auth/me route.
func (c *Client) AuthMe(ctx context.Context) (*svc.User, error) {
	out := &svc.User{}
	if err := c.do(ctx, "GET", "/auth/me", nil, out); err != nil {
		return nil, err
	}
	return out, nil
}

// Me returns the currently authenticated user.
func (c *Client) Me(ctx context.Context) (*svc.User, error) {
	out := &svc.User{}
	if err := c.do(ctx, "GET", "/api/v1/me", nil, out); err != nil {
		return nil, err
	}
	return out, nil
}

// --- universes ---

// ListUniverses returns every universe the server hosts.
func (c *Client) ListUniverses(ctx context.Context) ([]svc.Universe, error) {
	var out []svc.Universe
	if err := c.do(ctx, "GET", "/api/v1/universes", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// JoinUniverse creates the player's first planet in the given universe and
// returns the planet view.
func (c *Client) JoinUniverse(ctx context.Context, id int64) (*svc.Planet, error) {
	out := &svc.Planet{}
	path := fmt.Sprintf("/api/v1/universes/%d/join", id)
	if err := c.do(ctx, "POST", path, nil, out); err != nil {
		return nil, err
	}
	return out, nil
}

// --- planets ---

// ListPlanets returns every planet owned by the authenticated user.
func (c *Client) ListPlanets(ctx context.Context) ([]svc.Planet, error) {
	var out []svc.Planet
	if err := c.do(ctx, "GET", "/api/v1/planets", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetPlanet fetches a single planet by ID. The server refreshes resources
// before returning, so the response is always current.
func (c *Client) GetPlanet(ctx context.Context, id int64) (*svc.Planet, error) {
	out := &svc.Planet{}
	path := fmt.Sprintf("/api/v1/planets/%d", id)
	if err := c.do(ctx, "GET", path, nil, out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetProduction returns hourly production figures + storage caps.
func (c *Client) GetProduction(ctx context.Context, id int64) (*svc.ProductionReport, error) {
	out := &svc.ProductionReport{}
	path := fmt.Sprintf("/api/v1/planets/%d/production", id)
	if err := c.do(ctx, "GET", path, nil, out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetQueues returns the active build / research / shipyard items for a planet.
func (c *Client) GetQueues(ctx context.Context, planetID int64) ([]svc.QueueItem, error) {
	var out []svc.QueueItem
	path := fmt.Sprintf("/api/v1/planets/%d/queues", planetID)
	if err := c.do(ctx, "GET", path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// PlanetBuildings returns the render-ready resource-building rows for a planet:
// level, next-level cost, build time, affordability, and lock state all
// resolved server-side.
func (c *Client) PlanetBuildings(ctx context.Context, planetID int64) ([]svc.BuildingView, error) {
	var out []svc.BuildingView
	path := fmt.Sprintf("/api/v1/planets/%d/buildings", planetID)
	if err := c.do(ctx, "GET", path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// PlanetFacilities returns the render-ready facility-building rows for a planet.
func (c *Client) PlanetFacilities(ctx context.Context, planetID int64) ([]svc.BuildingView, error) {
	var out []svc.BuildingView
	path := fmt.Sprintf("/api/v1/planets/%d/facilities", planetID)
	if err := c.do(ctx, "GET", path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// PlanetResearch returns the research view (levels, costs, prereqs, tree
// parents) for a planet's owner.
func (c *Client) PlanetResearch(ctx context.Context, planetID int64) (*svc.ResearchView, error) {
	out := &svc.ResearchView{}
	path := fmt.Sprintf("/api/v1/planets/%d/research", planetID)
	if err := c.do(ctx, "GET", path, nil, out); err != nil {
		return nil, err
	}
	return out, nil
}

// PlanetShipyard returns the buildable-ship rows for a planet.
func (c *Client) PlanetShipyard(ctx context.Context, planetID int64) ([]svc.UnitView, error) {
	var out []svc.UnitView
	path := fmt.Sprintf("/api/v1/planets/%d/shipyard", planetID)
	if err := c.do(ctx, "GET", path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// PlanetDefense returns the buildable-defense rows for a planet.
func (c *Client) PlanetDefense(ctx context.Context, planetID int64) ([]svc.UnitView, error) {
	var out []svc.UnitView
	path := fmt.Sprintf("/api/v1/planets/%d/defense", planetID)
	if err := c.do(ctx, "GET", path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// QueueBuilding enqueues a single upgrade for the named building key
// (e.g. "metal_mine"). The server decides the target level.
func (c *Client) QueueBuilding(ctx context.Context, planetID int64, building string) (*svc.QueueItem, error) {
	body := map[string]string{"building": building}
	out := &svc.QueueItem{}
	path := fmt.Sprintf("/api/v1/planets/%d/buildings", planetID)
	if err := c.do(ctx, "POST", path, body, out); err != nil {
		return nil, err
	}
	return out, nil
}

// QueueShip enqueues N ships of the given key (e.g. "light_fighter").
func (c *Client) QueueShip(ctx context.Context, planetID int64, ship string, count int) (*svc.QueueItem, error) {
	body := map[string]any{"ship": ship, "count": count}
	out := &svc.QueueItem{}
	path := fmt.Sprintf("/api/v1/planets/%d/shipyard", planetID)
	if err := c.do(ctx, "POST", path, body, out); err != nil {
		return nil, err
	}
	return out, nil
}

// QueueDefense enqueues N defense units of the given key (e.g. "rocket_launcher").
func (c *Client) QueueDefense(ctx context.Context, planetID int64, defense string, count int) (*svc.QueueItem, error) {
	body := map[string]any{"defense": defense, "count": count}
	out := &svc.QueueItem{}
	path := fmt.Sprintf("/api/v1/planets/%d/defense", planetID)
	if err := c.do(ctx, "POST", path, body, out); err != nil {
		return nil, err
	}
	return out, nil
}

// --- research ---

// ListResearch returns every technology level for the authenticated user.
func (c *Client) ListResearch(ctx context.Context) ([]svc.ResearchLevel, error) {
	var out []svc.ResearchLevel
	if err := c.do(ctx, "GET", "/api/v1/research", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// QueueResearch enqueues the next level of the named technology. Research
// is consumed from the chosen lab planet.
func (c *Client) QueueResearch(ctx context.Context, planetID int64, tech string) (*svc.QueueItem, error) {
	body := map[string]any{"tech": tech, "planet_id": planetID}
	out := &svc.QueueItem{}
	if err := c.do(ctx, "POST", "/api/v1/research", body, out); err != nil {
		return nil, err
	}
	return out, nil
}

// --- galaxy ---

// ViewSystem returns the 15 planet slots in the given system.
func (c *Client) ViewSystem(ctx context.Context, galaxy, system int) (*svc.SystemView, error) {
	out := &svc.SystemView{}
	path := fmt.Sprintf("/api/v1/galaxy/%d/%d", galaxy, system)
	if err := c.do(ctx, "GET", path, nil, out); err != nil {
		return nil, err
	}
	return out, nil
}

// --- fleet ---

// DispatchFleet sends a fleet on the requested mission.
func (c *Client) DispatchFleet(ctx context.Context, req svc.FleetDispatchRequest) (*svc.Fleet, error) {
	out := &svc.Fleet{}
	if err := c.do(ctx, "POST", "/api/v1/fleet", req, out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListFleet returns every fleet that belongs to the authenticated user,
// including outbound, returning, and holding fleets.
func (c *Client) ListFleet(ctx context.Context) ([]svc.Fleet, error) {
	var out []svc.Fleet
	if err := c.do(ctx, "GET", "/api/v1/fleet", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// RecallFleet turns an outbound fleet around immediately.
func (c *Client) RecallFleet(ctx context.Context, fleetID int64) (*svc.Fleet, error) {
	out := &svc.Fleet{}
	path := fmt.Sprintf("/api/v1/fleet/%d/recall", fleetID)
	if err := c.do(ctx, "POST", path, nil, out); err != nil {
		return nil, err
	}
	return out, nil
}

// --- messages ---

// ListMessages returns the user's inbox, newest first.
func (c *Client) ListMessages(ctx context.Context) ([]svc.Message, error) {
	var out []svc.Message
	if err := c.do(ctx, "GET", "/api/v1/messages", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetMessage fetches and marks-read a single message.
func (c *Client) GetMessage(ctx context.Context, id int64) (*svc.Message, error) {
	out := &svc.Message{}
	path := fmt.Sprintf("/api/v1/messages/%d", id)
	if err := c.do(ctx, "GET", path, nil, out); err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteMessage removes a message permanently.
func (c *Client) DeleteMessage(ctx context.Context, id int64) error {
	path := fmt.Sprintf("/api/v1/messages/%d", id)
	return c.do(ctx, "DELETE", path, nil, nil)
}

// SendMessage sends an in-game player message by recipient username.
func (c *Client) SendMessage(ctx context.Context, to, body string) (*svc.Message, error) {
	out := &svc.Message{}
	if err := c.do(ctx, "POST", "/api/v1/messages", map[string]string{"to": to, "body": body}, out); err != nil {
		return nil, err
	}
	return out, nil
}

// --- reports ---

// ListReports returns combat and espionage reports for the current user.
func (c *Client) ListReports(ctx context.Context) ([]svc.Report, error) {
	var out []svc.Report
	if err := c.do(ctx, "GET", "/api/v1/reports", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetReport fetches a single report with full payload.
func (c *Client) GetReport(ctx context.Context, id int64) (*svc.Report, error) {
	out := &svc.Report{}
	path := fmt.Sprintf("/api/v1/reports/%d", id)
	if err := c.do(ctx, "GET", path, nil, out); err != nil {
		return nil, err
	}
	return out, nil
}

// --- alliance ---

// ListAlliances returns every alliance the server knows about.
func (c *Client) ListAlliances(ctx context.Context) ([]svc.Alliance, error) {
	var out []svc.Alliance
	if err := c.do(ctx, "GET", "/api/v1/alliance", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateAlliance founds a new alliance with the user as owner.
func (c *Client) CreateAlliance(ctx context.Context, tag, name, description string) (*svc.Alliance, error) {
	body := map[string]string{"tag": tag, "name": name, "description": description}
	out := &svc.Alliance{}
	if err := c.do(ctx, "POST", "/api/v1/alliance", body, out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetAlliance fetches a single alliance by ID.
func (c *Client) GetAlliance(ctx context.Context, id int64) (*svc.Alliance, error) {
	out := &svc.Alliance{}
	path := fmt.Sprintf("/api/v1/alliance/%d", id)
	if err := c.do(ctx, "GET", path, nil, out); err != nil {
		return nil, err
	}
	return out, nil
}

// JoinAlliance enrols the current user.
func (c *Client) JoinAlliance(ctx context.Context, id int64) error {
	path := fmt.Sprintf("/api/v1/alliance/%d/join", id)
	return c.do(ctx, "POST", path, nil, nil)
}

// LeaveAlliance removes the current user. The founder cannot leave while
// other members remain - the server enforces this.
func (c *Client) LeaveAlliance(ctx context.Context, id int64) error {
	path := fmt.Sprintf("/api/v1/alliance/%d/leave", id)
	return c.do(ctx, "POST", path, nil, nil)
}

// --- leaderboard + stats ---

// Leaderboard returns the top players by aggregate score.
func (c *Client) Leaderboard(ctx context.Context) ([]svc.LeaderboardEntry, error) {
	var out []svc.LeaderboardEntry
	if err := c.do(ctx, "GET", "/api/v1/leaderboard", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Stats returns server-wide counters (universes, players, fleets, uptime).
func (c *Client) Stats(ctx context.Context) (*svc.StatsOverview, error) {
	out := &svc.StatsOverview{}
	if err := c.do(ctx, "GET", "/api/v1/stats", nil, out); err != nil {
		return nil, err
	}
	return out, nil
}

// PublicStats returns the unauthenticated lobby stats, including the server's
// build version. Used at startup to warn when the client and server versions
// differ (a mismatch can make the player issue commands the server rejects or
// interprets differently).
func (c *Client) PublicStats(ctx context.Context) (*svc.PublicServerStats, error) {
	out := &svc.PublicServerStats{}
	if err := c.do(ctx, "GET", "/stats", nil, out); err != nil {
		return nil, err
	}
	return out, nil
}
