package store

import "context"

// Building represents one building row for a planet.
type Building struct {
	ID           int64
	PlanetID     int64
	BuildingType string
	Level        int
}

// ListBuildingsForPlanet returns every building row of a planet as a map.
// Missing rows mean level 0; the map only contains explicitly-built types.
func (q *Queries) ListBuildingsForPlanet(ctx context.Context, planetID int64) (map[string]int, error) {
	rows, err := q.db.Query(ctx, `
		SELECT building_type, level
		FROM buildings
		WHERE planet_id = $1
	`, planetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]int)
	for rows.Next() {
		var t string
		var l int
		if err := rows.Scan(&t, &l); err != nil {
			return nil, err
		}
		out[t] = l
	}
	return out, rows.Err()
}

// GetBuildingLevel returns one building level (0 if not present).
func (q *Queries) GetBuildingLevel(ctx context.Context, planetID int64, buildingType string) (int, error) {
	var lvl int
	err := q.db.QueryRow(ctx, `
		SELECT level FROM buildings WHERE planet_id = $1 AND building_type = $2
	`, planetID, buildingType).Scan(&lvl)
	if err != nil {
		if err.Error() == "no rows in result set" || normalize(err) == ErrNotFound {
			return 0, nil
		}
		return 0, normalize(err)
	}
	return lvl, nil
}

// SetBuildingLevel upserts a building level.
func (q *Queries) SetBuildingLevel(ctx context.Context, planetID int64, buildingType string, level int) error {
	_, err := q.db.Exec(ctx, `
		INSERT INTO buildings (planet_id, building_type, level)
		VALUES ($1, $2, $3)
		ON CONFLICT (planet_id, building_type) DO UPDATE SET level = EXCLUDED.level
	`, planetID, buildingType, level)
	return err
}

// IncrementBuildingLevel bumps a building level by delta (default 1).
func (q *Queries) IncrementBuildingLevel(ctx context.Context, planetID int64, buildingType string, delta int) error {
	_, err := q.db.Exec(ctx, `
		INSERT INTO buildings (planet_id, building_type, level)
		VALUES ($1, $2, $3)
		ON CONFLICT (planet_id, building_type) DO UPDATE SET level = buildings.level + $3
	`, planetID, buildingType, delta)
	return err
}

// MaxBuildingLevelAcrossUser returns the highest level of a building type
// across all planets owned by a user. Used to compute Research Lab ceiling.
func (q *Queries) MaxBuildingLevelAcrossUser(ctx context.Context, userID int64, buildingType string) (int, error) {
	var lvl int
	err := q.db.QueryRow(ctx, `
		SELECT COALESCE(MAX(b.level), 0)
		FROM buildings b
		JOIN planets p ON p.id = b.planet_id
		WHERE p.owner_user_id = $1 AND b.building_type = $2
	`, userID, buildingType).Scan(&lvl)
	return lvl, err
}
