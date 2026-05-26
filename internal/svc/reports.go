package svc

import (
	"context"
	"errors"

	"github.com/cobanov/terminal-army-go/internal/store"
)

// ReportsService surfaces combat / espionage / mission reports that the
// scheduler writes during fleet resolution. Reports are owner-scoped: a user
// can only see and read their own reports. Today the service is read-only;
// writes happen server-side from internal/scheduler/fleet.go.

// List returns the user's recent reports, newest first. The store helper caps
// the result at the supplied limit (200 is the playable history depth).
func (s *ReportsService) List(ctx context.Context, uid int64) ([]Report, error) {
	rows, err := s.app.Queries.ListReportsForUser(ctx, uid, 200)
	if err != nil {
		return nil, err
	}
	out := make([]Report, 0, len(rows))
	for i := range rows {
		out = append(out, reportToPublic(&rows[i]))
	}
	return out, nil
}

// Get fetches one report by id. Owner check is enforced in the SQL
// (owner_id = $userID), so a missing or wrong-owner row both surface as
// ErrNotFound - we never leak the distinction.
func (s *ReportsService) Get(ctx context.Context, uid, id int64) (*Report, error) {
	row, err := s.app.Queries.GetReportForUser(ctx, uid, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	out := reportToPublic(row)
	return &out, nil
}

// reportToPublic maps the store row onto the public JSON view. Title becomes
// Subject and ReportType becomes Kind so the API stays consistent with how
// other entities expose human-facing fields.
func reportToPublic(r *store.Report) Report {
	payload := r.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	return Report{
		ID:        r.ID,
		UserID:    r.OwnerID,
		Kind:      r.ReportType,
		Subject:   r.Title,
		Payload:   payload,
		CreatedAt: r.CreatedAt,
	}
}
