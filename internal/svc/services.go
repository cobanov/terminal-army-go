package svc

// Struct definitions for services whose methods live in their own files.
// Each service that has graduated keeps its type here; the method bodies are
// next to their owning data access in dedicated files (messages.go,
// reports.go, alliance.go, leaderboard.go, stats.go, ...).

// GalaxyService is defined in galaxy.go.
type GalaxyService struct{ app *App }

// FleetService is defined in fleet.go. Arrival and return processing live in
// the scheduler so a single tick can resolve many fleets at once.

// MessagesService is the in-game inbox. Methods live in messages.go.
type MessagesService struct{ app *App }

// ReportsService stores combat and espionage reports. Methods live in reports.go.
type ReportsService struct{ app *App }

// AllianceService owns alliances, membership, and join requests. Methods live
// in alliance.go.
type AllianceService struct{ app *App }

// LeaderboardService computes scoring leaderboards. Methods live in
// leaderboard.go.
type LeaderboardService struct{ app *App }

// StatsService exposes server-wide counters used by /stats and admin tooling.
// Methods live in stats.go.
type StatsService struct{ app *App }
