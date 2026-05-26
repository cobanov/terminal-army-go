package svc

import (
	"context"
	"errors"

	"github.com/cobanov/terminal-army-go/internal/store"
)

// PlanetService exposes planet metadata, hydrating the public Planet struct
// from the planets row plus the side tables (buildings, ships, defense). All
// reads go through Resources.Refresh first so the caller always sees current
// stockpiles even if the planet has been idle for hours.
type PlanetService struct{ app *App }

// ListByUser returns every planet owned by the user with resources refreshed.
// Buildings/ships/defense are included so the TUI overview can render without
// a per-planet round trip.
func (s *PlanetService) ListByUser(ctx context.Context, userID int64) ([]Planet, error) {
	rows, err := s.app.Queries.ListPlanetsByOwner(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]Planet, 0, len(rows))
	for i := range rows {
		// Lazy-refresh resources before serialising. Errors from refresh are
		// non-fatal - we still return the stale snapshot rather than block
		// the entire list because one planet had a hiccup.
		refreshed, err := s.refreshAndReload(ctx, &rows[i])
		if err == nil && refreshed != nil {
			rows[i] = *refreshed
		}
		pub, err := s.toPublic(ctx, &rows[i])
		if err != nil {
			return nil, err
		}
		out = append(out, *pub)
	}
	return out, nil
}

// GetForUser fetches one planet, asserting ownership.
func (s *PlanetService) GetForUser(ctx context.Context, userID, planetID int64) (*Planet, error) {
	row, err := s.app.Queries.GetPlanet(ctx, planetID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if row.OwnerUserID != userID {
		return nil, ErrForbidden
	}
	refreshed, err := s.refreshAndReload(ctx, row)
	if err == nil && refreshed != nil {
		row = refreshed
	}
	return s.toPublic(ctx, row)
}

// GetByCodeForUser fetches a planet by its short code, asserting ownership.
func (s *PlanetService) GetByCodeForUser(ctx context.Context, userID int64, code string) (*Planet, error) {
	row, err := s.app.Queries.GetPlanetByCode(ctx, normalizePlanetCode(code))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if row.OwnerUserID != userID {
		return nil, ErrForbidden
	}
	refreshed, err := s.refreshAndReload(ctx, row)
	if err == nil && refreshed != nil {
		row = refreshed
	}
	return s.toPublic(ctx, row)
}

// Production returns a per-resource production report including the energy
// balance that scales actual output. The handler at /api/v1/planet/{id}/production
// surfaces this so the TUI can show "x metal/hour".
func (s *PlanetService) Production(ctx context.Context, userID, planetID int64) (*ProductionReport, error) {
	planet, err := s.GetForUser(ctx, userID, planetID)
	if err != nil {
		return nil, err
	}
	// Compute via ResourceService so we don't duplicate the formula here.
	return s.app.Resources.computeReport(ctx, planet)
}

// Rename changes the cosmetic planet name, capped to a sane length.
func (s *PlanetService) Rename(ctx context.Context, userID, planetID int64, name string) (*Planet, error) {
	if len(name) < 1 || len(name) > 32 {
		return nil, errors.New("planet name must be 1-32 characters")
	}
	row, err := s.app.Queries.GetPlanet(ctx, planetID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if row.OwnerUserID != userID {
		return nil, ErrForbidden
	}
	if err := s.app.Queries.SetPlanetName(ctx, planetID, name); err != nil {
		return nil, err
	}
	return s.GetForUser(ctx, userID, planetID)
}

// refreshAndReload calls the resource service to apply lazy production for the
// elapsed window, then re-reads the row. Returns nil row if resources service
// is not wired (very early-boot path).
func (s *PlanetService) refreshAndReload(ctx context.Context, p *store.Planet) (*store.Planet, error) {
	if s.app.Resources == nil {
		return nil, nil
	}
	if err := s.app.Resources.Refresh(ctx, p.ID); err != nil {
		return nil, err
	}
	return s.app.Queries.GetPlanet(ctx, p.ID)
}

// toPublic converts a planets row plus its side tables into the public Planet
// shape returned by the API. Buildings is always populated (empty map if no
// rows); ships and defense are omitted unless non-empty.
func (s *PlanetService) toPublic(ctx context.Context, row *store.Planet) (*Planet, error) {
	buildings, err := s.app.Queries.ListBuildingsForPlanet(ctx, row.ID)
	if err != nil {
		return nil, err
	}
	ships, _ := s.app.Queries.ListShipsForPlanet(ctx, row.ID)
	def, _ := s.app.Queries.ListDefensesForPlanet(ctx, row.ID)

	// EnergyUsed / EnergyProduced are derived at production-report time so we
	// can keep the row narrow. The overview UI calls Production for that.
	out := &Planet{
		ID:                     row.ID,
		Code:                   row.Code,
		Name:                   row.Name,
		OwnerUserID:            row.OwnerUserID,
		UniverseID:             row.UniverseID,
		Galaxy:                 row.Galaxy,
		System:                 row.System,
		Position:               row.Position,
		FieldsUsed:             row.FieldsUsed,
		FieldsTotal:            row.FieldsTotal,
		TempMin:                row.TempMin,
		TempMax:                row.TempMax,
		Metal:                  row.Metal,
		Crystal:                row.Crystal,
		Deuterium:              row.Deuterium,
		ResourcesLastUpdatedAt: row.ResourcesLastUpdatedAt,
		Buildings:              buildings,
	}
	if len(ships) > 0 {
		out.Ships = ships
	}
	if len(def) > 0 {
		out.Defense = def
	}
	return out, nil
}
