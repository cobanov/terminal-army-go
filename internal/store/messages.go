package store

import (
	"context"
	"time"
)

// Message mirrors the messages table.
type Message struct {
	ID          int64
	SenderID    *int64
	RecipientID int64
	Subject     string
	Body        string
	Category    string
	Read        bool
	CreatedAt   time.Time
}

const messageCols = `id, sender_id, recipient_id, subject, body, category, read, created_at`

func scanMessage(row interface {
	Scan(dest ...any) error
}) (*Message, error) {
	var m Message
	err := row.Scan(
		&m.ID, &m.SenderID, &m.RecipientID, &m.Subject,
		&m.Body, &m.Category, &m.Read, &m.CreatedAt,
	)
	if err != nil {
		return nil, normalize(err)
	}
	return &m, nil
}

// InsertMessage stores a new inbox message.
func (q *Queries) InsertMessage(ctx context.Context, senderID *int64, recipientID int64, subject, body, category string) (*Message, error) {
	row := q.db.QueryRow(ctx, `
		INSERT INTO messages (sender_id, recipient_id, subject, body, category)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING `+messageCols,
		senderID, recipientID, subject, body, category)
	return scanMessage(row)
}

// ListMessagesForUser returns latest messages for a user, newest first.
func (q *Queries) ListMessagesForUser(ctx context.Context, userID int64, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := q.db.Query(ctx, `
		SELECT `+messageCols+`
		FROM messages
		WHERE recipient_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(
			&m.ID, &m.SenderID, &m.RecipientID, &m.Subject,
			&m.Body, &m.Category, &m.Read, &m.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// GetMessageForUser returns one message owned by the user.
func (q *Queries) GetMessageForUser(ctx context.Context, userID, id int64) (*Message, error) {
	row := q.db.QueryRow(ctx, `
		SELECT `+messageCols+` FROM messages WHERE id = $1 AND recipient_id = $2
	`, id, userID)
	return scanMessage(row)
}

// MarkMessageRead flips read=true.
func (q *Queries) MarkMessageRead(ctx context.Context, userID, id int64) error {
	_, err := q.db.Exec(ctx, `
		UPDATE messages SET read = TRUE WHERE id = $1 AND recipient_id = $2
	`, id, userID)
	return err
}

// DeleteMessageForUser removes one message.
func (q *Queries) DeleteMessageForUser(ctx context.Context, userID, id int64) error {
	_, err := q.db.Exec(ctx, `DELETE FROM messages WHERE id = $1 AND recipient_id = $2`, id, userID)
	return err
}

// CountUnreadMessages counts unread inbox items.
func (q *Queries) CountUnreadMessages(ctx context.Context, userID int64) (int, error) {
	var n int
	err := q.db.QueryRow(ctx, `SELECT COUNT(*) FROM messages WHERE recipient_id = $1 AND read = FALSE`, userID).Scan(&n)
	return n, err
}
