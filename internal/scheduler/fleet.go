// Fleet arrival + return processing for the scheduler.
//
// Two new sweeps run every tick:
//
//  1. arrivalSweep claims rows from ListDueArrivals (status='outbound',
//     arrival_at <= now, FOR UPDATE SKIP LOCKED) and dispatches each fleet by
//     mission. Combat, transport drop-off, deploy hand-off, colonization,
//     espionage, and recycle each have their own resolver.
//
//  2. returnSweep claims rows from ListDueReturns (status='returning',
//     return_at <= now, FOR UPDATE SKIP LOCKED) and unloads each fleet at its
//     origin planet: ships are added back to planet_ships, cargo is credited
//     to the planet's resource pools.
//
// Both sweeps run inside one transaction per batch so the SKIP LOCKED claim
// and the side effects commit together. Per-fleet errors are logged and the
// row is skipped (left for a later sweep) rather than aborting the whole
// batch.
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"strings"
	"time"

	"github.com/cobanov/terminal-army-go/internal/game"
	"github.com/cobanov/terminal-army-go/internal/store"
	"github.com/jackc/pgx/v5"
)

// arrivalSweep processes fleets that just reached their target.
func (s *Scheduler) arrivalSweep(ctx context.Context) {
	err := store.InTx(ctx, s.app.Pool, func(tx pgx.Tx) error {
		qtx := s.app.Queries.WithTx(tx)
		due, err := qtx.ListDueArrivals(ctx, time.Now().UTC(), batchSize)
		if err != nil {
			return err
		}
		for i := range due {
			f := &due[i]
			if err := s.handleArrival(ctx, qtx, f); err != nil {
				slog.Error("fleet arrival failed",
					"fleet_id", f.ID, "mission", f.Mission, "err", err)
				continue
			}
		}
		return nil
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("fleet arrival sweep failed", "err", err)
	}
}

// returnSweep processes fleets coming back to origin.
func (s *Scheduler) returnSweep(ctx context.Context) {
	err := store.InTx(ctx, s.app.Pool, func(tx pgx.Tx) error {
		qtx := s.app.Queries.WithTx(tx)
		due, err := qtx.ListDueReturns(ctx, time.Now().UTC(), batchSize)
		if err != nil {
			return err
		}
		for i := range due {
			f := &due[i]
			if err := s.handleReturn(ctx, qtx, f); err != nil {
				slog.Error("fleet return failed",
					"fleet_id", f.ID, "mission", f.Mission, "err", err)
				continue
			}
		}
		return nil
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("fleet return sweep failed", "err", err)
	}
}

// handleArrival routes by mission. Each handler is responsible for calling
// MarkArrivalProcessed (or DeleteFleet for self-destruct cases) so the row
// does not stay in ListDueArrivals on the next tick.
func (s *Scheduler) handleArrival(ctx context.Context, qtx *store.Queries, f *store.Fleet) error {
	switch strings.ToLower(f.Mission) {
	case "attack":
		return s.resolveAttack(ctx, qtx, f)
	case "transport":
		return s.resolveTransport(ctx, qtx, f)
	case "deploy":
		return s.resolveDeploy(ctx, qtx, f)
	case "colonize":
		return s.resolveColonize(ctx, qtx, f)
	case "espionage":
		return s.resolveEspionage(ctx, qtx, f)
	case "recycle":
		return s.resolveRecycle(ctx, qtx, f)
	default:
		return fmt.Errorf("unknown mission: %s", f.Mission)
	}
}

// ------ attack -------------------------------------------------------------
//
// Combat resolution: single-round simplified model. Attacker brings only the
// ships in fleet_ships; defender brings every ship + every defense parked on
// the target planet. Both sides apply weapons/shielding/armour tech bonuses.
// The winner takes loot capped at the smaller of (50% of unprotected
// resources, remaining cargo capacity). Defender ship destruction generates
// 30% debris (defenses leave none). Both sides receive a combat report. The
// attacker fleet flips to status='returning' and races home along the same
// flight time it took to arrive.
func (s *Scheduler) resolveAttack(ctx context.Context, qtx *store.Queries, f *store.Fleet) error {
	fleetShips, err := qtx.ListFleetShips(ctx, f.ID)
	if err != nil {
		return err
	}
	if len(fleetShips) == 0 {
		// Edge case: empty fleet somehow made it through dispatch. Mark
		// returning with empty hands; the return handler will no-op.
		return s.markAttackReturning(ctx, qtx, f)
	}

	attackerTech, err := qtx.ListResearchesForUser(ctx, f.OwnerID)
	if err != nil {
		return err
	}

	// Target may be an empty slot (planet deleted, never colonised). Treat as
	// an undefended cargo run: attacker gets nothing, fleet just turns around.
	if f.TargetPlanetID == nil {
		return s.markAttackReturning(ctx, qtx, f)
	}
	target, err := qtx.GetPlanetForUpdate(ctx, *f.TargetPlanetID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return s.markAttackReturning(ctx, qtx, f)
		}
		return err
	}

	// Refresh defender resources so loot is computed from up-to-date pools.
	if err := s.app.Resources.RefreshInTx(ctx, qtx, target.ID); err != nil {
		return err
	}
	target, err = qtx.GetPlanetForUpdate(ctx, *f.TargetPlanetID)
	if err != nil {
		return err
	}

	defShipMap, err := qtx.ListShipsForPlanet(ctx, target.ID)
	if err != nil {
		return err
	}
	defDefMap, err := qtx.ListDefensesForPlanet(ctx, target.ID)
	if err != nil {
		return err
	}
	defTech, err := qtx.ListResearchesForUser(ctx, target.OwnerUserID)
	if err != nil {
		return err
	}

	attackerUnits := game.BuildUnitsFromShips(
		toShipMap(fleetShips),
		attackerTech[string(game.TechWeapons)],
		attackerTech[string(game.TechShielding)],
		attackerTech[string(game.TechArmour)],
	)
	defShipUnits := game.BuildUnitsFromShips(
		toShipMap(defShipMap),
		defTech[string(game.TechWeapons)],
		defTech[string(game.TechShielding)],
		defTech[string(game.TechArmour)],
	)
	defDefenseUnits := game.BuildUnitsFromDefenses(
		toDefenseMap(defDefMap),
		defTech[string(game.TechWeapons)],
		defTech[string(game.TechShielding)],
		defTech[string(game.TechArmour)],
	)

	result := game.SimulateCombat(attackerUnits, defShipUnits, defDefenseUnits)

	// Apply attacker losses to fleet_ships. fleet_ships has no GREATEST guard,
	// so we update each row directly.
	survivingFleetShips := map[string]int{}
	for name, before := range fleetShips {
		killed := result.AttackerDestroyed[name]
		remaining := before - killed
		if remaining < 0 {
			remaining = 0
		}
		survivingFleetShips[name] = remaining
		if remaining == before {
			continue
		}
		// fleet_ships does not have an AddShips equivalent, so we overwrite via
		// INSERT ... ON CONFLICT DO UPDATE on (fleet_id, ship_type).
		if err := setFleetShipCount(ctx, qtx, f.ID, name, remaining); err != nil {
			return err
		}
	}

	// Apply defender losses.
	for name, killed := range result.DefenderShipsDestroyed {
		if killed > 0 {
			if err := qtx.AddShips(ctx, target.ID, name, -killed); err != nil {
				return err
			}
		}
	}
	for name, killed := range result.DefenderDefensesDestroyed {
		if killed > 0 {
			if err := qtx.AddDefenses(ctx, target.ID, name, -killed); err != nil {
				return err
			}
		}
	}

	// Loot only if the attacker won and has survivors.
	totalSurvivors := 0
	for _, n := range survivingFleetShips {
		totalSurvivors += n
	}

	lootMetal, lootCrystal, lootDeut := 0, 0, 0
	if result.Winner == "attacker" && totalSurvivors > 0 {
		// Half of each resource pool is lootable.
		lootMetal = int(target.Metal / 2)
		lootCrystal = int(target.Crystal / 2)
		lootDeut = int(target.Deuterium / 2)

		// Cargo capacity remaining = capacity minus what fleet already carries.
		surviveTyped := make(map[game.ShipType]int, len(survivingFleetShips))
		for k, v := range survivingFleetShips {
			surviveTyped[game.ShipType(k)] = v
		}
		capRemaining := game.FleetCargoCapacity(surviveTyped) -
			(f.CargoMetal + f.CargoCrystal + f.CargoDeuterium)
		if capRemaining < 0 {
			capRemaining = 0
		}

		// Distribute capacity across the three resources, metal first then
		// crystal then deuterium (OGame-style priority).
		take := capRemaining
		takeMetal := minInt(lootMetal, take)
		take -= takeMetal
		takeCrystal := minInt(lootCrystal, take)
		take -= takeCrystal
		takeDeut := minInt(lootDeut, take)

		lootMetal, lootCrystal, lootDeut = takeMetal, takeCrystal, takeDeut

		// Debit defender pools, credit fleet cargo.
		newM := target.Metal - float64(lootMetal)
		newC := target.Crystal - float64(lootCrystal)
		newD := target.Deuterium - float64(lootDeut)
		now := time.Now().UTC()
		if err := qtx.UpdatePlanetResources(ctx, target.ID, newM, newC, newD, now); err != nil {
			return err
		}
		if err := qtx.UpdateFleetCargo(ctx, f.ID,
			f.CargoMetal+lootMetal,
			f.CargoCrystal+lootCrystal,
			f.CargoDeuterium+lootDeut,
		); err != nil {
			return err
		}
	}

	// Combat reports. Attacker and defender both get one; payload carries the
	// full result so the TUI can render it however it wants.
	reportPayload := map[string]any{
		"winner":                      result.Winner,
		"attacker_total_attack":       result.AttackerTotalAttack,
		"defender_total_attack":       result.DefenderTotalAttack,
		"attacker_remaining":          result.AttackerRemaining,
		"attacker_destroyed":          result.AttackerDestroyed,
		"defender_ships_remaining":    result.DefenderShipsRemaining,
		"defender_ships_destroyed":    result.DefenderShipsDestroyed,
		"defender_defenses_remaining": result.DefenderDefensesRemaining,
		"defender_defenses_destroyed": result.DefenderDefensesDestroyed,
		"debris_metal":                result.DebrisMetal,
		"debris_crystal":              result.DebrisCrystal,
		"loot_metal":                  lootMetal,
		"loot_crystal":                lootCrystal,
		"loot_deuterium":              lootDeut,
		"target_galaxy":               f.TargetGalaxy,
		"target_system":               f.TargetSystem,
		"target_position":             f.TargetPosition,
	}
	title := fmt.Sprintf("Combat report %d:%d:%d", f.TargetGalaxy, f.TargetSystem, f.TargetPosition)
	body := fmt.Sprintf("Winner: %s. Attacker losses: %s. Defender losses: %s.",
		result.Winner,
		summariseLosses(result.AttackerDestroyed),
		summariseLosses(mergeLossMaps(result.DefenderShipsDestroyed, result.DefenderDefensesDestroyed)),
	)
	if _, err := qtx.InsertReport(ctx, &store.Report{
		OwnerID:        f.OwnerID,
		ReportType:     "combat",
		Title:          title,
		Body:           body,
		Payload:        reportPayload,
		TargetGalaxy:   f.TargetGalaxy,
		TargetSystem:   f.TargetSystem,
		TargetPosition: f.TargetPosition,
	}); err != nil {
		return err
	}
	if _, err := qtx.InsertReport(ctx, &store.Report{
		OwnerID:        target.OwnerUserID,
		ReportType:     "combat",
		Title:          title,
		Body:           body,
		Payload:        reportPayload,
		TargetGalaxy:   f.TargetGalaxy,
		TargetSystem:   f.TargetSystem,
		TargetPosition: f.TargetPosition,
	}); err != nil {
		return err
	}

	// Defender notification so they see the attack without polling reports.
	if _, err := qtx.InsertMessage(ctx, nil, target.OwnerUserID,
		fmt.Sprintf("Attack on %s", target.Name),
		fmt.Sprintf("Your planet at %d:%d:%d was attacked. See combat report for details.",
			target.Galaxy, target.System, target.Position),
		"combat",
	); err != nil {
		return err
	}

	// Fire-and-forget websocket events.
	s.broadcastFleetEvent(f.OwnerID, "fleet.combat", map[string]any{
		"fleet_id": f.ID,
		"winner":   result.Winner,
		"loot":     map[string]int{"metal": lootMetal, "crystal": lootCrystal, "deuterium": lootDeut},
	})
	s.broadcastFleetEvent(target.OwnerUserID, "planet.attacked", map[string]any{
		"planet_id": target.ID,
		"winner":    result.Winner,
	})

	// If every attacker ship died there is nothing left to return; delete the
	// fleet so it does not loop in the return sweep.
	if totalSurvivors == 0 {
		return qtx.DeleteFleet(ctx, f.ID)
	}
	return s.markAttackReturning(ctx, qtx, f)
}

// markAttackReturning flips the fleet to status='returning' with return_at =
// now + (now - departure_at) so the return leg takes the same time as the
// outbound leg.
func (s *Scheduler) markAttackReturning(ctx context.Context, qtx *store.Queries, f *store.Fleet) error {
	now := time.Now().UTC()
	elapsed := now.Sub(f.DepartureAt)
	if elapsed < 0 {
		elapsed = 0
	}
	ret := now.Add(elapsed)
	return qtx.MarkArrivalProcessed(ctx, f.ID, "returning", &ret)
}

// ------ transport ----------------------------------------------------------
//
// Transport: drop cargo at target (if owned by the fleet owner OR if the
// target accepts free deliveries). MVP keeps it simple: any planet receives
// the cargo. Returning leg is symmetric to attack.
func (s *Scheduler) resolveTransport(ctx context.Context, qtx *store.Queries, f *store.Fleet) error {
	if f.TargetPlanetID == nil {
		// No planet at target slot; cargo cannot be delivered. Return with
		// it intact.
		return s.markAttackReturning(ctx, qtx, f)
	}
	target, err := qtx.GetPlanetForUpdate(ctx, *f.TargetPlanetID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return s.markAttackReturning(ctx, qtx, f)
		}
		return err
	}
	if err := s.app.Resources.RefreshInTx(ctx, qtx, target.ID); err != nil {
		return err
	}
	target, err = qtx.GetPlanetForUpdate(ctx, *f.TargetPlanetID)
	if err != nil {
		return err
	}

	cargoM := f.CargoMetal
	cargoC := f.CargoCrystal
	cargoD := f.CargoDeuterium
	if cargoM > 0 || cargoC > 0 || cargoD > 0 {
		now := time.Now().UTC()
		if err := qtx.UpdatePlanetResources(ctx, target.ID,
			target.Metal+float64(cargoM),
			target.Crystal+float64(cargoC),
			target.Deuterium+float64(cargoD),
			now,
		); err != nil {
			return err
		}
		if err := qtx.UpdateFleetCargo(ctx, f.ID, 0, 0, 0); err != nil {
			return err
		}
	}

	// Notify target owner if they are not the same user as the sender.
	if target.OwnerUserID != f.OwnerID {
		if _, err := qtx.InsertMessage(ctx, &f.OwnerID, target.OwnerUserID,
			fmt.Sprintf("Transport delivered to %s", target.Name),
			fmt.Sprintf("Received: metal=%d crystal=%d deuterium=%d", cargoM, cargoC, cargoD),
			"transport",
		); err != nil {
			return err
		}
	}

	s.broadcastFleetEvent(f.OwnerID, "fleet.delivered", map[string]any{
		"fleet_id":  f.ID,
		"target_id": target.ID,
	})

	return s.markAttackReturning(ctx, qtx, f)
}

// ------ deploy -------------------------------------------------------------
//
// Deploy: leaves ships + cargo at the target planet (which must be owned by
// the fleet owner). The fleet itself is deleted because there is no return
// leg.
func (s *Scheduler) resolveDeploy(ctx context.Context, qtx *store.Queries, f *store.Fleet) error {
	if f.TargetPlanetID == nil {
		// No planet at target slot; cannot deploy. Bounce back.
		return s.markAttackReturning(ctx, qtx, f)
	}
	target, err := qtx.GetPlanetForUpdate(ctx, *f.TargetPlanetID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return s.markAttackReturning(ctx, qtx, f)
		}
		return err
	}
	if target.OwnerUserID != f.OwnerID {
		// Cannot deploy to another player; treat as a transport, drop cargo,
		// keep ships, fly home.
		return s.resolveTransport(ctx, qtx, f)
	}
	if err := s.app.Resources.RefreshInTx(ctx, qtx, target.ID); err != nil {
		return err
	}
	target, err = qtx.GetPlanetForUpdate(ctx, *f.TargetPlanetID)
	if err != nil {
		return err
	}

	ships, err := qtx.ListFleetShips(ctx, f.ID)
	if err != nil {
		return err
	}
	for name, count := range ships {
		if count > 0 {
			if err := qtx.AddShips(ctx, target.ID, name, count); err != nil {
				return err
			}
		}
	}
	if f.CargoMetal > 0 || f.CargoCrystal > 0 || f.CargoDeuterium > 0 {
		now := time.Now().UTC()
		if err := qtx.UpdatePlanetResources(ctx, target.ID,
			target.Metal+float64(f.CargoMetal),
			target.Crystal+float64(f.CargoCrystal),
			target.Deuterium+float64(f.CargoDeuterium),
			now,
		); err != nil {
			return err
		}
	}

	s.broadcastFleetEvent(f.OwnerID, "fleet.deployed", map[string]any{
		"fleet_id":  f.ID,
		"target_id": target.ID,
	})

	// Fleet has no return leg; delete it outright.
	return qtx.DeleteFleet(ctx, f.ID)
}

// ------ colonize -----------------------------------------------------------
//
// Colonize: if the target slot is empty, consume one colony ship from the
// fleet and create a new planet owned by the fleet owner. Surviving ships +
// remaining cargo are transferred to the new planet. If the slot is taken
// (somebody else colonised it first or the player retried), the colony ship
// is lost and a message explains why.
func (s *Scheduler) resolveColonize(ctx context.Context, qtx *store.Queries, f *store.Fleet) error {
	universe, err := qtx.GetUniverse(ctx, f.UniverseID)
	if err != nil {
		return err
	}
	_ = universe

	// Re-check slot occupancy under transaction lock. We don't have a
	// SELECT FOR UPDATE on the slot itself; rely on the unique constraint at
	// insert time as the authoritative check.
	occupied, err := qtx.GetPlanetByCoord(ctx, f.UniverseID, f.TargetGalaxy, f.TargetSystem, f.TargetPosition)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return err
	}
	if occupied != nil {
		// Slot taken; colonization fails. Bounce back with everything intact.
		if _, mErr := qtx.InsertMessage(ctx, nil, f.OwnerID,
			"Colonization failed",
			fmt.Sprintf("Slot %d:%d:%d is already occupied.", f.TargetGalaxy, f.TargetSystem, f.TargetPosition),
			"colonization",
		); mErr != nil {
			return mErr
		}
		return s.markAttackReturning(ctx, qtx, f)
	}

	// Roll planet attributes from the position.
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	attrs, err := game.GeneratePlanetAttributes(f.TargetPosition, rng)
	if err != nil {
		return err
	}

	code := generatePlanetCode(f.UniverseID, f.TargetGalaxy, f.TargetSystem, f.TargetPosition)
	name := fmt.Sprintf("Colony %d:%d:%d", f.TargetGalaxy, f.TargetSystem, f.TargetPosition)

	created, err := qtx.CreatePlanet(ctx, &store.Planet{
		Code:                   code,
		OwnerUserID:            f.OwnerID,
		UniverseID:             f.UniverseID,
		Galaxy:                 f.TargetGalaxy,
		System:                 f.TargetSystem,
		Position:               f.TargetPosition,
		Name:                   name,
		FieldsUsed:             0,
		FieldsTotal:            attrs.FieldsTotal,
		TempMin:                attrs.TempMin,
		TempMax:                attrs.TempMax,
		Metal:                  float64(f.CargoMetal),
		Crystal:                float64(f.CargoCrystal),
		Deuterium:              float64(f.CargoDeuterium),
		ResourcesLastUpdatedAt: time.Now().UTC(),
	})
	if err != nil {
		// Unique constraint race: someone colonised first. Fall back to the
		// "slot taken" path.
		if store.IsSlotTaken(err) {
			if _, mErr := qtx.InsertMessage(ctx, nil, f.OwnerID,
				"Colonization failed",
				fmt.Sprintf("Slot %d:%d:%d was just taken.", f.TargetGalaxy, f.TargetSystem, f.TargetPosition),
				"colonization",
			); mErr != nil {
				return mErr
			}
			return s.markAttackReturning(ctx, qtx, f)
		}
		return err
	}

	// Transfer surviving ships (minus one colony ship) to the new planet.
	ships, err := qtx.ListFleetShips(ctx, f.ID)
	if err != nil {
		return err
	}
	// One colony ship is consumed by the colonization itself.
	if ships[string(game.ShipColonyShip)] > 0 {
		ships[string(game.ShipColonyShip)] -= 1
	}
	for name, count := range ships {
		if count > 0 {
			if err := qtx.AddShips(ctx, created.ID, name, count); err != nil {
				return err
			}
		}
	}

	if _, err := qtx.InsertMessage(ctx, nil, f.OwnerID,
		fmt.Sprintf("New colony at %d:%d:%d", f.TargetGalaxy, f.TargetSystem, f.TargetPosition),
		fmt.Sprintf("Planet '%s' was founded successfully.", name),
		"colonization",
	); err != nil {
		return err
	}

	s.broadcastFleetEvent(f.OwnerID, "planet.colonized", map[string]any{
		"planet_id": created.ID,
		"galaxy":    f.TargetGalaxy,
		"system":    f.TargetSystem,
		"position":  f.TargetPosition,
	})

	// Fleet ends its life at the new planet; no return leg.
	return qtx.DeleteFleet(ctx, f.ID)
}

// ------ espionage ----------------------------------------------------------
//
// Espionage: roll info level from probe count and tech delta. If counter-
// espionage rolls succeed, the probes are destroyed and the defender gets a
// "spy attempt" notification. Either way an espionage report lands in the
// attacker's inbox.
func (s *Scheduler) resolveEspionage(ctx context.Context, qtx *store.Queries, f *store.Fleet) error {
	atkTech, err := qtx.ListResearchesForUser(ctx, f.OwnerID)
	if err != nil {
		return err
	}
	probes := 0
	fleetShips, err := qtx.ListFleetShips(ctx, f.ID)
	if err != nil {
		return err
	}
	probes = fleetShips[string(game.ShipEspionageProbe)]

	atkSpy := atkTech[string(game.TechEspionage)]

	// Target may not exist (slot empty).
	if f.TargetPlanetID == nil {
		if _, err := qtx.InsertReport(ctx, &store.Report{
			OwnerID:        f.OwnerID,
			ReportType:     "espionage",
			Title:          fmt.Sprintf("Espionage report %d:%d:%d", f.TargetGalaxy, f.TargetSystem, f.TargetPosition),
			Body:           "Target slot is empty.",
			Payload:        map[string]any{"empty": true},
			TargetGalaxy:   f.TargetGalaxy,
			TargetSystem:   f.TargetSystem,
			TargetPosition: f.TargetPosition,
		}); err != nil {
			return err
		}
		return s.markAttackReturning(ctx, qtx, f)
	}

	target, err := qtx.GetPlanetForUpdate(ctx, *f.TargetPlanetID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return s.markAttackReturning(ctx, qtx, f)
		}
		return err
	}
	if err := s.app.Resources.RefreshInTx(ctx, qtx, target.ID); err != nil {
		return err
	}
	target, err = qtx.GetPlanetForUpdate(ctx, *f.TargetPlanetID)
	if err != nil {
		return err
	}

	defTech, err := qtx.ListResearchesForUser(ctx, target.OwnerUserID)
	if err != nil {
		return err
	}
	defSpy := defTech[string(game.TechEspionage)]

	level := game.EspionageInfoLevel(probes, atkSpy, defSpy)
	counter := game.CounterEspionageChance(probes, defSpy, atkSpy)

	payload := map[string]any{
		"info_level":     level,
		"resources":      map[string]float64{"metal": target.Metal, "crystal": target.Crystal, "deuterium": target.Deuterium},
		"counter_chance": counter,
	}
	if level >= 2 {
		shipMap, err := qtx.ListShipsForPlanet(ctx, target.ID)
		if err != nil {
			return err
		}
		payload["ships"] = shipMap
	}
	if level >= 3 {
		defMap, err := qtx.ListDefensesForPlanet(ctx, target.ID)
		if err != nil {
			return err
		}
		payload["defenses"] = defMap
	}
	if level >= 4 {
		buildings, err := qtx.ListBuildingsForPlanet(ctx, target.ID)
		if err != nil {
			return err
		}
		payload["buildings"] = buildings
	}
	if level >= 5 {
		payload["research"] = defTech
	}

	if _, err := qtx.InsertReport(ctx, &store.Report{
		OwnerID:        f.OwnerID,
		ReportType:     "espionage",
		Title:          fmt.Sprintf("Espionage report %d:%d:%d", f.TargetGalaxy, f.TargetSystem, f.TargetPosition),
		Body:           fmt.Sprintf("Info level %d/5 with %d probes.", level, probes),
		Payload:        payload,
		TargetGalaxy:   f.TargetGalaxy,
		TargetSystem:   f.TargetSystem,
		TargetPosition: f.TargetPosition,
	}); err != nil {
		return err
	}

	// Roll counter-espionage.
	rng := rand.New(rand.NewSource(time.Now().UnixNano() ^ f.ID))
	caught := rng.Float64() < counter
	if caught {
		// Notify defender, destroy fleet outright.
		if _, err := qtx.InsertMessage(ctx, nil, target.OwnerUserID,
			fmt.Sprintf("Spy attempt on %s", target.Name),
			fmt.Sprintf("Hostile probes were detected and destroyed at %d:%d:%d.",
				target.Galaxy, target.System, target.Position),
			"espionage",
		); err != nil {
			return err
		}
		s.broadcastFleetEvent(target.OwnerUserID, "planet.spied", map[string]any{
			"planet_id": target.ID,
			"caught":    true,
		})
		return qtx.DeleteFleet(ctx, f.ID)
	}

	s.broadcastFleetEvent(f.OwnerID, "fleet.espionage", map[string]any{
		"fleet_id":   f.ID,
		"info_level": level,
	})
	return s.markAttackReturning(ctx, qtx, f)
}

// ------ recycle ------------------------------------------------------------
//
// Recycle: harvest debris field at the target. MVP keeps it abstract: no
// per-coordinate debris table yet, so the recycler simply turns around with
// zero loot. The plumbing is in place for when debris tables ship.
func (s *Scheduler) resolveRecycle(ctx context.Context, qtx *store.Queries, f *store.Fleet) error {
	// TODO(post-MVP): pull from a debris_fields table and split between metal
	// and crystal based on recycler cargo. For now we just go home.
	s.broadcastFleetEvent(f.OwnerID, "fleet.recycled", map[string]any{
		"fleet_id": f.ID,
		"loot":     map[string]int{"metal": 0, "crystal": 0},
	})
	return s.markAttackReturning(ctx, qtx, f)
}

// ------ return handler -----------------------------------------------------
//
// handleReturn unloads a fleet back into its origin planet.
func (s *Scheduler) handleReturn(ctx context.Context, qtx *store.Queries, f *store.Fleet) error {
	planet, err := qtx.GetPlanetForUpdate(ctx, f.OriginPlanetID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			// Origin planet vanished (deleted user, lost colony). Drop the
			// fleet on the floor.
			if delErr := qtx.DeleteFleet(ctx, f.ID); delErr != nil {
				return delErr
			}
			return nil
		}
		return err
	}
	if err := s.app.Resources.RefreshInTx(ctx, qtx, planet.ID); err != nil {
		return err
	}
	planet, err = qtx.GetPlanetForUpdate(ctx, f.OriginPlanetID)
	if err != nil {
		return err
	}

	// Re-add ships.
	ships, err := qtx.ListFleetShips(ctx, f.ID)
	if err != nil {
		return err
	}
	for name, count := range ships {
		if count > 0 {
			if err := qtx.AddShips(ctx, planet.ID, name, count); err != nil {
				return err
			}
		}
	}

	// Credit cargo.
	if f.CargoMetal > 0 || f.CargoCrystal > 0 || f.CargoDeuterium > 0 {
		now := time.Now().UTC()
		if err := qtx.UpdatePlanetResources(ctx, planet.ID,
			planet.Metal+float64(f.CargoMetal),
			planet.Crystal+float64(f.CargoCrystal),
			planet.Deuterium+float64(f.CargoDeuterium),
			now,
		); err != nil {
			return err
		}
	}

	if err := qtx.MarkReturnProcessed(ctx, f.ID); err != nil {
		return err
	}

	s.broadcastFleetEvent(f.OwnerID, "fleet.returned", map[string]any{
		"fleet_id":  f.ID,
		"planet_id": planet.ID,
		"cargo": map[string]int{
			"metal":     f.CargoMetal,
			"crystal":   f.CargoCrystal,
			"deuterium": f.CargoDeuterium,
		},
	})
	return nil
}

// ------ helpers ------------------------------------------------------------

// setFleetShipCount writes a new count for one (fleet_id, ship_type) row.
// fleet_ships does not expose a delta helper, so the scheduler updates it
// directly. The combat resolver is the only writer.
func setFleetShipCount(ctx context.Context, qtx *store.Queries, fleetID int64, shipType string, newCount int) error {
	return qtx.SetFleetShipCount(ctx, fleetID, shipType, newCount)
}

// toShipMap converts a string-keyed ship map to game.ShipType keys.
func toShipMap(in map[string]int) map[game.ShipType]int {
	out := make(map[game.ShipType]int, len(in))
	for k, v := range in {
		out[game.ShipType(k)] = v
	}
	return out
}

// toDefenseMap converts a string-keyed defense map to game.DefenseType keys.
func toDefenseMap(in map[string]int) map[game.DefenseType]int {
	out := make(map[game.DefenseType]int, len(in))
	for k, v := range in {
		out[game.DefenseType(k)] = v
	}
	return out
}

// summariseLosses renders a map like {"light_fighter": 5, "cruiser": 1} as
// "light_fighter x5, cruiser x1". Order is not deterministic, that's fine
// for a free-form message body.
func summariseLosses(m map[string]int) string {
	if len(m) == 0 {
		return "none"
	}
	parts := make([]string, 0, len(m))
	for k, v := range m {
		if v <= 0 {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s x%d", k, v))
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ", ")
}

// mergeLossMaps combines two destroyed-count maps without modifying either.
func mergeLossMaps(a, b map[string]int) map[string]int {
	out := make(map[string]int, len(a)+len(b))
	for k, v := range a {
		out[k] += v
	}
	for k, v := range b {
		out[k] += v
	}
	return out
}

// generatePlanetCode builds a short slug from the coordinates.
func generatePlanetCode(universeID int64, g, s, p int) string {
	return fmt.Sprintf("u%d-%d-%d-%d-%d", universeID, g, s, p, time.Now().UnixNano()%100000)
}

// broadcastFleetEvent publishes a fleet event if an EventSink is wired up.
func (s *Scheduler) broadcastFleetEvent(userID int64, event string, payload map[string]any) {
	if s.app.Events == nil {
		return
	}
	s.app.Events.Broadcast(userID, event, payload)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
