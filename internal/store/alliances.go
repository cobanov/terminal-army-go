package store

import (
	"context"
	"time"
)

// Alliance mirrors the alliances table.
type Alliance struct {
	ID          int64
	Tag         string
	Name        string
	Description string
	FounderID   int64
	CreatedAt   time.Time
}

// AllianceMember mirrors the alliance_members table.
type AllianceMember struct {
	ID         int64
	AllianceID int64
	UserID     int64
	Role       string
	JoinedAt   time.Time
}

const allianceCols = `id, tag, name, description, founder_id, created_at`

func scanAlliance(row interface {
	Scan(dest ...any) error
}) (*Alliance, error) {
	var a Alliance
	err := row.Scan(&a.ID, &a.Tag, &a.Name, &a.Description, &a.FounderID, &a.CreatedAt)
	if err != nil {
		return nil, normalize(err)
	}
	return &a, nil
}

// CreateAlliance inserts a new alliance and a founder member row.
// Must be called inside a transaction so both rows commit atomically.
func (q *Queries) CreateAlliance(ctx context.Context, founderID int64, tag, name, description string) (*Alliance, error) {
	row := q.db.QueryRow(ctx, `
		INSERT INTO alliances (tag, name, description, founder_id)
		VALUES ($1, $2, $3, $4)
		RETURNING `+allianceCols,
		tag, name, description, founderID)
	a, err := scanAlliance(row)
	if err != nil {
		return nil, err
	}
	if _, err := q.db.Exec(ctx, `
		INSERT INTO alliance_members (alliance_id, user_id, role)
		VALUES ($1, $2, 'leader')
	`, a.ID, founderID); err != nil {
		return nil, err
	}
	return a, nil
}

// ListAlliances returns all alliances ordered by tag.
func (q *Queries) ListAlliances(ctx context.Context) ([]Alliance, error) {
	rows, err := q.db.Query(ctx, `SELECT `+allianceCols+` FROM alliances ORDER BY tag`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Alliance
	for rows.Next() {
		var a Alliance
		if err := rows.Scan(&a.ID, &a.Tag, &a.Name, &a.Description, &a.FounderID, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// GetAlliance returns one alliance by id.
func (q *Queries) GetAlliance(ctx context.Context, id int64) (*Alliance, error) {
	row := q.db.QueryRow(ctx, `SELECT `+allianceCols+` FROM alliances WHERE id = $1`, id)
	return scanAlliance(row)
}

// GetAllianceByTag looks up an alliance by its 6-char tag.
func (q *Queries) GetAllianceByTag(ctx context.Context, tag string) (*Alliance, error) {
	row := q.db.QueryRow(ctx, `SELECT `+allianceCols+` FROM alliances WHERE tag = $1`, tag)
	return scanAlliance(row)
}

// GetUserAlliance returns the alliance row for the user's membership, or
// ErrNotFound if unaffiliated.
func (q *Queries) GetUserAlliance(ctx context.Context, userID int64) (*Alliance, error) {
	row := q.db.QueryRow(ctx, `
		SELECT a.id, a.tag, a.name, a.description, a.founder_id, a.created_at
		FROM alliances a
		JOIN alliance_members m ON m.alliance_id = a.id
		WHERE m.user_id = $1
	`, userID)
	return scanAlliance(row)
}

// CountAllianceMembers returns the member count for an alliance.
func (q *Queries) CountAllianceMembers(ctx context.Context, allianceID int64) (int, error) {
	var n int
	err := q.db.QueryRow(ctx, `SELECT COUNT(*) FROM alliance_members WHERE alliance_id = $1`, allianceID).Scan(&n)
	return n, err
}

// AddAllianceMember inserts a member row (or errors on the unique).
func (q *Queries) AddAllianceMember(ctx context.Context, allianceID, userID int64, role string) error {
	_, err := q.db.Exec(ctx, `
		INSERT INTO alliance_members (alliance_id, user_id, role)
		VALUES ($1, $2, $3)
	`, allianceID, userID, role)
	return err
}

// RemoveAllianceMember drops a member row.
func (q *Queries) RemoveAllianceMember(ctx context.Context, allianceID, userID int64) error {
	_, err := q.db.Exec(ctx, `
		DELETE FROM alliance_members WHERE alliance_id = $1 AND user_id = $2
	`, allianceID, userID)
	return err
}

// ListAllianceMembers returns membership rows.
func (q *Queries) ListAllianceMembers(ctx context.Context, allianceID int64) ([]AllianceMember, error) {
	rows, err := q.db.Query(ctx, `
		SELECT id, alliance_id, user_id, role, joined_at
		FROM alliance_members WHERE alliance_id = $1 ORDER BY joined_at
	`, allianceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AllianceMember
	for rows.Next() {
		var m AllianceMember
		if err := rows.Scan(&m.ID, &m.AllianceID, &m.UserID, &m.Role, &m.JoinedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// AllianceTagsForUsers returns a map of user_id to alliance tag for all
// supplied user ids. Missing users are simply absent from the map.
func (q *Queries) AllianceTagsForUsers(ctx context.Context, userIDs []int64) (map[int64]string, error) {
	out := make(map[int64]string, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}
	rows, err := q.db.Query(ctx, `
		SELECT m.user_id, a.tag
		FROM alliance_members m
		JOIN alliances a ON a.id = m.alliance_id
		WHERE m.user_id = ANY($1)
	`, userIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var uid int64
		var tag string
		if err := rows.Scan(&uid, &tag); err != nil {
			return nil, err
		}
		out[uid] = tag
	}
	return out, rows.Err()
}
