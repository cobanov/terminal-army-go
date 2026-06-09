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

// CreatePendingDeviceSession inserts a short-lived browser auth code. The CLI
// polls this row until the web login/signup flow binds a token to it.
func (q *Queries) CreatePendingDeviceSession(ctx context.Context, code string, expiresAt time.Time) (int64, error) {
	var id int64
	err := q.db.QueryRow(ctx, `
		INSERT INTO device_sessions (code, token, user_id, expires_at)
		VALUES ($1, NULL, NULL, $2)
		RETURNING id
	`, code, expiresAt).Scan(&id)
	return id, err
}

// GetDeviceSessionByCode returns a pending or bound device auth row.
func (q *Queries) GetDeviceSessionByCode(ctx context.Context, code string) (*DeviceSession, error) {
	var s DeviceSession
	err := q.db.QueryRow(ctx, `
		SELECT id, code, token, user_id, created_at, expires_at
		FROM device_sessions
		WHERE code = $1
	`, code).Scan(&s.ID, &s.Code, &s.Token, &s.UserID, &s.CreatedAt, &s.ExpiresAt)
	if err != nil {
		return nil, normalize(err)
	}
	return &s, nil
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

// BindDeviceSessionToken attaches an issued JWT to a pending browser auth code.
func (q *Queries) BindDeviceSessionToken(ctx context.Context, code, token string, userID int64, expiresAt time.Time) error {
	tag, err := q.db.Exec(ctx, `
		UPDATE device_sessions
		SET token = $2, user_id = $3, expires_at = $4
		WHERE code = $1
		  AND token IS NULL
		  AND expires_at > NOW()
	`, code, token, userID, expiresAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteDeviceSessionByCode removes a pending device auth row.
func (q *Queries) DeleteDeviceSessionByCode(ctx context.Context, code string) error {
	_, err := q.db.Exec(ctx, `DELETE FROM device_sessions WHERE code = $1`, code)
	return err
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
