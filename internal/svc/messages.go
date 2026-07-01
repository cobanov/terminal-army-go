package svc

import (
	"context"
	"errors"
	"strings"

	"github.com/cobanov/terminal-army-go/internal/store"
)

// MessagesService is the in-game inbox. Messages flow from the scheduler
// (combat notifications, transport receipts, colonization results) and from
// alliance actions. List/Get/Delete are user-scoped so a user can never read
// or remove another player's inbox entries.
//
// List returns the user's inbox, newest first, capped at 200 rows.
func (s *MessagesService) List(ctx context.Context, uid int64) ([]Message, error) {
	rows, err := s.app.Queries.ListMessagesForUser(ctx, uid, 200)
	if err != nil {
		return nil, err
	}
	out := make([]Message, 0, len(rows))
	for i := range rows {
		out = append(out, messageToPublic(&rows[i]))
	}
	return out, nil
}

// Send stores a player-to-player message. The sender is resolved from the
// current session; the recipient is looked up by username.
func (s *MessagesService) Send(ctx context.Context, senderID int64, recipientUsername, body string) (*Message, error) {
	recipientUsername = strings.TrimSpace(recipientUsername)
	body = strings.TrimSpace(body)
	if recipientUsername == "" || body == "" {
		return nil, errors.New("recipient and message body are required")
	}
	if len(body) > 2000 {
		return nil, errors.New("message body must be at most 2000 characters")
	}
	recipient, err := s.app.Queries.GetUserByUsername(ctx, recipientUsername)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if recipient.ID == senderID {
		return nil, errors.New("cannot message yourself")
	}
	sid := senderID
	row, err := s.app.Queries.InsertMessage(ctx, &sid, recipient.ID, "Message", body, "player")
	if err != nil {
		return nil, err
	}
	out := messageToPublic(row)
	return &out, nil
}

// Get returns one inbox message and flips its read flag as a side effect.
// Returns ErrNotFound when the message does not exist or is owned by someone
// else (we do not leak the distinction).
func (s *MessagesService) Get(ctx context.Context, uid, id int64) (*Message, error) {
	row, err := s.app.Queries.GetMessageForUser(ctx, uid, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if !row.Read {
		// Best-effort: do not fail the read if the flag update errors.
		_ = s.app.Queries.MarkMessageRead(ctx, uid, id)
	}
	out := messageToPublic(row)
	return &out, nil
}

// Delete removes a single inbox message. Owner check is enforced in the SQL
// (recipient_id = $userID), so a wrong owner sees an unaffected DELETE that
// still returns nil.
func (s *MessagesService) Delete(ctx context.Context, uid, id int64) error {
	// Confirm the row exists and belongs to the caller before deleting so we
	// can return a meaningful 404. Otherwise a malicious id would silently
	// succeed.
	if _, err := s.app.Queries.GetMessageForUser(ctx, uid, id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	return s.app.Queries.DeleteMessageForUser(ctx, uid, id)
}

// messageToPublic maps the store row to the public JSON view. SenderID is
// dropped for now - the inbox does not surface sender identity.
func messageToPublic(m *store.Message) Message {
	return Message{
		ID:        m.ID,
		UserID:    m.RecipientID,
		Subject:   m.Subject,
		Body:      m.Body,
		Category:  m.Category,
		Read:      m.Read,
		CreatedAt: m.CreatedAt,
	}
}
