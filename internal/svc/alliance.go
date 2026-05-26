package svc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/cobanov/terminal-army-go/internal/store"
	"github.com/jackc/pgx/v5"
)

// AllianceService owns alliances and membership. The domain rules:
//   - tag is 3-6 ASCII chars, uppercase, unique
//   - name is 3-32 chars
//   - one user can be in at most one alliance at a time
//   - the founder cannot leave without first transferring leadership
//
// Create runs inside a transaction so the alliance row and the founder
// member row commit atomically (CreateAlliance does both writes).

// List returns every alliance with its member count attached for the lobby
// view. The dataset is small (hundreds at most) so a per-row count query is
// fine; if we ever cross thousands we will switch to a single GROUP BY.
func (s *AllianceService) List(ctx context.Context) ([]Alliance, error) {
	rows, err := s.app.Queries.ListAlliances(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Alliance, 0, len(rows))
	for i := range rows {
		a := &rows[i]
		count, cerr := s.app.Queries.CountAllianceMembers(ctx, a.ID)
		if cerr != nil {
			return nil, cerr
		}
		out = append(out, allianceToPublic(a, count))
	}
	return out, nil
}

// Create founds a new alliance with uid as leader. Validation rejects bad
// tag/name shapes early; uniqueness is checked once before the insert and
// also enforced at the DB level (unique constraint on alliances.tag).
func (s *AllianceService) Create(ctx context.Context, uid int64, tag, name, desc string) (*Alliance, error) {
	tag = strings.ToUpper(strings.TrimSpace(tag))
	name = strings.TrimSpace(name)
	desc = strings.TrimSpace(desc)
	if err := validateAllianceTag(tag); err != nil {
		return nil, err
	}
	if l := len(name); l < 3 || l > 32 {
		return nil, fmt.Errorf("name must be 3-32 chars")
	}
	if len(desc) > 1024 {
		return nil, fmt.Errorf("description must be at most 1024 chars")
	}

	// Block the second founding attempt for an already-affiliated user. The
	// DB has a unique on alliance_members.user_id too, but a clean error here
	// keeps the surface simple.
	if existing, err := s.app.Queries.GetUserAlliance(ctx, uid); err == nil && existing != nil {
		return nil, fmt.Errorf("user already in alliance %s", existing.Tag)
	} else if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}

	// Tag uniqueness check before insert - lets us return a friendly error
	// instead of a Postgres unique violation.
	if _, err := s.app.Queries.GetAllianceByTag(ctx, tag); err == nil {
		return nil, fmt.Errorf("alliance tag %s is taken", tag)
	} else if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}

	var created *store.Alliance
	if err := store.InTx(ctx, s.app.Pool, func(tx pgx.Tx) error {
		qtx := s.app.Queries.WithTx(tx)
		a, err := qtx.CreateAlliance(ctx, uid, tag, name, desc)
		if err != nil {
			return err
		}
		created = a
		return nil
	}); err != nil {
		return nil, err
	}

	out := allianceToPublic(created, 1)
	return &out, nil
}

// Get fetches one alliance plus its member count. Returns ErrNotFound when
// no row matches; callers map that to HTTP 404.
func (s *AllianceService) Get(ctx context.Context, id int64) (*Alliance, error) {
	a, err := s.app.Queries.GetAlliance(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	count, err := s.app.Queries.CountAllianceMembers(ctx, a.ID)
	if err != nil {
		return nil, err
	}
	out := allianceToPublic(a, count)
	return &out, nil
}

// Join adds uid to the alliance as a regular member. Rejects the request if
// the user is already in any alliance (one-alliance-per-user rule) or if the
// alliance does not exist.
func (s *AllianceService) Join(ctx context.Context, uid, id int64) error {
	if existing, err := s.app.Queries.GetUserAlliance(ctx, uid); err == nil && existing != nil {
		if existing.ID == id {
			return nil // already a member - treat as idempotent success
		}
		return fmt.Errorf("user already in alliance %s", existing.Tag)
	} else if err != nil && !errors.Is(err, store.ErrNotFound) {
		return err
	}

	if _, err := s.app.Queries.GetAlliance(ctx, id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}

	return s.app.Queries.AddAllianceMember(ctx, id, uid, "member")
}

// Leave removes uid from the alliance. The founder cannot leave while other
// members remain; they have to disband or transfer leadership first.
// Returns ErrNotFound if the alliance does not exist or the user is not in it.
func (s *AllianceService) Leave(ctx context.Context, uid, id int64) error {
	a, err := s.app.Queries.GetAlliance(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	// Verify the caller is actually in this alliance, not some other one.
	cur, err := s.app.Queries.GetUserAlliance(ctx, uid)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	if cur.ID != a.ID {
		return ErrNotFound
	}
	if a.FounderID == uid {
		count, err := s.app.Queries.CountAllianceMembers(ctx, a.ID)
		if err != nil {
			return err
		}
		if count > 1 {
			return fmt.Errorf("founder cannot leave while members remain; transfer leadership first")
		}
	}
	return s.app.Queries.RemoveAllianceMember(ctx, a.ID, uid)
}

// validateAllianceTag enforces 3-6 uppercase ASCII letters or digits. The
// constraint mirrors the SQL CHECK on alliances.tag.
func validateAllianceTag(tag string) error {
	if l := len(tag); l < 3 || l > 6 {
		return fmt.Errorf("tag must be 3-6 chars")
	}
	for _, r := range tag {
		if r > unicode.MaxASCII {
			return fmt.Errorf("tag must be ASCII")
		}
		if !(unicode.IsUpper(r) || unicode.IsDigit(r)) {
			return fmt.Errorf("tag must be uppercase letters or digits")
		}
	}
	return nil
}

// allianceToPublic maps the store row onto the public JSON view.
func allianceToPublic(a *store.Alliance, memberCount int) Alliance {
	return Alliance{
		ID:          a.ID,
		Tag:         a.Tag,
		Name:        a.Name,
		Description: a.Description,
		OwnerUserID: a.FounderID,
		MemberCount: memberCount,
		CreatedAt:   a.CreatedAt,
	}
}
