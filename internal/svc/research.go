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

// ResearchService owns the per-user technology tree and its single-slot queue.
// OGame allows only one research at a time per account, regardless of how many
// research labs the player owns, so the queue check looks at the user, not the
// planet.
type ResearchService struct{ app *App }

// List returns every researched tech for the user. Levels not yet started are
// omitted - the TUI / web client treats missing keys as level 0.
func (s *ResearchService) List(ctx context.Context, userID int64) ([]ResearchLevel, error) {
	rows, err := s.app.Queries.ListResearchesForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]ResearchLevel, 0, len(rows))
	for k, v := range rows {
		out = append(out, ResearchLevel{Tech: k, Level: v})
	}
	return out, nil
}

// Queue enqueues the next level of one tech, paying for it from the given
// planet. The cost comes out of that planet's stockpile but the unlocked tech
// applies to the whole account.
//
// Steps inside one tx:
//  1. SELECT FOR UPDATE the paying planet.
//  2. Lazy-refresh its resources.
//  3. Reject if the user already has an active research item.
//  4. Resolve target level (= current + 1) and verify tech-tree prereqs using
//     the highest Research Lab across the user's planets.
//  5. Compute cost, debit the planet.
//  6. Compute research duration using lab ceiling and universe speed.
//  7. Insert the queue row keyed to this planet so cancel can refund.
func (s *ResearchService) Queue(ctx context.Context, userID, planetID int64, techKey string) (*QueueItem, error) {
	tt, err := parseTechType(techKey)
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

		if err := s.app.Resources.RefreshInTx(ctx, qtx, planet.ID); err != nil {
			return err
		}
		planet, err = qtx.GetPlanetForUpdate(ctx, planetID)
		if err != nil {
			return err
		}

		// One research slot per account is the OGame rule.
		busy, err := qtx.HasActiveResearch(ctx, userID)
		if err != nil {
			return err
		}
		if busy {
			return ErrQueueBusy
		}

		researches, err := qtx.ListResearchesForUser(ctx, userID)
		if err != nil {
			return err
		}

		// Research Lab ceiling is the MAX level across the user's planets.
		// Inter-Galactic Research Network would let multiple labs add together
		// but that's a post-MVP tech, so we stick to the simpler ceiling.
		labLevel, err := qtx.MaxBuildingLevelAcrossUser(ctx, userID, string(game.BuildingResearchLab))
		if err != nil {
			return err
		}

		if ok, missing := game.CheckResearchPrereqs(tt, labLevel, toTechMap(researches)); !ok {
			return fmt.Errorf("%w: %s", ErrPrerequisiteNotMet, strings.Join(missing, ", "))
		}

		baseLevel := researches[string(tt)]
		target := baseLevel + 1

		costMetal, costCrystal, costDeut := game.ResearchLevelCost(tt, target)
		if int(planet.Metal) < costMetal || int(planet.Crystal) < costCrystal || int(planet.Deuterium) < costDeut {
			return ErrInsufficientResources
		}

		universe, err := qtx.GetUniverse(ctx, planet.UniverseID)
		if err != nil {
			return err
		}
		seconds := game.ResearchTimeSeconds(costMetal, costCrystal, labLevel, float64(universe.SpeedResearch))

		now := time.Now().UTC()
		start := now
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
			QueueType:     "research",
			ItemKey:       string(tt),
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

// parseTechType validates a user-supplied tech key against the known set.
func parseTechType(key string) (game.TechType, error) {
	key = strings.ToLower(strings.TrimSpace(key))
	tt := game.TechType(key)
	if _, ok := game.ResearchCosts[tt]; !ok {
		return "", fmt.Errorf("unknown technology: %q", key)
	}
	return tt, nil
}
