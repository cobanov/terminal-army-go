package store

import (
	"context"
	"time"
)

// Universe mirrors the universes table.
type Universe struct {
	ID            int64
	Name          string
	SpeedEconomy  int
	SpeedFleet    int
	SpeedResearch int
	GalaxiesCount int
	SystemsCount  int
	IsActive      bool
	CreatedAt     time.Time
}

const universeSelectCols = `id, name, speed_economy, speed_fleet, speed_research, galaxies_count, systems_count, is_active, created_at`

func scanUniverse(row interface {
	Scan(dest ...any) error
}) (*Universe, error) {
	var u Universe
	err := row.Scan(
		&u.ID, &u.Name, &u.SpeedEconomy, &u.SpeedFleet,
		&u.SpeedResearch, &u.GalaxiesCount, &u.SystemsCount,
		&u.IsActive, &u.CreatedAt,
	)
	if err != nil {
		return nil, normalize(err)
	}
	return &u, nil
}

// CreateUniverse inserts a universe row.
func (q *Queries) CreateUniverse(ctx context.Context, name string, galaxies, systems, eco, fleet, research int) (*Universe, error) {
	row := q.db.QueryRow(ctx, `
		INSERT INTO universes (name, galaxies_count, systems_count, speed_economy, speed_fleet, speed_research)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING `+universeSelectCols,
		name, galaxies, systems, eco, fleet, research)
	return scanUniverse(row)
}

// ListUniverses returns all universes ordered by id.
func (q *Queries) ListUniverses(ctx context.Context) ([]Universe, error) {
	rows, err := q.db.Query(ctx, `SELECT `+universeSelectCols+` FROM universes ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Universe
	for rows.Next() {
		var u Universe
		if err := rows.Scan(
			&u.ID, &u.Name, &u.SpeedEconomy, &u.SpeedFleet,
			&u.SpeedResearch, &u.GalaxiesCount, &u.SystemsCount,
			&u.IsActive, &u.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// GetUniverse returns one universe by id.
func (q *Queries) GetUniverse(ctx context.Context, id int64) (*Universe, error) {
	row := q.db.QueryRow(ctx, `SELECT `+universeSelectCols+` FROM universes WHERE id = $1`, id)
	return scanUniverse(row)
}

// GetUniverseByName looks up by unique name.
func (q *Queries) GetUniverseByName(ctx context.Context, name string) (*Universe, error) {
	row := q.db.QueryRow(ctx, `SELECT `+universeSelectCols+` FROM universes WHERE name = $1`, name)
	return scanUniverse(row)
}

// CountPlayersInUniverse returns how many users have planets in a universe.
func (q *Queries) CountPlayersInUniverse(ctx context.Context, universeID int64) (int, error) {
	var n int
	err := q.db.QueryRow(ctx, `
		SELECT COUNT(DISTINCT owner_user_id) FROM planets WHERE universe_id = $1
	`, universeID).Scan(&n)
	return n, err
}

// CountPlanetsInUniverse returns the total planet count.
func (q *Queries) CountPlanetsInUniverse(ctx context.Context, universeID int64) (int, error) {
	var n int
	err := q.db.QueryRow(ctx, `SELECT COUNT(*) FROM planets WHERE universe_id = $1`, universeID).Scan(&n)
	return n, err
}

// CountUniverses returns the number of universes.
func (q *Queries) CountUniverses(ctx context.Context) (int, error) {
	var n int
	err := q.db.QueryRow(ctx, `SELECT COUNT(*) FROM universes`).Scan(&n)
	return n, err
}
