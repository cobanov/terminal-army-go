package svc

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	mathrand "math/rand"
	"strings"
	"time"

	"github.com/cobanov/terminal-army-go/internal/game"
	"github.com/cobanov/terminal-army-go/internal/store"
	"github.com/jackc/pgx/v5"
)

// UniverseService manages the list of game universes and player joins.
type UniverseService struct{ app *App }

// ErrUniverseFull is returned by JoinUniverse when no slot can be allocated
// after a reasonable number of attempts. In practice this only fires when
// every system in every galaxy is saturated.
var ErrUniverseFull = errors.New("universe is full")

// ErrAlreadyInUniverse is returned when the user already has at least one
// planet in the requested universe.
var ErrAlreadyInUniverse = errors.New("user already has a planet in this universe")

// List returns every universe with live player and planet counts.
func (s *UniverseService) List(ctx context.Context) ([]Universe, error) {
	rows, err := s.app.Queries.ListUniverses(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Universe, 0, len(rows))
	for _, u := range rows {
		players, _ := s.app.Queries.CountPlayersInUniverse(ctx, u.ID)
		out = append(out, Universe{
			ID:            u.ID,
			Name:          u.Name,
			SpeedEconomy:  u.SpeedEconomy,
			SpeedFleet:    u.SpeedFleet,
			SpeedResearch: u.SpeedResearch,
			GalaxiesCount: u.GalaxiesCount,
			SystemsCount:  u.SystemsCount,
			PlayerCount:   players,
			CreatedAt:     u.CreatedAt,
		})
	}
	return out, nil
}

// Get fetches a single universe by id.
func (s *UniverseService) Get(ctx context.Context, id int64) (*Universe, error) {
	u, err := s.app.Queries.GetUniverse(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	players, _ := s.app.Queries.CountPlayersInUniverse(ctx, u.ID)
	return &Universe{
		ID:            u.ID,
		Name:          u.Name,
		SpeedEconomy:  u.SpeedEconomy,
		SpeedFleet:    u.SpeedFleet,
		SpeedResearch: u.SpeedResearch,
		GalaxiesCount: u.GalaxiesCount,
		SystemsCount:  u.SystemsCount,
		PlayerCount:   players,
		CreatedAt:     u.CreatedAt,
	}, nil
}

// EnsureDefaultUniverse creates the default universe on first boot. It is
// idempotent: if a universe with the configured name already exists it is
// returned unchanged. Called by serve/migrate command at startup.
func (s *UniverseService) EnsureDefaultUniverse(ctx context.Context) (*store.Universe, error) {
	cfg := s.app.Cfg
	existing, err := s.app.Queries.GetUniverseByName(ctx, cfg.DefaultUniverseName)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}
	return s.app.Queries.CreateUniverse(
		ctx,
		cfg.DefaultUniverseName,
		cfg.DefaultUniverseGalaxies,
		cfg.DefaultUniverseSystems,
		cfg.DefaultUniverseSpeedEconomy,
		cfg.DefaultUniverseSpeedFleet,
		cfg.DefaultUniverseSpeedResearch,
	)
}

// JoinUniverse colonises a random free slot in the given universe for the
// user and returns the freshly created home planet. Pinned positions 4..12
// (the temperate band) to keep new players away from the harsh outer slots.
// Idempotent in the sense that a user with an existing planet in the universe
// gets ErrAlreadyInUniverse rather than a duplicate.
func (s *UniverseService) JoinUniverse(ctx context.Context, userID, universeID int64) (*Planet, error) {
	universe, err := s.app.Queries.GetUniverse(ctx, universeID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	// Block double joins. We check first so the common path skips the rng
	// loop, but the DB unique constraint is the real guard.
	existing, err := s.app.Queries.ListPlanetsByOwner(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, p := range existing {
		if p.UniverseID == universeID {
			return nil, ErrAlreadyInUniverse
		}
	}

	rng := mathrand.New(mathrand.NewSource(time.Now().UnixNano() ^ userID))

	const homePosLo, homePosHi = 4, 12
	const maxAttempts = 60

	for attempt := 0; attempt < maxAttempts; attempt++ {
		g := rng.Intn(universe.GalaxiesCount) + 1
		sys := rng.Intn(universe.SystemsCount) + 1
		pos := rng.Intn(homePosHi-homePosLo+1) + homePosLo

		attrs, err := game.GeneratePlanetAttributes(pos, rng)
		if err != nil {
			continue
		}
		code, err := newPlanetCode()
		if err != nil {
			return nil, err
		}

		now := time.Now().UTC()
		row := &store.Planet{
			Code:                   code,
			OwnerUserID:            userID,
			UniverseID:             universeID,
			Galaxy:                 g,
			System:                 sys,
			Position:               pos,
			Name:                   "Homeworld",
			FieldsUsed:             game.StartingFieldsUsed,
			FieldsTotal:            attrs.FieldsTotal,
			TempMin:                attrs.TempMin,
			TempMax:                attrs.TempMax,
			Metal:                  float64(game.StartingMetal),
			Crystal:                float64(game.StartingCrystal),
			Deuterium:              float64(game.StartingDeuterium),
			ResourcesLastUpdatedAt: now,
		}

		var created *store.Planet
		err = store.InTx(ctx, s.app.Pool, func(tx pgx.Tx) error {
			qtx := s.app.Queries.WithTx(tx)
			p, err := qtx.CreatePlanet(ctx, row)
			if err != nil {
				return err
			}
			if err := qtx.SetCurrentUniverse(ctx, userID, universeID); err != nil {
				return err
			}
			created = p
			return nil
		})
		if err != nil {
			// Slot race or code collision: pick another slot and retry.
			if isUniqueViolation(err) {
				continue
			}
			return nil, err
		}

		return s.app.Planet.toPublic(ctx, created)
	}

	return nil, ErrUniverseFull
}

// newPlanetCode returns an 8-char base32-ish code for friendly URLs. We avoid
// 0/O/1/I/L to keep TUI-readable codes.
func newPlanetCode() (string, error) {
	const alphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"
	buf := make([]byte, 8)
	rb := make([]byte, 8)
	if _, err := rand.Read(rb); err != nil {
		return "", err
	}
	for i, b := range rb {
		buf[i] = alphabet[int(b)%len(alphabet)]
	}
	return string(buf), nil
}

// normalizePlanetCode upper-cases user input and strips dashes/spaces.
func normalizePlanetCode(in string) string {
	in = strings.ToUpper(strings.TrimSpace(in))
	in = strings.ReplaceAll(in, "-", "")
	in = strings.ReplaceAll(in, " ", "")
	return in
}

var _ = fmt.Sprintf // keep import stable for future error wrapping
