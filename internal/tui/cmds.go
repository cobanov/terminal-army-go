// tea.Cmd factories. Every async API call funnels through here so the
// patterns (context, error wrapping, typed-message return) stay consistent.
package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cobanov/terminal-army-go/internal/svc"
	"github.com/cobanov/terminal-army-go/internal/tui/client"
)

// apiTimeout is the per-call deadline. The server is local for `tarmy play`
// against a local server but we still want a hard cap so a hung HTTP call
// does not freeze the UI.
const apiTimeout = 15 * time.Second

// withCtx runs fn with a fresh timeout context and returns a tea.Cmd that
// emits either the success message or an errMsg. The factory removes a lot
// of boilerplate from individual screens.
func withCtx[T any](fn func(ctx context.Context) (T, error), wrap func(T) tea.Msg) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
		defer cancel()
		out, err := fn(ctx)
		if err != nil {
			return errMsg{err}
		}
		return wrap(out)
	}
}

func cmdLogin(c *client.Client, username, password string) tea.Cmd {
	return withCtx(
		func(ctx context.Context) (*svc.AuthResult, error) {
			return c.Login(ctx, username, password)
		},
		func(out *svc.AuthResult) tea.Msg {
			c.SetToken(out.Token)
			return sessionReadyMsg{user: out.User}
		},
	)
}

func cmdRegister(c *client.Client, username, email, password string) tea.Cmd {
	return withCtx(
		func(ctx context.Context) (*svc.AuthResult, error) {
			return c.Register(ctx, username, email, password)
		},
		func(out *svc.AuthResult) tea.Msg {
			c.SetToken(out.Token)
			return sessionReadyMsg{user: out.User}
		},
	)
}

func cmdMe(c *client.Client) tea.Cmd {
	return withCtx(
		func(ctx context.Context) (*svc.User, error) { return c.Me(ctx) },
		func(u *svc.User) tea.Msg { return sessionReadyMsg{user: u} },
	)
}

func cmdLogout(c *client.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
		defer cancel()
		_ = c.Logout(ctx)
		c.SetToken("")
		return loggedOutMsg{}
	}
}

func cmdListPlanets(c *client.Client) tea.Cmd {
	return withCtx(
		func(ctx context.Context) ([]svc.Planet, error) { return c.ListPlanets(ctx) },
		func(ps []svc.Planet) tea.Msg { return planetsLoadedMsg{ps} },
	)
}

func cmdGetPlanet(c *client.Client, id int64) tea.Cmd {
	return withCtx(
		func(ctx context.Context) (*svc.Planet, error) { return c.GetPlanet(ctx, id) },
		func(p *svc.Planet) tea.Msg { return planetLoadedMsg{p} },
	)
}

func cmdGetProduction(c *client.Client, id int64) tea.Cmd {
	return withCtx(
		func(ctx context.Context) (*svc.ProductionReport, error) { return c.GetProduction(ctx, id) },
		func(p *svc.ProductionReport) tea.Msg { return productionLoadedMsg{p} },
	)
}

func cmdGetQueues(c *client.Client, planetID int64) tea.Cmd {
	return withCtx(
		func(ctx context.Context) ([]svc.QueueItem, error) { return c.GetQueues(ctx, planetID) },
		func(qs []svc.QueueItem) tea.Msg { return queuesLoadedMsg{qs} },
	)
}

func cmdQueueBuilding(c *client.Client, planetID int64, key string) tea.Cmd {
	return withCtx(
		func(ctx context.Context) (*svc.QueueItem, error) { return c.QueueBuilding(ctx, planetID, key) },
		func(q *svc.QueueItem) tea.Msg { return queueQueuedMsg{q} },
	)
}

func cmdQueueResearch(c *client.Client, planetID int64, tech string) tea.Cmd {
	return withCtx(
		func(ctx context.Context) (*svc.QueueItem, error) { return c.QueueResearch(ctx, planetID, tech) },
		func(q *svc.QueueItem) tea.Msg { return queueQueuedMsg{q} },
	)
}

func cmdQueueShip(c *client.Client, planetID int64, ship string, count int) tea.Cmd {
	return withCtx(
		func(ctx context.Context) (*svc.QueueItem, error) { return c.QueueShip(ctx, planetID, ship, count) },
		func(q *svc.QueueItem) tea.Msg { return queueQueuedMsg{q} },
	)
}

func cmdQueueDefense(c *client.Client, planetID int64, def string, count int) tea.Cmd {
	return withCtx(
		func(ctx context.Context) (*svc.QueueItem, error) { return c.QueueDefense(ctx, planetID, def, count) },
		func(q *svc.QueueItem) tea.Msg { return queueQueuedMsg{q} },
	)
}

func cmdListResearch(c *client.Client) tea.Cmd {
	return withCtx(
		func(ctx context.Context) ([]svc.ResearchLevel, error) { return c.ListResearch(ctx) },
		func(rs []svc.ResearchLevel) tea.Msg { return researchLoadedMsg{rs} },
	)
}

func cmdViewSystem(c *client.Client, g, s int) tea.Cmd {
	return withCtx(
		func(ctx context.Context) (*svc.SystemView, error) { return c.ViewSystem(ctx, g, s) },
		func(v *svc.SystemView) tea.Msg { return systemLoadedMsg{v} },
	)
}

func cmdListFleet(c *client.Client) tea.Cmd {
	return withCtx(
		func(ctx context.Context) ([]svc.Fleet, error) { return c.ListFleet(ctx) },
		func(fs []svc.Fleet) tea.Msg { return fleetsLoadedMsg{fs} },
	)
}

func cmdDispatchFleet(c *client.Client, req svc.FleetDispatchRequest) tea.Cmd {
	return withCtx(
		func(ctx context.Context) (*svc.Fleet, error) { return c.DispatchFleet(ctx, req) },
		func(f *svc.Fleet) tea.Msg { return fleetDispatchedMsg{f} },
	)
}

func cmdRecallFleet(c *client.Client, id int64) tea.Cmd {
	return withCtx(
		func(ctx context.Context) (*svc.Fleet, error) { return c.RecallFleet(ctx, id) },
		func(f *svc.Fleet) tea.Msg { return fleetDispatchedMsg{f} },
	)
}

func cmdListMessages(c *client.Client) tea.Cmd {
	return withCtx(
		func(ctx context.Context) ([]svc.Message, error) { return c.ListMessages(ctx) },
		func(ms []svc.Message) tea.Msg { return messagesLoadedMsg{ms} },
	)
}

func cmdGetMessage(c *client.Client, id int64) tea.Cmd {
	return withCtx(
		func(ctx context.Context) (*svc.Message, error) { return c.GetMessage(ctx, id) },
		func(m *svc.Message) tea.Msg { return messageLoadedMsg{m} },
	)
}

func cmdDeleteMessage(c *client.Client, id int64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
		defer cancel()
		if err := c.DeleteMessage(ctx, id); err != nil {
			return errMsg{err}
		}
		return messageDeletedMsg{id: id}
	}
}

func cmdLeaderboard(c *client.Client) tea.Cmd {
	return withCtx(
		func(ctx context.Context) ([]svc.LeaderboardEntry, error) { return c.Leaderboard(ctx) },
		func(es []svc.LeaderboardEntry) tea.Msg { return leaderboardLoadedMsg{es} },
	)
}

func cmdStats(c *client.Client) tea.Cmd {
	return withCtx(
		func(ctx context.Context) (*svc.StatsOverview, error) { return c.Stats(ctx) },
		func(s *svc.StatsOverview) tea.Msg { return statsLoadedMsg{s} },
	)
}

func cmdListUniverses(c *client.Client) tea.Cmd {
	return withCtx(
		func(ctx context.Context) ([]svc.Universe, error) { return c.ListUniverses(ctx) },
		func(us []svc.Universe) tea.Msg { return universesLoadedMsg{us} },
	)
}

func cmdJoinUniverse(c *client.Client, id int64) tea.Cmd {
	return withCtx(
		func(ctx context.Context) (*svc.Planet, error) { return c.JoinUniverse(ctx, id) },
		func(p *svc.Planet) tea.Msg { return universeJoinedMsg{p} },
	)
}

// cmdStatus emits a transient banner and schedules its clear after 3 seconds.
func cmdStatus(text string, level statusLevel) tea.Cmd {
	return tea.Batch(
		func() tea.Msg { return statusMsg{text: text, level: level} },
		tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearStatusMsg{} }),
	)
}
