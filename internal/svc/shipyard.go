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

// ShipyardService owns ship and defense production. Both items live in the
// same build_queue table as buildings and research, distinguished by the
// queue_type column. The shipyard does not pause for research the way the
// real OGame does (that's a UI nicety we can layer in later); instead the
// per-planet queue cap is the only throttle.
type ShipyardService struct{ app *App }

// QueueShip enqueues a batch of one ship type.
//
// Cost = per-unit cost * count, charged up-front against the planet.
// Duration = per-unit build time * count, chained to the planet's queue tail
// so a ship build does not jump ahead of in-progress upgrades.
func (s *ShipyardService) QueueShip(ctx context.Context, userID, planetID int64, shipKey string, count int) (*QueueItem, error) {
	st, err := parseShipType(shipKey)
	if err != nil {
		return nil, err
	}
	if count < 1 {
		return nil, fmt.Errorf("count must be at least 1")
	}
	if count > game.MaxUnitBatch {
		return nil, fmt.Errorf("count must be at most %d", game.MaxUnitBatch)
	}

	var out *QueueItem
	err = store.InTx(ctx, s.app.Pool, func(tx pgx.Tx) error {
		qtx := s.app.Queries.WithTx(tx)
		planet, err := qtx.GetPlanetForUpdate(ctx, planetID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return ErrNotFound
			}
			return err
		}
		if planet.OwnerUserID != userID {
			return ErrForbidden
		}

		if err := s.app.Resources.RefreshInTx(ctx, qtx, planet.ID); err != nil {
			return err
		}
		planet, err = qtx.GetPlanetForUpdate(ctx, planetID)
		if err != nil {
			return err
		}

		activeCount, err := qtx.CountActiveQueueForPlanet(ctx, planet.ID)
		if err != nil {
			return err
		}
		if activeCount >= game.BuildQueueMaxActive {
			return ErrQueueBusy
		}

		buildings, err := qtx.ListBuildingsForPlanet(ctx, planet.ID)
		if err != nil {
			return err
		}
		researches, err := qtx.ListResearchesForUser(ctx, userID)
		if err != nil {
			return err
		}

		shipyardLevel := buildings[string(game.BuildingShipyard)]
		if ok, missing := game.CheckShipPrereqs(st, shipyardLevel, toTechMap(researches)); !ok {
			return fmt.Errorf("%w: %s", ErrPrerequisiteNotMet, strings.Join(missing, ", "))
		}

		unitMetal, unitCrystal, unitDeut := game.ShipUnitCost(st)
		costMetal := unitMetal * count
		costCrystal := unitCrystal * count
		costDeut := unitDeut * count
		if int(planet.Metal) < costMetal || int(planet.Crystal) < costCrystal || int(planet.Deuterium) < costDeut {
			return ErrInsufficientResources
		}

		universe, err := qtx.GetUniverse(ctx, planet.UniverseID)
		if err != nil {
			return err
		}
		robotics := buildings[string(game.BuildingRoboticsFactory)]
		nanite := buildings[string(game.BuildingNaniteFactory)]
		perUnitSeconds := game.BuildTimeSeconds(unitMetal, unitCrystal, robotics, nanite, float64(universe.SpeedEconomy))
		seconds := perUnitSeconds * count

		now := time.Now().UTC()
		start := now
		pending, err := qtx.ListActiveQueueForPlanet(ctx, planet.ID)
		if err != nil {
			return err
		}
		if len(pending) > 0 {
			tail, err := qtx.LatestActiveQueueFinish(ctx, planet.ID)
			if err != nil {
				return err
			}
			if tail.After(start) {
				start = tail
			}
		}
		finish := start.Add(time.Duration(seconds) * time.Second)

		newMetal := planet.Metal - float64(costMetal)
		newCrystal := planet.Crystal - float64(costCrystal)
		newDeut := planet.Deuterium - float64(costDeut)
		if err := qtx.UpdatePlanetResources(ctx, planet.ID, newMetal, newCrystal, newDeut, now); err != nil {
			return err
		}

		pid := planet.ID
		row, err := qtx.InsertQueueItem(ctx, &store.QueueItem{
			PlanetID:      &pid,
			UserID:        userID,
			QueueType:     "ship",
			ItemKey:       string(st),
			TargetLevel:   0,
			Count:         count,
			CostMetal:     costMetal,
			CostCrystal:   costCrystal,
			CostDeuterium: costDeut,
			StartedAt:     start,
			FinishedAt:    finish,
		})
		if err != nil {
			return err
		}
		out = queueItemToPublic(row)
		return nil
	})
	return out, err
}

// QueueDefense mirrors QueueShip for stationary defenses. Defenses cannot be
// destroyed once built (in OGame they explode then partially rebuild for free,
// but that's combat-resolver scope, not here).
func (s *ShipyardService) QueueDefense(ctx context.Context, userID, planetID int64, defenseKey string, count int) (*QueueItem, error) {
	dt, err := parseDefenseType(defenseKey)
	if err != nil {
		return nil, err
	}
	if count < 1 {
		return nil, fmt.Errorf("count must be at least 1")
	}
	if count > game.MaxUnitBatch {
		return nil, fmt.Errorf("count must be at most %d", game.MaxUnitBatch)
	}

	var out *QueueItem
	err = store.InTx(ctx, s.app.Pool, func(tx pgx.Tx) error {
		qtx := s.app.Queries.WithTx(tx)
		planet, err := qtx.GetPlanetForUpdate(ctx, planetID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return ErrNotFound
			}
			return err
		}
		if planet.OwnerUserID != userID {
			return ErrForbidden
		}

		if err := s.app.Resources.RefreshInTx(ctx, qtx, planet.ID); err != nil {
			return err
		}
		planet, err = qtx.GetPlanetForUpdate(ctx, planetID)
		if err != nil {
			return err
		}

		activeCount, err := qtx.CountActiveQueueForPlanet(ctx, planet.ID)
		if err != nil {
			return err
		}
		if activeCount >= game.BuildQueueMaxActive {
			return ErrQueueBusy
		}

		buildings, err := qtx.ListBuildingsForPlanet(ctx, planet.ID)
		if err != nil {
			return err
		}
		researches, err := qtx.ListResearchesForUser(ctx, userID)
		if err != nil {
			return err
		}

		shipyardLevel := buildings[string(game.BuildingShipyard)]
		if ok, missing := game.CheckDefensePrereqs(dt, shipyardLevel, toTechMap(researches)); !ok {
			return fmt.Errorf("%w: %s", ErrPrerequisiteNotMet, strings.Join(missing, ", "))
		}

		unitMetal, unitCrystal, unitDeut := game.DefenseUnitCost(dt)
		costMetal := unitMetal * count
		costCrystal := unitCrystal * count
		costDeut := unitDeut * count
		if int(planet.Metal) < costMetal || int(planet.Crystal) < costCrystal || int(planet.Deuterium) < costDeut {
			return ErrInsufficientResources
		}

		universe, err := qtx.GetUniverse(ctx, planet.UniverseID)
		if err != nil {
			return err
		}
		robotics := buildings[string(game.BuildingRoboticsFactory)]
		nanite := buildings[string(game.BuildingNaniteFactory)]
		perUnitSeconds := game.BuildTimeSeconds(unitMetal, unitCrystal, robotics, nanite, float64(universe.SpeedEconomy))
		seconds := perUnitSeconds * count

		now := time.Now().UTC()
		start := now
		pending, err := qtx.ListActiveQueueForPlanet(ctx, planet.ID)
		if err != nil {
			return err
		}
		if len(pending) > 0 {
			tail, err := qtx.LatestActiveQueueFinish(ctx, planet.ID)
			if err != nil {
				return err
			}
			if tail.After(start) {
				start = tail
			}
		}
		finish := start.Add(time.Duration(seconds) * time.Second)

		newMetal := planet.Metal - float64(costMetal)
		newCrystal := planet.Crystal - float64(costCrystal)
		newDeut := planet.Deuterium - float64(costDeut)
		if err := qtx.UpdatePlanetResources(ctx, planet.ID, newMetal, newCrystal, newDeut, now); err != nil {
			return err
		}

		pid := planet.ID
		row, err := qtx.InsertQueueItem(ctx, &store.QueueItem{
			PlanetID:      &pid,
			UserID:        userID,
			QueueType:     "defense",
			ItemKey:       string(dt),
			TargetLevel:   0,
			Count:         count,
			CostMetal:     costMetal,
			CostCrystal:   costCrystal,
			CostDeuterium: costDeut,
			StartedAt:     start,
			FinishedAt:    finish,
		})
		if err != nil {
			return err
		}
		out = queueItemToPublic(row)
		return nil
	})
	return out, err
}

// parseShipType validates a user-supplied ship key.
func parseShipType(key string) (game.ShipType, error) {
	key = strings.ToLower(strings.TrimSpace(key))
	st := game.ShipType(key)
	if _, ok := game.ShipStats[st]; !ok {
		return "", fmt.Errorf("unknown ship: %q", key)
	}
	return st, nil
}

// parseDefenseType validates a user-supplied defense key.
func parseDefenseType(key string) (game.DefenseType, error) {
	key = strings.ToLower(strings.TrimSpace(key))
	dt := game.DefenseType(key)
	if _, ok := game.DefenseStats[dt]; !ok {
		return "", fmt.Errorf("unknown defense: %q", key)
	}
	return dt, nil
}
