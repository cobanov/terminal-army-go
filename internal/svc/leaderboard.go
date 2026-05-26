package svc

import "context"

// LeaderboardService computes the global player ranking. Scoring is a cheap
// MVP aggregate over current building/research/ship state; see
// store.TopPlayersByScore for the formula. A real OGame "points = resources
// spent / 1000" tally needs a separate spend ledger which is out of scope
// for the v1 backend.

// Top returns the top N entries with their alliance tag (if any) and online
// status from the in-memory presence tracker.
func (s *LeaderboardService) Top(ctx context.Context, n int) ([]LeaderboardEntry, error) {
	if n <= 0 {
		n = 100
	}
	rows, err := s.app.Queries.TopPlayersByScore(ctx, n)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []LeaderboardEntry{}, nil
	}

	ids := make([]int64, 0, len(rows))
	for i := range rows {
		ids = append(ids, rows[i].UserID)
	}
	tags, err := s.app.Queries.AllianceTagsForUsers(ctx, ids)
	if err != nil {
		return nil, err
	}

	out := make([]LeaderboardEntry, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		out = append(out, LeaderboardEntry{
			Rank:     i + 1,
			UserID:   r.UserID,
			Username: r.Username,
			Score:    r.Score,
			Alliance: tags[r.UserID],
		})
	}
	return out, nil
}
