package svc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cobanov/terminal-army-go/internal/game"
	"github.com/cobanov/terminal-army-go/internal/store"
	"github.com/jackc/pgx/v5"
)

// FleetService owns fleet dispatch, listing, and recall. Arrival and return
// processing happens in the scheduler so a single tick can resolve many fleets
// in one transaction. The MVP supports six missions: attack, transport,
// colonize, deploy, espionage, recycle. Stationing (mission "stay") and
// alliance-attack mixing are post-MVP.
type FleetService struct{ app *App }

// Valid mission keys. Any other value rejects the dispatch.
var validMissions = map[string]struct{}{
	"attack":    {},
	"transport": {},
	"colonize":  {},
	"deploy":    {},
	"espionage": {},
	"recycle":   {},
}

// Dispatch launches a fleet from the user's planet toward a coordinate.
//
// Steps inside one tx:
//  1. Validate mission, ship composition, and per-mission ship requirements.
//  2. SELECT FOR UPDATE the origin planet, verify ownership.
//  3. Lazy-refresh resources so the deuterium check sees current pools.
//  4. Re-read the planet, the per-planet ship stockpile, and user tech.
//  5. Verify the planet has every requested ship.
//  6. Compute distance, slowest ship speed, flight duration, fuel cost,
//     cargo capacity, and check the cargo fits.
//  7. Resolve target_planet_id if a planet already sits at the target slot.
//  8. Debit fuel + cargo + ships from the planet.
//  9. Insert the fleets row and the per-type fleet_ships rows.
func (s *FleetService) Dispatch(ctx context.Context, userID int64, in FleetDispatchRequest) (*Fleet, error) {
	mission := strings.ToLower(strings.TrimSpace(in.Mission))
	if _, ok := validMissions[mission]; !ok {
		return nil, fmt.Errorf("unknown mission: %q", in.Mission)
	}
	if in.OriginPlanetID <= 0 {
		return nil, errors.New("origin_planet_id is required")
	}
	if in.TargetGalaxy < 1 || in.TargetSystem < 1 || in.TargetPosition < 1 || in.TargetPosition > 15 {
		return nil, errors.New("target coordinates out of range")
	}
	if len(in.Ships) == 0 {
		return nil, errors.New("at least one ship is required")
	}

	// Validate per-ship counts up front so we fail fast.
	shipsTyped := make(map[game.ShipType]int, len(in.Ships))
	for k, count := range in.Ships {
		if count <= 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(k))
		st := game.ShipType(key)
		if _, ok := game.ShipStats[st]; !ok {
			return nil, fmt.Errorf("unknown ship: %q", k)
		}
		shipsTyped[st] = count
	}
	if len(shipsTyped) == 0 {
		return nil, errors.New("at least one ship is required")
	}

	// Mission-specific ship requirements. We keep these small and explicit so
	// the API surfaces a clear error rather than silently accepting nonsense.
	if err := requireMissionShips(mission, shipsTyped); err != nil {
		return nil, err
	}

	// Normalise cargo. Negative values are rejected; nil map becomes empty.
	cargoMetal, cargoCrystal, cargoDeut := 0, 0, 0
	for k, v := range in.Cargo {
		if v < 0 {
			return nil, errors.New("cargo amounts must be non-negative")
		}
		switch strings.ToLower(strings.TrimSpace(k)) {
		case "metal":
			cargoMetal = v
		case "crystal":
			cargoCrystal = v
		case "deuterium":
			cargoDeut = v
		default:
			return nil, fmt.Errorf("unknown cargo key: %q", k)
		}
	}
	cargoSum := cargoMetal + cargoCrystal + cargoDeut

	speedPct := in.SpeedPercent
	if speedPct <= 0 {
		speedPct = 100
	}
	if speedPct < 10 || speedPct > 100 {
		return nil, errors.New("speed_percent must be between 10 and 100")
	}

	var out *Fleet
	err := store.InTx(ctx, s.app.Pool, func(tx pgx.Tx) error {
		qtx := s.app.Queries.WithTx(tx)
		planet, err := qtx.GetPlanetForUpdate(ctx, in.OriginPlanetID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return ErrNotFound
			}
			return err
		}
		if planet.OwnerUserID != userID {
			return ErrForbidden
		}

		// Self-target with mission=attack would otherwise hit distance=5 and
		// pass; refuse it explicitly so the resolver never has to handle it.
		if planet.Galaxy == in.TargetGalaxy && planet.System == in.TargetSystem && planet.Position == in.TargetPosition {
			return errors.New("fleet target cannot be the origin planet")
		}

		// Refresh stockpiles so the fuel/cargo check sees current pools.
		if err := s.app.Resources.RefreshInTx(ctx, qtx, planet.ID); err != nil {
			return err
		}
		planet, err = qtx.GetPlanetForUpdate(ctx, in.OriginPlanetID)
		if err != nil {
			return err
		}

		// Verify every requested ship is present on the planet.
		stockpile, err := qtx.ListShipsForPlanet(ctx, planet.ID)
		if err != nil {
			return err
		}
		for st, count := range shipsTyped {
			if stockpile[string(st)] < count {
				return fmt.Errorf("not enough %s on planet (have %d, need %d)", st, stockpile[string(st)], count)
			}
		}

		// Load research and convert to the typed map fleet formulas expect.
		researches, err := qtx.ListResearchesForUser(ctx, userID)
		if err != nil {
			return err
		}
		techs := toTechMap(researches)

		// Universe drives the fleet-speed multiplier.
		universe, err := qtx.GetUniverse(ctx, planet.UniverseID)
		if err != nil {
			return err
		}
		if in.TargetGalaxy > universe.GalaxiesCount || in.TargetSystem > universe.SystemsCount {
			return errors.New("target outside universe bounds")
		}

		distance := game.Distance(
			planet.Galaxy, planet.System, planet.Position,
			in.TargetGalaxy, in.TargetSystem, in.TargetPosition,
		)
		fleetSpeed := game.SlowestShipSpeed(shipsTyped, techs)
		seconds := game.FlightDurationSeconds(distance, fleetSpeed, universe.SpeedFleet, speedPct)
		fuel := game.FleetFuelConsumption(shipsTyped, distance, speedPct)

		capacity := game.FleetCargoCapacity(shipsTyped)
		if cargoSum > capacity {
			return fmt.Errorf("cargo %d exceeds fleet capacity %d", cargoSum, capacity)
		}

		// Check the planet can pay for fuel + cargo. Fuel is always deuterium.
		needMetal := float64(cargoMetal)
		needCrystal := float64(cargoCrystal)
		needDeut := float64(cargoDeut + fuel)
		if planet.Metal < needMetal || planet.Crystal < needCrystal || planet.Deuterium < needDeut {
			return ErrInsufficientResources
		}

		// Resolve target_planet_id if a planet sits in the destination slot.
		// Stored as a nullable so empty slots stay empty (colonize creates one
		// later in the scheduler).
		var targetPlanetID *int64
		if tp, err := qtx.GetPlanetByCoord(ctx, planet.UniverseID, in.TargetGalaxy, in.TargetSystem, in.TargetPosition); err == nil {
			id := tp.ID
			targetPlanetID = &id
		} else if !errors.Is(err, store.ErrNotFound) {
			return err
		}

		// Deduct fuel + cargo from the planet stockpile.
		now := time.Now().UTC()
		newMetal := planet.Metal - needMetal
		newCrystal := planet.Crystal - needCrystal
		newDeut := planet.Deuterium - needDeut
		if err := qtx.UpdatePlanetResources(ctx, planet.ID, newMetal, newCrystal, newDeut, now); err != nil {
			return err
		}

		// Move the ships from the planet stockpile into the fleet rows.
		for st, count := range shipsTyped {
			if err := qtx.AddShips(ctx, planet.ID, string(st), -count); err != nil {
				return err
			}
		}

		arrival := now.Add(time.Duration(seconds) * time.Second)
		row, err := qtx.InsertFleet(ctx, &store.Fleet{
			OwnerID:        userID,
			OriginPlanetID: planet.ID,
			Mission:        mission,
			Status:         "outbound",
			UniverseID:     planet.UniverseID,
			TargetGalaxy:   in.TargetGalaxy,
			TargetSystem:   in.TargetSystem,
			TargetPosition: in.TargetPosition,
			TargetPlanetID: targetPlanetID,
			SpeedPercent:   speedPct,
			DepartureAt:    now,
			ArrivalAt:      arrival,
			ReturnAt:       nil,
			CargoMetal:     cargoMetal,
			CargoCrystal:   cargoCrystal,
			CargoDeuterium: cargoDeut,
			FuelCost:       fuel,
		})
		if err != nil {
			return err
		}
		for st, count := range shipsTyped {
			if err := qtx.InsertFleetShip(ctx, row.ID, string(st), count); err != nil {
				return err
			}
		}

		// Build the public view directly so callers do not have to re-query
		// fleet_ships for the freshly-inserted row.
		shipsOut := make(map[string]int, len(shipsTyped))
		for st, count := range shipsTyped {
			shipsOut[string(st)] = count
		}
		out = fleetToPublic(row, shipsOut)
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Notify the player so the TUI can update its fleet list without polling.
	if s.app.Events != nil {
		s.app.Events.Broadcast(userID, "fleet.dispatched", map[string]any{
			"fleet_id":        out.ID,
			"mission":         out.Mission,
			"arrival_at":      out.ArrivalAt,
			"origin_planet":   out.OriginPlanetID,
			"target_galaxy":   out.TargetGalaxy,
			"target_system":   out.TargetSystem,
			"target_position": out.TargetPosition,
		})
	}
	return out, nil
}

// List returns the user's in-flight fleets (outbound, holding, returning).
func (s *FleetService) List(ctx context.Context, userID int64) ([]Fleet, error) {
	rows, err := s.app.Queries.ListActiveFleetsByOwner(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]Fleet, 0, len(rows))
	for i := range rows {
		ships, err := s.app.Queries.ListFleetShips(ctx, rows[i].ID)
		if err != nil {
			return nil, err
		}
		out = append(out, *fleetToPublic(&rows[i], ships))
	}
	return out, nil
}

// Recall turns an outbound fleet around. The flight time already spent becomes
// the return time, so the fleet retraces its path symmetrically. The scheduler
// will skip it because ListDueArrivals filters on status='outbound'.
func (s *FleetService) Recall(ctx context.Context, userID, fleetID int64) (*Fleet, error) {
	var out *Fleet
	err := store.InTx(ctx, s.app.Pool, func(tx pgx.Tx) error {
		qtx := s.app.Queries.WithTx(tx)
		fleet, err := qtx.GetFleet(ctx, fleetID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return ErrNotFound
			}
			return err
		}
		if fleet.OwnerID != userID {
			return ErrForbidden
		}
		if fleet.Status != "outbound" {
			return fmt.Errorf("fleet is not recallable in state %q", fleet.Status)
		}

		now := time.Now().UTC()
		elapsed := now.Sub(fleet.DepartureAt)
		if elapsed < 0 {
			elapsed = 0
		}
		returnAt := now.Add(elapsed)

		if err := qtx.SetFleetStatus(ctx, fleet.ID, "returning"); err != nil {
			return err
		}
		if err := qtx.SetFleetReturn(ctx, fleet.ID, returnAt); err != nil {
			return err
		}

		// Re-read for the freshly-applied status / return_at.
		fleet, err = qtx.GetFleet(ctx, fleet.ID)
		if err != nil {
			return err
		}
		ships, err := qtx.ListFleetShips(ctx, fleet.ID)
		if err != nil {
			return err
		}
		out = fleetToPublic(fleet, ships)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if s.app.Events != nil {
		s.app.Events.Broadcast(userID, "fleet.recalled", map[string]any{
			"fleet_id":  out.ID,
			"return_at": out.ReturnAt,
		})
	}
	return out, nil
}

// requireMissionShips enforces the minimum ship type for missions that need a
// specific hull (colony ship for colonize, recycler for recycle, probe for
// espionage). Other missions accept any combination.
func requireMissionShips(mission string, ships map[game.ShipType]int) error {
	switch mission {
	case "colonize":
		if ships[game.ShipColonyShip] < 1 {
			return errors.New("colonize requires at least one colony_ship")
		}
	case "recycle":
		if ships[game.ShipRecycler] < 1 {
			return errors.New("recycle requires at least one recycler")
		}
	case "espionage":
		if ships[game.ShipEspionageProbe] < 1 {
			return errors.New("espionage requires at least one espionage_probe")
		}
	}
	return nil
}

// fleetToPublic builds the public Fleet view from a store row plus its ship
// composition. Cargo is omitted when zero so the JSON stays tidy.
func fleetToPublic(f *store.Fleet, ships map[string]int) *Fleet {
	cargo := map[string]int{}
	if f.CargoMetal > 0 {
		cargo["metal"] = f.CargoMetal
	}
	if f.CargoCrystal > 0 {
		cargo["crystal"] = f.CargoCrystal
	}
	if f.CargoDeuterium > 0 {
		cargo["deuterium"] = f.CargoDeuterium
	}
	out := &Fleet{
		ID:             f.ID,
		UserID:         f.OwnerID,
		OriginPlanetID: f.OriginPlanetID,
		TargetGalaxy:   f.TargetGalaxy,
		TargetSystem:   f.TargetSystem,
		TargetPosition: f.TargetPosition,
		Mission:        f.Mission,
		State:          f.Status,
		DepartureAt:    f.DepartureAt,
		ArrivalAt:      f.ArrivalAt,
		ReturnAt:       f.ReturnAt,
		Ships:          ships,
	}
	if len(cargo) > 0 {
		out.Cargo = cargo
	}
	return out
}
