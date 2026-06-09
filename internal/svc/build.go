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

// BuildService owns the planet building queue. Each enqueue debits resources
// inside the same transaction that inserts the row, so concurrent commands on
// the same planet cannot double-spend.
type BuildService struct{ app *App }

// QueueBuilding enqueues a single building level upgrade.
//
// Steps inside one tx:
//  1. SELECT FOR UPDATE the planet (so we serialise concurrent commands).
//  2. Lazy-refresh resources up to "now" so the cost check sees current pools.
//  3. Verify the queue isn't already at the per-planet cap.
//  4. Resolve target level = current + 1, look up cost, debit pools.
//  5. Compute build duration with current robotics + nanite levels.
//  6. Insert the queue row, starting at now or chained to the queue tail.
func (s *BuildService) QueueBuilding(ctx context.Context, userID, planetID int64, buildingKey string) (*QueueItem, error) {
	bt, err := parseBuildingType(buildingKey)
	if err != nil {
		return nil, err
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

		// Make sure the stockpile reflects production up to the moment we
		// check costs.
		if err := s.app.Resources.RefreshInTx(ctx, qtx, planet.ID); err != nil {
			return err
		}
		// Re-read after refresh so we see the freshly-applied resources.
		planet, err = qtx.GetPlanetForUpdate(ctx, planetID)
		if err != nil {
			return err
		}

		buildings, err := qtx.ListBuildingsForPlanet(ctx, planet.ID)
		if err != nil {
			return err
		}
		researches, err := qtx.ListResearchesForUser(ctx, planet.OwnerUserID)
		if err != nil {
			return err
		}

		// Account for already-queued upgrades so chained builds target the
		// right level. Per-planet queue is ordered by finished_at.
		pending, err := qtx.ListActiveQueueForPlanet(ctx, planet.ID)
		if err != nil {
			return err
		}
		pending, err = repairCompletedBuildingQueues(ctx, qtx, buildings, pending)
		if err != nil {
			return err
		}
		if len(pending) >= game.BuildQueueMaxActive {
			return ErrQueueBusy
		}
		baseLevel := buildings[string(bt)]
		for _, q := range pending {
			if q.QueueType == "building" && q.ItemKey == string(bt) {
				if q.TargetLevel > baseLevel {
					baseLevel = q.TargetLevel
				}
			}
		}
		target := baseLevel + 1

		if ok, missing := game.CheckBuildingPrereqs(bt, toBuildingMap(buildings), toTechMap(researches)); !ok {
			return fmt.Errorf("%w: %s", ErrPrerequisiteNotMet, strings.Join(missing, ", "))
		}

		universe, err := qtx.GetUniverse(ctx, planet.UniverseID)
		if err != nil {
			return err
		}

		costMetal, costCrystal, costDeut := game.BuildingLevelCost(bt, target)
		costMetal = applyFirstSolarRescueCost(bt, target, planet, buildings, researches, costMetal, costCrystal, costDeut, float64(universe.SpeedEconomy))
		if int(planet.Metal) < costMetal || int(planet.Crystal) < costCrystal || int(planet.Deuterium) < costDeut {
			return ErrInsufficientResources
		}

		robotics := buildings[string(game.BuildingRoboticsFactory)]
		nanite := buildings[string(game.BuildingNaniteFactory)]
		seconds := game.BuildTimeSeconds(costMetal, costCrystal, robotics, nanite, float64(universe.SpeedEconomy))

		// Chain to queue tail when something is already queued.
		now := time.Now().UTC()
		start := now
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

		// Debit resources.
		newMetal := planet.Metal - float64(costMetal)
		newCrystal := planet.Crystal - float64(costCrystal)
		newDeut := planet.Deuterium - float64(costDeut)
		if err := qtx.UpdatePlanetResources(ctx, planet.ID, newMetal, newCrystal, newDeut, now); err != nil {
			return err
		}

		pid := planet.ID
		row, err := qtx.InsertQueueItem(ctx, &store.QueueItem{
			PlanetID:      &pid,
			UserID:        planet.OwnerUserID,
			QueueType:     "building",
			ItemKey:       string(bt),
			TargetLevel:   target,
			Count:         1,
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

func applyFirstSolarRescueCost(bt game.BuildingType, target int, planet *store.Planet, buildings map[string]int, researches map[string]int, costMetal, costCrystal, costDeut int, speed float64) int {
	if bt != game.BuildingSolarPlant || target != 1 || buildings[string(game.BuildingSolarPlant)] > 0 {
		return costMetal
	}
	if int(planet.Crystal) < costCrystal || int(planet.Deuterium) < costDeut || int(planet.Metal) >= costMetal {
		return costMetal
	}
	report := game.ComputePlanetProduction(
		toBuildingMap(buildings),
		toTechMap(researches),
		planet.TempMin, planet.TempMax,
		game.MetalBonusByPosition[planet.Position],
		game.CrystalBonusByPosition[planet.Position],
		speed,
	)
	if report.EnergyProduced > 0 || report.EnergyConsumed == 0 {
		return costMetal
	}
	if planet.Metal <= 0 {
		return 0
	}
	return int(planet.Metal)
}

// PlanetQueues returns the active (non-cancelled, non-applied) queue items
// for one planet, ordered by finished_at.
func (s *BuildService) PlanetQueues(ctx context.Context, userID, planetID int64) ([]QueueItem, error) {
	planet, err := s.app.Queries.GetPlanet(ctx, planetID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if planet.OwnerUserID != userID {
		return nil, ErrForbidden
	}
	rows, err := s.app.Queries.ListActiveQueueForPlanet(ctx, planetID)
	if err != nil {
		return nil, err
	}
	buildings, err := s.app.Queries.ListBuildingsForPlanet(ctx, planetID)
	if err != nil {
		return nil, err
	}
	rows, err = repairCompletedBuildingQueues(ctx, s.app.Queries, buildings, rows)
	if err != nil {
		return nil, err
	}
	out := make([]QueueItem, 0, len(rows))
	for i := range rows {
		out = append(out, *queueItemToPublic(&rows[i]))
	}
	return out, nil
}

func repairCompletedBuildingQueues(ctx context.Context, qtx *store.Queries, buildings map[string]int, rows []store.QueueItem) ([]store.QueueItem, error) {
	out := rows[:0]
	for _, row := range rows {
		if row.QueueType == "building" && row.TargetLevel > 0 && buildings[row.ItemKey] >= row.TargetLevel {
			if err := qtx.MarkQueueApplied(ctx, row.ID); err != nil {
				return nil, err
			}
			continue
		}
		out = append(out, row)
	}
	return out, nil
}

// Cancel marks a queue item cancelled and refunds resources. Only the owner
// may cancel, and only items that have not yet been applied.
func (s *BuildService) Cancel(ctx context.Context, userID, queueID int64) error {
	return store.InTx(ctx, s.app.Pool, func(tx pgx.Tx) error {
		qtx := s.app.Queries.WithTx(tx)
		item, err := qtx.GetQueueItem(ctx, queueID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return ErrNotFound
			}
			return err
		}
		if item.UserID != userID {
			return ErrForbidden
		}
		if item.Cancelled || item.Applied {
			return errors.New("queue item already resolved")
		}
		if err := qtx.CancelQueueItem(ctx, queueID); err != nil {
			return err
		}
		// Refund 100% of the resource cost. OGame charges a 50% refund on
		// active builds, but the simpler full-refund keeps the MVP friendly.
		if item.PlanetID != nil {
			planet, err := qtx.GetPlanetForUpdate(ctx, *item.PlanetID)
			if err != nil {
				return err
			}
			if err := s.app.Resources.RefreshInTx(ctx, qtx, planet.ID); err != nil {
				return err
			}
			planet, err = qtx.GetPlanetForUpdate(ctx, *item.PlanetID)
			if err != nil {
				return err
			}
			now := time.Now().UTC()
			err = qtx.UpdatePlanetResources(ctx, planet.ID,
				planet.Metal+float64(item.CostMetal),
				planet.Crystal+float64(item.CostCrystal),
				planet.Deuterium+float64(item.CostDeuterium),
				now,
			)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// parseBuildingType validates the user-supplied building key.
func parseBuildingType(key string) (game.BuildingType, error) {
	key = strings.ToLower(strings.TrimSpace(key))
	bt := game.BuildingType(key)
	if _, ok := game.BuildingCosts[bt]; !ok {
		return "", fmt.Errorf("unknown building: %q", key)
	}
	return bt, nil
}

// queueItemToPublic strips internal cost fields the API doesn't need to
// surface and returns the slim public view.
func queueItemToPublic(q *store.QueueItem) *QueueItem {
	return &QueueItem{
		ID:          q.ID,
		PlanetID:    q.PlanetID,
		UserID:      q.UserID,
		QueueType:   q.QueueType,
		ItemKey:     q.ItemKey,
		TargetLevel: q.TargetLevel,
		Count:       q.Count,
		StartedAt:   q.StartedAt,
		FinishedAt:  q.FinishedAt,
	}
}
