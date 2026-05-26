package store

import (
	"context"
	"time"
)

// QueueItem mirrors a build_queue row.
type QueueItem struct {
	ID            int64
	PlanetID      *int64
	UserID        int64
	QueueType     string
	ItemKey       string
	TargetLevel   int
	Count         int
	CostMetal     int
	CostCrystal   int
	CostDeuterium int
	StartedAt     time.Time
	FinishedAt    time.Time
	Cancelled     bool
	Applied       bool
}

const queueCols = `
	id, planet_id, user_id, queue_type, item_key, target_level, count,
	cost_metal, cost_crystal, cost_deuterium,
	started_at, finished_at, cancelled, applied`

func scanQueueItem(row interface {
	Scan(dest ...any) error
}) (*QueueItem, error) {
	var q QueueItem
	err := row.Scan(
		&q.ID, &q.PlanetID, &q.UserID, &q.QueueType, &q.ItemKey,
		&q.TargetLevel, &q.Count,
		&q.CostMetal, &q.CostCrystal, &q.CostDeuterium,
		&q.StartedAt, &q.FinishedAt, &q.Cancelled, &q.Applied,
	)
	if err != nil {
		return nil, normalize(err)
	}
	return &q, nil
}

// InsertQueueItem appends a new queue row.
func (q *Queries) InsertQueueItem(ctx context.Context, item *QueueItem) (*QueueItem, error) {
	row := q.db.QueryRow(ctx, `
		INSERT INTO build_queue (
			planet_id, user_id, queue_type, item_key, target_level, count,
			cost_metal, cost_crystal, cost_deuterium, started_at, finished_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11
		)
		RETURNING `+queueCols,
		item.PlanetID, item.UserID, item.QueueType, item.ItemKey,
		item.TargetLevel, item.Count,
		item.CostMetal, item.CostCrystal, item.CostDeuterium,
		item.StartedAt, item.FinishedAt,
	)
	return scanQueueItem(row)
}

// ListActiveQueueForPlanet returns non-cancelled, non-applied queue items.
func (q *Queries) ListActiveQueueForPlanet(ctx context.Context, planetID int64) ([]QueueItem, error) {
	rows, err := q.db.Query(ctx, `
		SELECT `+queueCols+`
		FROM build_queue
		WHERE planet_id = $1 AND cancelled = FALSE AND applied = FALSE
		ORDER BY finished_at ASC
	`, planetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []QueueItem
	for rows.Next() {
		var q QueueItem
		if err := rows.Scan(
			&q.ID, &q.PlanetID, &q.UserID, &q.QueueType, &q.ItemKey,
			&q.TargetLevel, &q.Count,
			&q.CostMetal, &q.CostCrystal, &q.CostDeuterium,
			&q.StartedAt, &q.FinishedAt, &q.Cancelled, &q.Applied,
		); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

// ListActiveQueueForUser returns active queue items across every planet
// owned by the user (used for research queue checks).
func (q *Queries) ListActiveQueueForUser(ctx context.Context, userID int64) ([]QueueItem, error) {
	rows, err := q.db.Query(ctx, `
		SELECT `+queueCols+`
		FROM build_queue
		WHERE user_id = $1 AND cancelled = FALSE AND applied = FALSE
		ORDER BY finished_at ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []QueueItem
	for rows.Next() {
		var q QueueItem
		if err := rows.Scan(
			&q.ID, &q.PlanetID, &q.UserID, &q.QueueType, &q.ItemKey,
			&q.TargetLevel, &q.Count,
			&q.CostMetal, &q.CostCrystal, &q.CostDeuterium,
			&q.StartedAt, &q.FinishedAt, &q.Cancelled, &q.Applied,
		); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

// CountActiveQueueForPlanet returns the count of active queue items, used to
// enforce BuildQueueMaxActive.
func (q *Queries) CountActiveQueueForPlanet(ctx context.Context, planetID int64) (int, error) {
	var n int
	err := q.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM build_queue
		WHERE planet_id = $1 AND cancelled = FALSE AND applied = FALSE
	`, planetID).Scan(&n)
	return n, err
}

// HasActiveResearch returns true if the user already has an unfinished
// research queue item. Only one research may run at a time per user.
func (q *Queries) HasActiveResearch(ctx context.Context, userID int64) (bool, error) {
	var n int
	err := q.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM build_queue
		WHERE user_id = $1 AND queue_type = 'research'
		  AND cancelled = FALSE AND applied = FALSE
	`, userID).Scan(&n)
	return n > 0, err
}

// LatestActiveQueueFinish returns the latest finished_at across active items
// for a planet (so a chained build starts after the current tail).
func (q *Queries) LatestActiveQueueFinish(ctx context.Context, planetID int64) (time.Time, error) {
	var ts time.Time
	err := q.db.QueryRow(ctx, `
		SELECT COALESCE(MAX(finished_at), NOW())
		FROM build_queue
		WHERE planet_id = $1 AND cancelled = FALSE AND applied = FALSE
	`, planetID).Scan(&ts)
	return ts, err
}

// ListDueQueueItems returns queue items whose finished_at has passed and
// which are still unapplied / not cancelled. Uses SKIP LOCKED so multiple
// scheduler workers don't trip over each other.
func (q *Queries) ListDueQueueItems(ctx context.Context, now time.Time, limit int) ([]QueueItem, error) {
	rows, err := q.db.Query(ctx, `
		SELECT `+queueCols+`
		FROM build_queue
		WHERE cancelled = FALSE AND applied = FALSE AND finished_at <= $1
		ORDER BY finished_at ASC
		LIMIT $2
		FOR UPDATE SKIP LOCKED
	`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []QueueItem
	for rows.Next() {
		var q QueueItem
		if err := rows.Scan(
			&q.ID, &q.PlanetID, &q.UserID, &q.QueueType, &q.ItemKey,
			&q.TargetLevel, &q.Count,
			&q.CostMetal, &q.CostCrystal, &q.CostDeuterium,
			&q.StartedAt, &q.FinishedAt, &q.Cancelled, &q.Applied,
		); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

// MarkQueueApplied flips applied=true on a row after the effect has been
// committed.
func (q *Queries) MarkQueueApplied(ctx context.Context, id int64) error {
	_, err := q.db.Exec(ctx, `UPDATE build_queue SET applied = TRUE WHERE id = $1`, id)
	return err
}

// CancelQueueItem marks a queue item as cancelled.
func (q *Queries) CancelQueueItem(ctx context.Context, id int64) error {
	_, err := q.db.Exec(ctx, `UPDATE build_queue SET cancelled = TRUE WHERE id = $1`, id)
	return err
}

// GetQueueItem returns a single queue row by id.
func (q *Queries) GetQueueItem(ctx context.Context, id int64) (*QueueItem, error) {
	row := q.db.QueryRow(ctx, `SELECT `+queueCols+` FROM build_queue WHERE id = $1`, id)
	return scanQueueItem(row)
}
