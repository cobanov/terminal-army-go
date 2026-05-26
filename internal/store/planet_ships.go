package store

import "context"

// ListShipsForPlanet returns ship counts for a planet as ship_type to count.
func (q *Queries) ListShipsForPlanet(ctx context.Context, planetID int64) (map[string]int, error) {
	rows, err := q.db.Query(ctx, `
		SELECT ship_type, count FROM planet_ships WHERE planet_id = $1
	`, planetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]int)
	for rows.Next() {
		var t string
		var c int
		if err := rows.Scan(&t, &c); err != nil {
			return nil, err
		}
		out[t] = c
	}
	return out, rows.Err()
}

// GetShipCount returns the count of a ship type (0 if missing).
func (q *Queries) GetShipCount(ctx context.Context, planetID int64, shipType string) (int, error) {
	var n int
	err := q.db.QueryRow(ctx, `
		SELECT count FROM planet_ships WHERE planet_id = $1 AND ship_type = $2
	`, planetID, shipType).Scan(&n)
	if err != nil {
		if normalize(err) == ErrNotFound {
			return 0, nil
		}
		return 0, normalize(err)
	}
	return n, nil
}

// AddShips bumps the ship count by delta (positive or negative).
// Will create the row at delta if absent.
func (q *Queries) AddShips(ctx context.Context, planetID int64, shipType string, delta int) error {
	_, err := q.db.Exec(ctx, `
		INSERT INTO planet_ships (planet_id, ship_type, count)
		VALUES ($1, $2, $3)
		ON CONFLICT (planet_id, ship_type) DO UPDATE
		  SET count = GREATEST(0, planet_ships.count + $3)
	`, planetID, shipType, delta)
	return err
}

// ListDefensesForPlanet returns defense counts for a planet.
func (q *Queries) ListDefensesForPlanet(ctx context.Context, planetID int64) (map[string]int, error) {
	rows, err := q.db.Query(ctx, `
		SELECT defense_type, count FROM planet_defenses WHERE planet_id = $1
	`, planetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]int)
	for rows.Next() {
		var t string
		var c int
		if err := rows.Scan(&t, &c); err != nil {
			return nil, err
		}
		out[t] = c
	}
	return out, rows.Err()
}

// AddDefenses bumps the defense count by delta.
func (q *Queries) AddDefenses(ctx context.Context, planetID int64, defenseType string, delta int) error {
	_, err := q.db.Exec(ctx, `
		INSERT INTO planet_defenses (planet_id, defense_type, count)
		VALUES ($1, $2, $3)
		ON CONFLICT (planet_id, defense_type) DO UPDATE
		  SET count = GREATEST(0, planet_defenses.count + $3)
	`, planetID, defenseType, delta)
	return err
}
