package store

import (
	"context"
	"time"
)

// Fleet mirrors the fleets table.
type Fleet struct {
	ID               int64
	OwnerID          int64
	OriginPlanetID   int64
	Mission          string
	Status           string
	UniverseID       int64
	TargetGalaxy     int
	TargetSystem     int
	TargetPosition   int
	TargetPlanetID   *int64
	SpeedPercent     int
	DepartureAt      time.Time
	ArrivalAt        time.Time
	ReturnAt         *time.Time
	CargoMetal       int
	CargoCrystal     int
	CargoDeuterium   int
	FuelCost         int
	ArrivalProcessed bool
	ReturnProcessed  bool
}

const fleetCols = `
	id, owner_id, origin_planet_id, mission, status, universe_id,
	target_galaxy, target_system, target_position, target_planet_id,
	speed_percent, departure_at, arrival_at, return_at,
	cargo_metal, cargo_crystal, cargo_deuterium, fuel_cost,
	arrival_processed, return_processed`

func scanFleet(row interface {
	Scan(dest ...any) error
}) (*Fleet, error) {
	var f Fleet
	err := row.Scan(
		&f.ID, &f.OwnerID, &f.OriginPlanetID, &f.Mission, &f.Status, &f.UniverseID,
		&f.TargetGalaxy, &f.TargetSystem, &f.TargetPosition, &f.TargetPlanetID,
		&f.SpeedPercent, &f.DepartureAt, &f.ArrivalAt, &f.ReturnAt,
		&f.CargoMetal, &f.CargoCrystal, &f.CargoDeuterium, &f.FuelCost,
		&f.ArrivalProcessed, &f.ReturnProcessed,
	)
	if err != nil {
		return nil, normalize(err)
	}
	return &f, nil
}

// InsertFleet stores a fleet row. Ship composition is added separately via
// InsertFleetShip. Call inside a transaction so both are atomic.
func (q *Queries) InsertFleet(ctx context.Context, f *Fleet) (*Fleet, error) {
	row := q.db.QueryRow(ctx, `
		INSERT INTO fleets (
			owner_id, origin_planet_id, mission, status, universe_id,
			target_galaxy, target_system, target_position, target_planet_id,
			speed_percent, departure_at, arrival_at, return_at,
			cargo_metal, cargo_crystal, cargo_deuterium, fuel_cost
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12, $13,
			$14, $15, $16, $17
		)
		RETURNING `+fleetCols,
		f.OwnerID, f.OriginPlanetID, f.Mission, f.Status, f.UniverseID,
		f.TargetGalaxy, f.TargetSystem, f.TargetPosition, f.TargetPlanetID,
		f.SpeedPercent, f.DepartureAt, f.ArrivalAt, f.ReturnAt,
		f.CargoMetal, f.CargoCrystal, f.CargoDeuterium, f.FuelCost,
	)
	return scanFleet(row)
}

// InsertFleetShip adds a ship row to a fleet.
func (q *Queries) InsertFleetShip(ctx context.Context, fleetID int64, shipType string, count int) error {
	_, err := q.db.Exec(ctx, `
		INSERT INTO fleet_ships (fleet_id, ship_type, count)
		VALUES ($1, $2, $3)
	`, fleetID, shipType, count)
	return err
}

// GetFleet returns a single fleet by id.
func (q *Queries) GetFleet(ctx context.Context, id int64) (*Fleet, error) {
	row := q.db.QueryRow(ctx, `SELECT `+fleetCols+` FROM fleets WHERE id = $1`, id)
	return scanFleet(row)
}

// ListFleetsByOwner returns recent fleets for a user.
func (q *Queries) ListFleetsByOwner(ctx context.Context, userID int64, limit int) ([]Fleet, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := q.db.Query(ctx, `
		SELECT `+fleetCols+`
		FROM fleets
		WHERE owner_id = $1
		ORDER BY departure_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectFleets(rows)
}

// ListFleetsByOriginPlanet returns active fleets that launched from a planet.
func (q *Queries) ListFleetsByOriginPlanet(ctx context.Context, planetID int64) ([]Fleet, error) {
	rows, err := q.db.Query(ctx, `
		SELECT `+fleetCols+`
		FROM fleets
		WHERE origin_planet_id = $1 AND status IN ('outbound', 'holding', 'returning')
		ORDER BY arrival_at ASC
	`, planetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectFleets(rows)
}

// ListActiveFleetsByOwner returns fleets still in motion for a user.
func (q *Queries) ListActiveFleetsByOwner(ctx context.Context, userID int64) ([]Fleet, error) {
	rows, err := q.db.Query(ctx, `
		SELECT `+fleetCols+`
		FROM fleets
		WHERE owner_id = $1 AND status IN ('outbound', 'holding', 'returning')
		ORDER BY arrival_at ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectFleets(rows)
}

// ListDueArrivals returns fleets whose arrival_at has passed and are still
// unprocessed. Locked with SKIP LOCKED so multiple schedulers can share work.
func (q *Queries) ListDueArrivals(ctx context.Context, now time.Time, limit int) ([]Fleet, error) {
	rows, err := q.db.Query(ctx, `
		SELECT `+fleetCols+`
		FROM fleets
		WHERE status = 'outbound'
		  AND arrival_processed = FALSE
		  AND arrival_at <= $1
		ORDER BY arrival_at ASC
		LIMIT $2
		FOR UPDATE SKIP LOCKED
	`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectFleets(rows)
}

// ListDueReturns returns returning fleets whose return_at has passed.
func (q *Queries) ListDueReturns(ctx context.Context, now time.Time, limit int) ([]Fleet, error) {
	rows, err := q.db.Query(ctx, `
		SELECT `+fleetCols+`
		FROM fleets
		WHERE status = 'returning'
		  AND return_processed = FALSE
		  AND return_at IS NOT NULL
		  AND return_at <= $1
		ORDER BY return_at ASC
		LIMIT $2
		FOR UPDATE SKIP LOCKED
	`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectFleets(rows)
}

// SetFleetStatus updates the status (e.g. outbound -> returning, holding).
func (q *Queries) SetFleetStatus(ctx context.Context, id int64, status string) error {
	_, err := q.db.Exec(ctx, `UPDATE fleets SET status = $2 WHERE id = $1`, id, status)
	return err
}

// SetFleetReturn schedules the return leg.
func (q *Queries) SetFleetReturn(ctx context.Context, id int64, returnAt time.Time) error {
	_, err := q.db.Exec(ctx, `UPDATE fleets SET return_at = $2 WHERE id = $1`, id, returnAt)
	return err
}

// MarkArrivalProcessed flips arrival_processed=true and usually transitions
// the fleet to 'returning' or 'holding' depending on mission.
func (q *Queries) MarkArrivalProcessed(ctx context.Context, id int64, newStatus string, returnAt *time.Time) error {
	_, err := q.db.Exec(ctx, `
		UPDATE fleets
		SET arrival_processed = TRUE,
		    status = $2,
		    return_at = COALESCE($3, return_at)
		WHERE id = $1
	`, id, newStatus, returnAt)
	return err
}

// MarkReturnProcessed flips return_processed=true and sets status to 'returned'.
func (q *Queries) MarkReturnProcessed(ctx context.Context, id int64) error {
	_, err := q.db.Exec(ctx, `
		UPDATE fleets
		SET return_processed = TRUE,
		    status = 'returned'
		WHERE id = $1
	`, id)
	return err
}

// UpdateFleetCargo replaces the cargo amounts (used when fleets pick up
// resources on arrival or drop off on return).
func (q *Queries) UpdateFleetCargo(ctx context.Context, id int64, metal, crystal, deuterium int) error {
	_, err := q.db.Exec(ctx, `
		UPDATE fleets
		SET cargo_metal = $2, cargo_crystal = $3, cargo_deuterium = $4
		WHERE id = $1
	`, id, metal, crystal, deuterium)
	return err
}

// ListFleetShips returns ship_type to count for a fleet.
func (q *Queries) ListFleetShips(ctx context.Context, fleetID int64) (map[string]int, error) {
	rows, err := q.db.Query(ctx, `
		SELECT ship_type, count FROM fleet_ships WHERE fleet_id = $1
	`, fleetID)
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

// DeleteFleet removes the fleet (and cascades fleet_ships).
func (q *Queries) DeleteFleet(ctx context.Context, id int64) error {
	_, err := q.db.Exec(ctx, `DELETE FROM fleets WHERE id = $1`, id)
	return err
}

// SetFleetShipCount overwrites the count for one (fleet_id, ship_type) row.
// Used by combat resolution to apply losses to attacker ships in-place.
// Counts are clamped at zero so callers can pass any value safely.
func (q *Queries) SetFleetShipCount(ctx context.Context, fleetID int64, shipType string, count int) error {
	if count < 0 {
		count = 0
	}
	_, err := q.db.Exec(ctx, `
		INSERT INTO fleet_ships (fleet_id, ship_type, count)
		VALUES ($1, $2, $3)
		ON CONFLICT (fleet_id, ship_type) DO UPDATE
		  SET count = EXCLUDED.count
	`, fleetID, shipType, count)
	return err
}

// DeleteFleetShip removes a single ship row from a fleet (used when combat
// destroys every unit of a given type).
func (q *Queries) DeleteFleetShip(ctx context.Context, fleetID int64, shipType string) error {
	_, err := q.db.Exec(ctx, `
		DELETE FROM fleet_ships WHERE fleet_id = $1 AND ship_type = $2
	`, fleetID, shipType)
	return err
}

// collectFleets drains a rows iterator into a Fleet slice.
func collectFleets(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]Fleet, error) {
	var out []Fleet
	for rows.Next() {
		var f Fleet
		if err := rows.Scan(
			&f.ID, &f.OwnerID, &f.OriginPlanetID, &f.Mission, &f.Status, &f.UniverseID,
			&f.TargetGalaxy, &f.TargetSystem, &f.TargetPosition, &f.TargetPlanetID,
			&f.SpeedPercent, &f.DepartureAt, &f.ArrivalAt, &f.ReturnAt,
			&f.CargoMetal, &f.CargoCrystal, &f.CargoDeuterium, &f.FuelCost,
			&f.ArrivalProcessed, &f.ReturnProcessed,
		); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}
