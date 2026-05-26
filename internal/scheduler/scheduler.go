// Package scheduler runs the periodic queue completion sweep. Every tick it
// looks for build_queue rows with finished_at <= now() that haven't been
// applied yet and finalises them in a single transaction with row locking.
package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/cobanov/terminal-army-go/internal/store"
	"github.com/cobanov/terminal-army-go/internal/svc"
	"github.com/jackc/pgx/v5"
)

// batchSize caps how many due items one tick claims. The cap keeps
// transaction duration bounded even after a long backlog. Anything that does
// not fit will be picked up on the next tick.
const batchSize = 50

type Scheduler struct {
	app  *svc.App
	tick time.Duration
}

func New(app *svc.App, tick time.Duration) *Scheduler {
	if tick <= 0 {
		tick = 5 * time.Second
	}
	return &Scheduler{app: app, tick: tick}
}

// Run polls until ctx is cancelled. Each tick processes any due work; errors
// are logged so the sweep keeps making progress on the next tick.
func (s *Scheduler) Run(ctx context.Context) {
	// Sweep once on startup so a fresh process catches anything that finished
	// while it was down.
	s.tickOnce(ctx)

	t := time.NewTicker(s.tick)
	defer t.Stop()
	slog.Info("scheduler started", "tick", s.tick)
	for {
		select {
		case <-ctx.Done():
			slog.Info("scheduler stopped")
			return
		case <-t.C:
			s.tickOnce(ctx)
		}
	}
}

// tickOnce claims any due rows and applies them. Three things run in
// sequence each tick: the build queue sweep (buildings, research, ships,
// defenses), the fleet arrival sweep, and the fleet return sweep. Each
// runs in its own transaction so a failure in one does not stall the
// others. The transactions use SKIP LOCKED so multiple schedulers can
// share work safely.
func (s *Scheduler) tickOnce(ctx context.Context) {
	s.queueSweep(ctx)
	s.arrivalSweep(ctx)
	s.returnSweep(ctx)
}

// queueSweep processes due build_queue rows (buildings, research, ships,
// defenses). Pulled out of tickOnce so each sweep owns its own transaction.
func (s *Scheduler) queueSweep(ctx context.Context) {
	err := store.InTx(ctx, s.app.Pool, func(tx pgx.Tx) error {
		qtx := s.app.Queries.WithTx(tx)
		due, err := qtx.ListDueQueueItems(ctx, time.Now().UTC(), batchSize)
		if err != nil {
			return err
		}
		for i := range due {
			item := &due[i]
			if err := s.applyOne(ctx, qtx, item); err != nil {
				// Log and skip; rolling back the whole tx would also lose
				// progress on every other item in the batch.
				slog.Error("apply queue item failed",
					"id", item.ID, "type", item.QueueType, "key", item.ItemKey, "err", err)
				continue
			}
			if err := qtx.MarkQueueApplied(ctx, item.ID); err != nil {
				slog.Error("mark applied failed", "id", item.ID, "err", err)
				continue
			}
			s.broadcast(item)
		}
		return nil
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("scheduler queue sweep failed", "err", err)
	}
}

// applyOne dispatches an item to its per-type apply function.
func (s *Scheduler) applyOne(ctx context.Context, qtx *store.Queries, item *store.QueueItem) error {
	switch item.QueueType {
	case "building":
		return s.applyBuilding(ctx, qtx, item)
	case "research":
		return s.applyResearch(ctx, qtx, item)
	case "ship":
		return s.applyShip(ctx, qtx, item)
	case "defense":
		return s.applyDefense(ctx, qtx, item)
	default:
		return errInvalidQueueType
	}
}

var errInvalidQueueType = errors.New("unknown queue_type")

// applyBuilding increments the building level and consumes one planet field
// per level. We re-read the planet for the implicit row lock so concurrent
// queue commands cannot interleave with the apply.
func (s *Scheduler) applyBuilding(ctx context.Context, qtx *store.Queries, item *store.QueueItem) error {
	if item.PlanetID == nil {
		return errors.New("building queue item missing planet_id")
	}
	if err := qtx.IncrementBuildingLevel(ctx, *item.PlanetID, item.ItemKey, 1); err != nil {
		return err
	}
	return qtx.IncrementPlanetFields(ctx, *item.PlanetID, 1)
}

// applyResearch bumps the user's tech level. Research is account-wide, so it
// does not touch the planet row at all.
func (s *Scheduler) applyResearch(ctx context.Context, qtx *store.Queries, item *store.QueueItem) error {
	return qtx.IncrementResearchLevel(ctx, item.UserID, item.ItemKey, 1)
}

// applyShip adds the batch count to planet_ships.
func (s *Scheduler) applyShip(ctx context.Context, qtx *store.Queries, item *store.QueueItem) error {
	if item.PlanetID == nil {
		return errors.New("ship queue item missing planet_id")
	}
	delta := item.Count
	if delta <= 0 {
		delta = 1
	}
	return qtx.AddShips(ctx, *item.PlanetID, item.ItemKey, delta)
}

// applyDefense adds the batch count to planet_defenses.
func (s *Scheduler) applyDefense(ctx context.Context, qtx *store.Queries, item *store.QueueItem) error {
	if item.PlanetID == nil {
		return errors.New("defense queue item missing planet_id")
	}
	delta := item.Count
	if delta <= 0 {
		delta = 1
	}
	return qtx.AddDefenses(ctx, *item.PlanetID, item.ItemKey, delta)
}

// broadcast pushes a completion event to the owner via the WebSocket hub.
// Quietly skipped when Events is nil or a no-op sink.
func (s *Scheduler) broadcast(item *store.QueueItem) {
	if s.app.Events == nil {
		return
	}
	payload := map[string]any{
		"queue_id":   item.ID,
		"queue_type": item.QueueType,
		"item_key":   item.ItemKey,
		"count":      item.Count,
		"planet_id":  item.PlanetID,
	}
	s.app.Events.Broadcast(item.UserID, "queue.completed", payload)
}
