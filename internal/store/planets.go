package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Planet mirrors the planets table.
type Planet struct {
	ID                     int64
	Code                   string
	OwnerUserID            int64
	UniverseID             int64
	Galaxy                 int
	System                 int
	Position               int
	Name                   string
	FieldsUsed             int
	FieldsTotal            int
	TempMin                int
	TempMax                int
	Metal                  float64
	Crystal                float64
	Deuterium              float64
	ResourcesLastUpdatedAt time.Time
	CreatedAt              time.Time
}

const planetCols = `
	id, code, owner_user_id, universe_id, galaxy, system, position,
	name, fields_used, fields_total, temp_min, temp_max,
	resources_metal, resources_crystal, resources_deuterium,
	resources_last_updated_at, created_at`

func scanPlanet(row interface {
	Scan(dest ...any) error
}) (*Planet, error) {
	var p Planet
	err := row.Scan(
		&p.ID, &p.Code, &p.OwnerUserID, &p.UniverseID,
		&p.Galaxy, &p.System, &p.Position, &p.Name,
		&p.FieldsUsed, &p.FieldsTotal, &p.TempMin, &p.TempMax,
		&p.Metal, &p.Crystal, &p.Deuterium,
		&p.ResourcesLastUpdatedAt, &p.CreatedAt,
	)
	if err != nil {
		return nil, normalize(err)
	}
	return &p, nil
}

// CreatePlanet inserts a new planet row.
func (q *Queries) CreatePlanet(ctx context.Context, p *Planet) (*Planet, error) {
	row := q.db.QueryRow(ctx, `
		INSERT INTO planets (
			code, owner_user_id, universe_id, galaxy, system, position,
			name, fields_used, fields_total, temp_min, temp_max,
			resources_metal, resources_crystal, resources_deuterium,
			resources_last_updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11,
			$12, $13, $14, $15
		)
		RETURNING `+planetCols,
		p.Code, p.OwnerUserID, p.UniverseID, p.Galaxy, p.System, p.Position,
		p.Name, p.FieldsUsed, p.FieldsTotal, p.TempMin, p.TempMax,
		p.Metal, p.Crystal, p.Deuterium,
		p.ResourcesLastUpdatedAt,
	)
	return scanPlanet(row)
}

// GetPlanet returns a planet by id.
func (q *Queries) GetPlanet(ctx context.Context, id int64) (*Planet, error) {
	row := q.db.QueryRow(ctx, `SELECT `+planetCols+` FROM planets WHERE id = $1`, id)
	return scanPlanet(row)
}

// GetPlanetByCode returns a planet by its short code.
func (q *Queries) GetPlanetByCode(ctx context.Context, code string) (*Planet, error) {
	row := q.db.QueryRow(ctx, `SELECT `+planetCols+` FROM planets WHERE code = $1`, code)
	return scanPlanet(row)
}

// GetPlanetForUpdate locks a planet row for the duration of the transaction
// so two concurrent build commands can't race on the same resources. Must be
// called inside a transaction.
func (q *Queries) GetPlanetForUpdate(ctx context.Context, id int64) (*Planet, error) {
	row := q.db.QueryRow(ctx, `SELECT `+planetCols+` FROM planets WHERE id = $1 FOR UPDATE`, id)
	return scanPlanet(row)
}

// GetPlanetByCoord looks up a planet by universe + galaxy/system/position.
func (q *Queries) GetPlanetByCoord(ctx context.Context, universeID int64, g, s, pos int) (*Planet, error) {
	row := q.db.QueryRow(ctx, `
		SELECT `+planetCols+`
		FROM planets
		WHERE universe_id = $1 AND galaxy = $2 AND system = $3 AND position = $4
	`, universeID, g, s, pos)
	return scanPlanet(row)
}

// ListPlanetsByOwner returns every planet owned by a user, ordered by id.
func (q *Queries) ListPlanetsByOwner(ctx context.Context, userID int64) ([]Planet, error) {
	rows, err := q.db.Query(ctx, `SELECT `+planetCols+` FROM planets WHERE owner_user_id = $1 ORDER BY id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Planet
	for rows.Next() {
		var p Planet
		if err := rows.Scan(
			&p.ID, &p.Code, &p.OwnerUserID, &p.UniverseID,
			&p.Galaxy, &p.System, &p.Position, &p.Name,
			&p.FieldsUsed, &p.FieldsTotal, &p.TempMin, &p.TempMax,
			&p.Metal, &p.Crystal, &p.Deuterium,
			&p.ResourcesLastUpdatedAt, &p.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ListPlanetsInSystem returns every planet in a single solar system.
func (q *Queries) ListPlanetsInSystem(ctx context.Context, universeID int64, g, s int) ([]Planet, error) {
	rows, err := q.db.Query(ctx, `
		SELECT `+planetCols+`
		FROM planets
		WHERE universe_id = $1 AND galaxy = $2 AND system = $3
		ORDER BY position
	`, universeID, g, s)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Planet
	for rows.Next() {
		var p Planet
		if err := rows.Scan(
			&p.ID, &p.Code, &p.OwnerUserID, &p.UniverseID,
			&p.Galaxy, &p.System, &p.Position, &p.Name,
			&p.FieldsUsed, &p.FieldsTotal, &p.TempMin, &p.TempMax,
			&p.Metal, &p.Crystal, &p.Deuterium,
			&p.ResourcesLastUpdatedAt, &p.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// UpdatePlanetResources writes new resource amounts and bumps the timestamp.
func (q *Queries) UpdatePlanetResources(ctx context.Context, id int64, metal, crystal, deuterium float64, ts time.Time) error {
	_, err := q.db.Exec(ctx, `
		UPDATE planets
		SET resources_metal = $2,
		    resources_crystal = $3,
		    resources_deuterium = $4,
		    resources_last_updated_at = $5
		WHERE id = $1
	`, id, metal, crystal, deuterium, ts)
	return err
}

// SetPlanetName renames a planet.
func (q *Queries) SetPlanetName(ctx context.Context, id int64, name string) error {
	_, err := q.db.Exec(ctx, `UPDATE planets SET name = $2 WHERE id = $1`, id, name)
	return err
}

// IncrementPlanetFields bumps the used-fields counter (e.g. after building).
func (q *Queries) IncrementPlanetFields(ctx context.Context, id int64, delta int) error {
	_, err := q.db.Exec(ctx, `UPDATE planets SET fields_used = fields_used + $2 WHERE id = $1`, id, delta)
	return err
}

// ErrSlotTaken is returned when CreatePlanet hits the (universe, galaxy, system, position) unique.
var ErrSlotTaken = errors.New("planet slot already occupied")

// IsSlotTaken inspects pgx errors for the planet uniqueness violation.
func IsSlotTaken(err error) bool {
	return errors.Is(err, ErrSlotTaken)
}

// CountPlanetsByUser returns how many planets a user owns.
func (q *Queries) CountPlanetsByUser(ctx context.Context, userID int64) (int, error) {
	var n int
	err := q.db.QueryRow(ctx, `SELECT COUNT(*) FROM planets WHERE owner_user_id = $1`, userID).Scan(&n)
	return n, err
}

// TotalPlanets returns global planet count.
func (q *Queries) TotalPlanets(ctx context.Context) (int, error) {
	var n int
	err := q.db.QueryRow(ctx, `SELECT COUNT(*) FROM planets`).Scan(&n)
	return n, err
}

// OccupiedPositionsInSystem returns the set of taken slot positions in a system.
func (q *Queries) OccupiedPositionsInSystem(ctx context.Context, universeID int64, g, s int) (map[int]bool, error) {
	rows, err := q.db.Query(ctx, `
		SELECT position FROM planets
		WHERE universe_id = $1 AND galaxy = $2 AND system = $3
	`, universeID, g, s)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[int]bool)
	for rows.Next() {
		var pos int
		if err := rows.Scan(&pos); err != nil {
			return nil, err
		}
		out[pos] = true
	}
	return out, rows.Err()
}

// asPgxRow is a tiny adapter so pgx.Row can be passed where Scan is expected.
var _ pgx.Row = pgx.Row(nil)
