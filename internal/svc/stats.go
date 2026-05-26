package svc

import (
	"context"
	"time"
)

// startedAt is captured when the binary boots so /stats can report uptime
// without any DB lookups. Set from main via SetStartTime; defaults to the
// time the package was loaded so /stats is always meaningful.
var startedAt = time.Now()

// SetStartTime overrides the process start moment. Called from cmd/serve so
// the uptime counter survives package init quirks (e.g. tests that import
// svc indirectly).
func SetStartTime(t time.Time) {
	startedAt = t
}

// Overview returns aggregate server counters used by /stats and admin UIs.
// Each counter is one cheap SELECT COUNT - this endpoint is unauthenticated
// so we keep the work bounded.
func (s *StatsService) Overview(ctx context.Context) (*StatsOverview, error) {
	universes, err := s.app.Queries.CountUniverses(ctx)
	if err != nil {
		return nil, err
	}
	players, err := s.app.Queries.CountUsers(ctx)
	if err != nil {
		return nil, err
	}
	planets, err := s.app.Queries.CountAllPlanets(ctx)
	if err != nil {
		return nil, err
	}
	fleets, err := s.app.Queries.CountFleetsInFlight(ctx)
	if err != nil {
		return nil, err
	}
	online := 0
	if s.app.Presence != nil {
		online = s.app.Presence.Count()
	}

	uptime := int64(time.Since(startedAt).Seconds())
	if uptime < 0 {
		uptime = 0
	}
	return &StatsOverview{
		Universes:      universes,
		Players:        players,
		Planets:        planets,
		OnlinePlayers:  online,
		FleetsInFlight: fleets,
		UptimeSeconds:  uptime,
	}, nil
}
