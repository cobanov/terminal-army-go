// Package svc wires together the application services and shared dependencies.
// Each sub-service owns one slice of game state; handlers in internal/httpapi
// stay thin and only translate HTTP to a sub-service call.
package svc

import (
	"context"
	"time"

	"github.com/cobanov/terminal-army-go/internal/config"
	"github.com/cobanov/terminal-army-go/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EventSink is implemented by event broadcasters (currently the WebSocket hub).
// The Python port relies on polling, so the sink is optional - services may
// publish events but the runtime can run with a no-op sink.
type EventSink interface {
	Broadcast(userID int64, event string, payload any)
}

type noopSink struct{}

func (noopSink) Broadcast(int64, string, any) {}

// TokenIssuer signs and verifies bearer tokens. The real implementation lives
// in internal/auth; defining the interface here keeps svc free of the JWT
// dependency and avoids an import cycle (internal/auth already imports svc).
type TokenIssuer interface {
	Issue(userID int64, sessionCode string) (token string, expiresAt time.Time, err error)
	Verify(token string) (userID int64, sessionCode string, err error)
}

// App is the dependency graph shared across HTTP handlers, the scheduler,
// and the CLI. Sub-services hold pointers back to App for cross-service calls.
type App struct {
	Cfg     *config.Config
	Pool    *pgxpool.Pool
	Queries *store.Queries
	Events  EventSink
	Tokens  TokenIssuer

	Auth        *AuthService
	Universe    *UniverseService
	Planet      *PlanetService
	Resources   *ResourceService
	Build       *BuildService
	Shipyard    *ShipyardService
	Research    *ResearchService
	Galaxy      *GalaxyService
	Fleet       *FleetService
	Messages    *MessagesService
	Reports     *ReportsService
	Alliance    *AllianceService
	Leaderboard *LeaderboardService
	Stats       *StatsService
	View        *ViewService
	Presence    *PresenceTracker
}

func NewApp(cfg *config.Config, pool *pgxpool.Pool, q *store.Queries, tokens TokenIssuer) *App {
	a := &App{
		Cfg:     cfg,
		Pool:    pool,
		Queries: q,
		Events:  noopSink{},
		Tokens:  tokens,
	}
	a.Presence = NewPresenceTracker()
	a.Auth = &AuthService{app: a}
	a.Universe = &UniverseService{app: a}
	a.Planet = &PlanetService{app: a}
	a.Resources = &ResourceService{app: a}
	a.Build = &BuildService{app: a}
	a.Shipyard = &ShipyardService{app: a}
	a.Research = &ResearchService{app: a}
	a.Galaxy = &GalaxyService{app: a}
	a.Fleet = &FleetService{app: a}
	a.Messages = &MessagesService{app: a}
	a.Reports = &ReportsService{app: a}
	a.Alliance = &AllianceService{app: a}
	a.Leaderboard = &LeaderboardService{app: a}
	a.Stats = &StatsService{app: a}
	a.View = &ViewService{app: a}
	return a
}

// SessionLookup is implemented by AuthService and is consumed by the auth
// middleware. Defining it here keeps internal/auth from depending on svc.
type SessionLookup interface {
	ResolveSession(ctx context.Context, token string) (*Session, error)
}
