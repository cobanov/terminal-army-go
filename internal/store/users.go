package store

import (
	"context"
	"time"
)

// User mirrors the users table. Service layer maps this into svc.User.
type User struct {
	ID                int64
	Username          string
	Email             string
	PasswordHash      string
	Role              string
	CurrentUniverseID *int64
	LastSeenAt        *time.Time
	CreatedAt         time.Time
}

// CreateUser inserts a new user row and returns its id.
func (q *Queries) CreateUser(ctx context.Context, username, email, passwordHash string) (int64, error) {
	var id int64
	err := q.db.QueryRow(ctx, `
		INSERT INTO users (username, email, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id
	`, username, email, passwordHash).Scan(&id)
	return id, err
}

func scanUser(row interface {
	Scan(dest ...any) error
}) (*User, error) {
	var u User
	err := row.Scan(
		&u.ID, &u.Username, &u.Email, &u.PasswordHash,
		&u.Role, &u.CurrentUniverseID, &u.LastSeenAt, &u.CreatedAt,
	)
	if err != nil {
		return nil, normalize(err)
	}
	return &u, nil
}

const userSelectCols = `id, username, email, password_hash, role, current_universe_id, last_seen_at, created_at`

// GetUserByID returns the user record for an id, or ErrNotFound.
func (q *Queries) GetUserByID(ctx context.Context, id int64) (*User, error) {
	row := q.db.QueryRow(ctx, `SELECT `+userSelectCols+` FROM users WHERE id = $1`, id)
	return scanUser(row)
}

// GetUserByUsername looks up by username (case-sensitive).
func (q *Queries) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	row := q.db.QueryRow(ctx, `SELECT `+userSelectCols+` FROM users WHERE username = $1`, username)
	return scanUser(row)
}

// GetUserByEmail looks up by email.
func (q *Queries) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	row := q.db.QueryRow(ctx, `SELECT `+userSelectCols+` FROM users WHERE email = $1`, email)
	return scanUser(row)
}

// UpdateUserLastSeen bumps the last_seen_at timestamp.
func (q *Queries) UpdateUserLastSeen(ctx context.Context, id int64, ts time.Time) error {
	_, err := q.db.Exec(ctx, `UPDATE users SET last_seen_at = $2 WHERE id = $1`, id, ts)
	return err
}

// SetCurrentUniverse pins a user's active universe for the session.
func (q *Queries) SetCurrentUniverse(ctx context.Context, userID, universeID int64) error {
	_, err := q.db.Exec(ctx, `UPDATE users SET current_universe_id = $2 WHERE id = $1`, userID, universeID)
	return err
}

// CountUsers returns the total user count.
func (q *Queries) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := q.db.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

// CountUsersSeenSince returns how many users have touched an authenticated
// route since the given timestamp.
func (q *Queries) CountUsersSeenSince(ctx context.Context, since time.Time) (int, error) {
	var n int
	err := q.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM users WHERE last_seen_at >= $1
	`, since).Scan(&n)
	return n, err
}

// SetUserRole writes the role column for a user. Used by the admin promote /
// demote commands. Role values are validated at the call site (svc/admin
// layer) so this query stays a plain UPDATE.
func (q *Queries) SetUserRole(ctx context.Context, userID int64, role string) error {
	tag, err := q.db.Exec(ctx, `UPDATE users SET role = $2 WHERE id = $1`, userID, role)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListUsers returns up to `limit` users ordered by id ASC, starting from
// `offset`. The set of columns mirrors GetUserByID so the same scan helper
// can be reused.
func (q *Queries) ListUsers(ctx context.Context, limit, offset int) ([]*User, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := q.db.Query(ctx,
		`SELECT `+userSelectCols+` FROM users ORDER BY id ASC LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*User, 0, limit)
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// CountActiveSessions returns the number of non-expired device sessions. Used
// by the admin stats view as a rough proxy for logged-in players.
func (q *Queries) CountActiveSessions(ctx context.Context) (int, error) {
	var n int
	err := q.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM device_sessions WHERE expires_at > NOW()`,
	).Scan(&n)
	return n, err
}

// GetUsernamesByIDs returns a map id to username for the given ids.
func (q *Queries) GetUsernamesByIDs(ctx context.Context, ids []int64) (map[int64]string, error) {
	out := make(map[int64]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := q.db.Query(ctx, `SELECT id, username FROM users WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[id] = name
	}
	return out, rows.Err()
}
