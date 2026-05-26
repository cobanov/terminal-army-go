// Package svc - galaxy.go renders one solar system as a 15-slot table.
// In OGame each system has positions 1..15; most are empty. The galaxy view
// is the read-side surface players use to scout neighbours before launching
// spy probes or attacks, so it must combine the planet roster with owner
// usernames, alliance tags, and live presence.
package svc

import (
	"context"
	"errors"

	"github.com/cobanov/terminal-army-go/internal/store"
)

// systemSlots mirrors the OGame layout: every system has positions 1 through
// 15 and the view should always return all 15, padded with empty slots so the
// TUI can render a fixed grid.
const systemSlots = 15

// ViewSystem returns one solar system view for the player's current universe.
// Slots without a planet appear with just their Position set so the caller
// can render a uniform 15-row table.
func (s *GalaxyService) ViewSystem(ctx context.Context, userID int64, galaxy, system int) (*SystemView, error) {
	if galaxy < 1 || system < 1 {
		return nil, errors.New("galaxy and system must be positive")
	}

	// Resolve the requesting user's active universe. Without a universe pin
	// the player has not joined any world yet and there is nothing to render.
	user, err := s.app.Queries.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if user.CurrentUniverseID == nil {
		return nil, ErrForbidden
	}

	universeID := *user.CurrentUniverseID
	universe, err := s.app.Queries.GetUniverse(ctx, universeID)
	if err != nil {
		return nil, err
	}
	if galaxy > universe.GalaxiesCount || system > universe.SystemsCount {
		return nil, errors.New("coordinates outside universe bounds")
	}

	planets, err := s.app.Queries.ListPlanetsInSystem(ctx, universeID, galaxy, system)
	if err != nil {
		return nil, err
	}

	// Lookup owner usernames and alliance tags in a single round trip each.
	ownerIDs := make([]int64, 0, len(planets))
	seen := make(map[int64]bool, len(planets))
	for i := range planets {
		oid := planets[i].OwnerUserID
		if !seen[oid] {
			seen[oid] = true
			ownerIDs = append(ownerIDs, oid)
		}
	}
	names, err := s.app.Queries.GetUsernamesByIDs(ctx, ownerIDs)
	if err != nil {
		return nil, err
	}
	tags, err := s.app.Queries.AllianceTagsForUsers(ctx, ownerIDs)
	if err != nil {
		return nil, err
	}

	// Build a position -> planet index so we can fill 15 slots in order
	// without scanning the planet slice for each position.
	byPos := make(map[int]*store.Planet, len(planets))
	for i := range planets {
		byPos[planets[i].Position] = &planets[i]
	}

	out := &SystemView{
		Galaxy:  galaxy,
		System:  system,
		Planets: make([]SystemPlanetView, 0, systemSlots),
	}
	for pos := 1; pos <= systemSlots; pos++ {
		slot := SystemPlanetView{Position: pos}
		if p, ok := byPos[pos]; ok {
			slot.PlanetName = p.Name
			slot.OwnerName = names[p.OwnerUserID]
			slot.AllianceTag = tags[p.OwnerUserID]
			if s.app.Presence != nil {
				slot.Online = s.app.Presence.Online(p.OwnerUserID)
			}
			// Score is left at 0 in the MVP; computing it requires summing
			// every building and research level for the owner, which the
			// leaderboard service will own once it lands. Keeping the field
			// zeroed (omitempty in the JSON tag) avoids a stale signal.
		}
		out.Planets = append(out.Planets, slot)
	}
	return out, nil
}
