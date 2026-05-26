package store

import "context"

// LeaderboardRow is one row in the global player ranking. Score is a simple
// MVP-grade aggregate: sum(building level) + sum(research level) + ceil(
// sum(ship count) / 10). Defenses are counted the same way as ships. The
// real OGame scoring is "resources spent / 1000" but the spent total is
// expensive to derive after the fact, so we approximate with what is cheap
// to read (current levels and unit counts).
type LeaderboardRow struct {
	UserID   int64
	Username string
	Score    int64
}

// TopPlayersByScore returns the top N players by aggregate score.
//
// Implementation note: each subquery scopes to the user. UNION ALL into a
// single CTE lets one ORDER BY + LIMIT do the ranking. For the player counts
// we expect (hundreds, low thousands) this stays comfortably under a
// millisecond.
func (q *Queries) TopPlayersByScore(ctx context.Context, limit int) ([]LeaderboardRow, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := q.db.Query(ctx, `
		WITH building_scores AS (
			SELECT p.owner_user_id AS user_id, COALESCE(SUM(b.level), 0) AS score
			FROM planets p
			LEFT JOIN buildings b ON b.planet_id = p.id
			GROUP BY p.owner_user_id
		),
		research_scores AS (
			SELECT user_id, COALESCE(SUM(level), 0) AS score
			FROM researches
			GROUP BY user_id
		),
		ship_scores AS (
			SELECT p.owner_user_id AS user_id,
			       COALESCE(CEIL(SUM(s.count)::numeric / 10), 0) AS score
			FROM planets p
			LEFT JOIN planet_ships s ON s.planet_id = p.id
			GROUP BY p.owner_user_id
		),
		totals AS (
			SELECT u.id AS user_id, u.username,
			       COALESCE(bs.score, 0) + COALESCE(rs.score, 0) + COALESCE(ss.score, 0) AS score
			FROM users u
			LEFT JOIN building_scores bs ON bs.user_id = u.id
			LEFT JOIN research_scores rs ON rs.user_id = u.id
			LEFT JOIN ship_scores ss ON ss.user_id = u.id
		)
		SELECT user_id, username, score
		FROM totals
		ORDER BY score DESC, user_id ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LeaderboardRow
	for rows.Next() {
		var r LeaderboardRow
		if err := rows.Scan(&r.UserID, &r.Username, &r.Score); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// CountFleetsInFlight returns the number of fleets that are still moving
// (outbound or returning). Holding fleets are counted too because they are
// not yet returned to base. Used by /stats.
func (q *Queries) CountFleetsInFlight(ctx context.Context) (int, error) {
	var n int
	err := q.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM fleets WHERE status IN ('outbound', 'returning', 'holding')
	`).Scan(&n)
	return n, err
}

// CountAllPlanets returns the total planet count across every universe.
func (q *Queries) CountAllPlanets(ctx context.Context) (int, error) {
	var n int
	err := q.db.QueryRow(ctx, `SELECT COUNT(*) FROM planets`).Scan(&n)
	return n, err
}
