package svc

import (
	"context"
	"errors"
	"time"

	"github.com/cobanov/terminal-army-go/internal/game"
	"github.com/cobanov/terminal-army-go/internal/store"
	"github.com/jackc/pgx/v5"
)

// ResourceService implements the OGame-style lazy production model: nothing
// runs on a per-second timer, instead every read/write call first applies the
// production formula for the window (now - resources_last_updated_at) and
// then bumps the timestamp. This keeps the DB write-rate proportional to
// player activity rather than to wall-clock time.
type ResourceService struct{ app *App }

// Refresh applies elapsed-time production to one planet. Safe to call from
// any code path; it short-circuits if the row was updated within the last
// second to avoid wasting bcrypt-cost cycles on rapid bursts. Runs inside a
// transaction with row-level locking so concurrent build/dispatch commands
// can't double-credit the same window.
func (s *ResourceService) Refresh(ctx context.Context, planetID int64) error {
	if planetID == 0 {
		return nil
	}
	return store.InTx(ctx, s.app.Pool, func(tx pgx.Tx) error {
		qtx := s.app.Queries.WithTx(tx)
		return s.refreshTx(ctx, qtx, planetID)
	})
}

// RefreshInTx is the in-transaction version - callers that already hold a tx
// (build queue, fleet dispatch) call this so the resource update is part of
// the same atomic write that debits costs.
func (s *ResourceService) RefreshInTx(ctx context.Context, qtx *store.Queries, planetID int64) error {
	return s.refreshTx(ctx, qtx, planetID)
}

func (s *ResourceService) refreshTx(ctx context.Context, qtx *store.Queries, planetID int64) error {
	planet, err := qtx.GetPlanetForUpdate(ctx, planetID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if planet.ResourcesLastUpdatedAt.IsZero() {
		// Brand-new row; nothing to back-fill but bump the cursor so the next
		// pass starts cleanly.
		return qtx.UpdatePlanetResources(ctx, planet.ID, planet.Metal, planet.Crystal, planet.Deuterium, now)
	}
	elapsed := now.Sub(planet.ResourcesLastUpdatedAt)
	if elapsed < time.Second {
		return nil
	}

	report, err := s.reportForPlanet(ctx, qtx, planet)
	if err != nil {
		return err
	}

	hours := elapsed.Hours()
	metal := planet.Metal + report.MetalPerHour*hours
	crystal := planet.Crystal + report.CrystalPerHour*hours
	deut := planet.Deuterium + report.DeuteriumPerHour*hours

	// Cap each pool at the matching storage building's capacity. We cap rather
	// than overflow so the player loses production when storage is full, which
	// matches OGame behaviour and pushes players to build storages.
	buildings, err := qtx.ListBuildingsForPlanet(ctx, planet.ID)
	if err != nil {
		return err
	}
	capM := float64(game.StorageCapacity(buildings[string(game.BuildingMetalStorage)]))
	capC := float64(game.StorageCapacity(buildings[string(game.BuildingCrystalStorage)]))
	capD := float64(game.StorageCapacity(buildings[string(game.BuildingDeuteriumTank)]))
	if metal > capM {
		metal = capM
	}
	if crystal > capC {
		crystal = capC
	}
	if deut > capD {
		deut = capD
	}
	if metal < 0 {
		metal = 0
	}
	if crystal < 0 {
		crystal = 0
	}
	if deut < 0 {
		deut = 0
	}

	return qtx.UpdatePlanetResources(ctx, planet.ID, metal, crystal, deut, now)
}

// computeReport is the public entry point that PlanetService.Production calls
// to render a snapshot to the API. It refreshes first so the numbers shown to
// the player match what the next Refresh will credit.
func (s *ResourceService) computeReport(ctx context.Context, planet *Planet) (*ProductionReport, error) {
	if planet == nil {
		return nil, errors.New("nil planet")
	}
	universe, err := s.app.Queries.GetUniverse(ctx, planet.UniverseID)
	if err != nil {
		return nil, err
	}
	researches, err := s.app.Queries.ListResearchesForUser(ctx, planet.OwnerUserID)
	if err != nil {
		return nil, err
	}

	report := game.ComputePlanetProduction(
		toBuildingMap(planet.Buildings),
		toTechMap(researches),
		planet.TempMin, planet.TempMax,
		game.MetalBonusByPosition[planet.Position],
		game.CrystalBonusByPosition[planet.Position],
		float64(universe.SpeedEconomy),
	)
	return &ProductionReport{
		PlanetID:            planet.ID,
		MetalPerHour:        report.MetalPerHour,
		CrystalPerHour:      report.CrystalPerHour,
		DeuteriumPerHour:    report.DeuteriumPerHour,
		EnergyProduced:      report.EnergyProduced,
		EnergyUsed:          report.EnergyConsumed,
		ProductionFactor:    report.ProductionFactor,
		StorageCapMetal:     game.StorageCapacity(planet.Buildings[string(game.BuildingMetalStorage)]),
		StorageCapCrystal:   game.StorageCapacity(planet.Buildings[string(game.BuildingCrystalStorage)]),
		StorageCapDeuterium: game.StorageCapacity(planet.Buildings[string(game.BuildingDeuteriumTank)]),
	}, nil
}

// reportForPlanet is the in-tx equivalent of computeReport - takes a row
// rather than the public Planet shape so refreshTx doesn't have to round-trip
// through PlanetService.toPublic.
func (s *ResourceService) reportForPlanet(ctx context.Context, qtx *store.Queries, p *store.Planet) (game.ProductionReport, error) {
	universe, err := qtx.GetUniverse(ctx, p.UniverseID)
	if err != nil {
		return game.ProductionReport{}, err
	}
	buildings, err := qtx.ListBuildingsForPlanet(ctx, p.ID)
	if err != nil {
		return game.ProductionReport{}, err
	}
	researches, err := qtx.ListResearchesForUser(ctx, p.OwnerUserID)
	if err != nil {
		return game.ProductionReport{}, err
	}
	return game.ComputePlanetProduction(
		toBuildingMap(buildings),
		toTechMap(researches),
		p.TempMin, p.TempMax,
		game.MetalBonusByPosition[p.Position],
		game.CrystalBonusByPosition[p.Position],
		float64(universe.SpeedEconomy),
	), nil
}

func toBuildingMap(in map[string]int) map[game.BuildingType]int {
	out := make(map[game.BuildingType]int, len(in))
	for k, v := range in {
		out[game.BuildingType(k)] = v
	}
	return out
}

func toTechMap(in map[string]int) map[game.TechType]int {
	out := make(map[game.TechType]int, len(in))
	for k, v := range in {
		out[game.TechType(k)] = v
	}
	return out
}
