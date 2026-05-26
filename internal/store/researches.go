package store

import "context"

// ListResearchesForUser returns every tech level for a user as a map.
func (q *Queries) ListResearchesForUser(ctx context.Context, userID int64) (map[string]int, error) {
	rows, err := q.db.Query(ctx, `SELECT tech_type, level FROM researches WHERE user_id = $1`, userID)
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

// GetResearchLevel returns one tech level (0 if missing).
func (q *Queries) GetResearchLevel(ctx context.Context, userID int64, techType string) (int, error) {
	var lvl int
	err := q.db.QueryRow(ctx, `
		SELECT level FROM researches WHERE user_id = $1 AND tech_type = $2
	`, userID, techType).Scan(&lvl)
	if err != nil {
		if normalize(err) == ErrNotFound {
			return 0, nil
		}
		return 0, normalize(err)
	}
	return lvl, nil
}

// SetResearchLevel upserts a tech level.
func (q *Queries) SetResearchLevel(ctx context.Context, userID int64, techType string, level int) error {
	_, err := q.db.Exec(ctx, `
		INSERT INTO researches (user_id, tech_type, level)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, tech_type) DO UPDATE SET level = EXCLUDED.level
	`, userID, techType, level)
	return err
}

// IncrementResearchLevel bumps a tech by delta (creates row at delta if absent).
func (q *Queries) IncrementResearchLevel(ctx context.Context, userID int64, techType string, delta int) error {
	_, err := q.db.Exec(ctx, `
		INSERT INTO researches (user_id, tech_type, level)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, tech_type) DO UPDATE SET level = researches.level + $3
	`, userID, techType, delta)
	return err
}
