package store

import (
	"context"
	"time"
)

// DeviceSession is one issued JWT/device pair.
type DeviceSession struct {
	ID        int64
	Code      string
	Token     *string
	UserID    *int64
	CreatedAt time.Time
	ExpiresAt time.Time
}

// CreateDeviceSession inserts a new session row.
func (q *Queries) CreateDeviceSession(ctx context.Context, code, token string, userID int64, expiresAt time.Time) (int64, error) {
	var id int64
	err := q.db.QueryRow(ctx, `
		INSERT INTO device_sessions (code, token, user_id, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, code, token, userID, expiresAt).Scan(&id)
	return id, err
}

// GetDeviceSessionByToken returns a session by raw token, or ErrNotFound.
func (q *Queries) GetDeviceSessionByToken(ctx context.Context, token string) (*DeviceSession, error) {
	var s DeviceSession
	err := q.db.QueryRow(ctx, `
		SELECT id, code, token, user_id, created_at, expires_at
		FROM device_sessions
		WHERE token = $1
	`, token).Scan(&s.ID, &s.Code, &s.Token, &s.UserID, &s.CreatedAt, &s.ExpiresAt)
	if err != nil {
		return nil, normalize(err)
	}
	return &s, nil
}

// DeleteDeviceSessionByToken removes a single session.
func (q *Queries) DeleteDeviceSessionByToken(ctx context.Context, token string) error {
	_, err := q.db.Exec(ctx, `DELETE FROM device_sessions WHERE token = $1`, token)
	return err
}

// DeleteExpiredDeviceSessions purges sessions whose expires_at is in the past.
func (q *Queries) DeleteExpiredDeviceSessions(ctx context.Context, now time.Time) (int64, error) {
	tag, err := q.db.Exec(ctx, `DELETE FROM device_sessions WHERE expires_at < $1`, now)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
