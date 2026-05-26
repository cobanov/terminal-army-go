package store

import (
	"context"
	"encoding/json"
	"time"
)

// Report mirrors the reports table.
type Report struct {
	ID             int64
	OwnerID        int64
	ReportType     string
	Title          string
	Body           string
	Payload        map[string]any
	TargetGalaxy   int
	TargetSystem   int
	TargetPosition int
	CreatedAt      time.Time
}

// InsertReport stores a new combat / espionage / mission report.
func (q *Queries) InsertReport(ctx context.Context, r *Report) (*Report, error) {
	payloadJSON, err := json.Marshal(r.Payload)
	if err != nil {
		return nil, err
	}
	row := q.db.QueryRow(ctx, `
		INSERT INTO reports (
			owner_id, report_type, title, body, payload,
			target_galaxy, target_system, target_position
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at
	`, r.OwnerID, r.ReportType, r.Title, r.Body, payloadJSON,
		r.TargetGalaxy, r.TargetSystem, r.TargetPosition)
	if err := row.Scan(&r.ID, &r.CreatedAt); err != nil {
		return nil, err
	}
	return r, nil
}

// ListReportsForUser returns latest reports.
func (q *Queries) ListReportsForUser(ctx context.Context, userID int64, limit int) ([]Report, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := q.db.Query(ctx, `
		SELECT id, owner_id, report_type, title, body, payload,
		       target_galaxy, target_system, target_position, created_at
		FROM reports
		WHERE owner_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Report
	for rows.Next() {
		var r Report
		var payloadJSON []byte
		if err := rows.Scan(
			&r.ID, &r.OwnerID, &r.ReportType, &r.Title, &r.Body, &payloadJSON,
			&r.TargetGalaxy, &r.TargetSystem, &r.TargetPosition, &r.CreatedAt,
		); err != nil {
			return nil, err
		}
		if len(payloadJSON) > 0 {
			_ = json.Unmarshal(payloadJSON, &r.Payload)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetReportForUser returns one report owned by the user.
func (q *Queries) GetReportForUser(ctx context.Context, userID, id int64) (*Report, error) {
	var r Report
	var payloadJSON []byte
	err := q.db.QueryRow(ctx, `
		SELECT id, owner_id, report_type, title, body, payload,
		       target_galaxy, target_system, target_position, created_at
		FROM reports
		WHERE id = $1 AND owner_id = $2
	`, id, userID).Scan(
		&r.ID, &r.OwnerID, &r.ReportType, &r.Title, &r.Body, &payloadJSON,
		&r.TargetGalaxy, &r.TargetSystem, &r.TargetPosition, &r.CreatedAt,
	)
	if err != nil {
		return nil, normalize(err)
	}
	if len(payloadJSON) > 0 {
		_ = json.Unmarshal(payloadJSON, &r.Payload)
	}
	return &r, nil
}
