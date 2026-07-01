// Package admin holds the CLI-callable admin operations. Each function opens
// its own pgx pool and exits when done, which keeps `tarmy admin ...` usable
// from a one-shot process without booting the HTTP server.
//
// The admin commands are intentionally tiny: they call into store.Queries and
// svc directly, print human-readable output, and avoid any persistent state
// beyond the database. Anything fancier (audit log, RBAC checks) belongs in
// the web admin surface, not here.
package admin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/cobanov/terminal-army-go/internal/auth"
	"github.com/cobanov/terminal-army-go/internal/config"
	"github.com/cobanov/terminal-army-go/internal/store"
	"github.com/cobanov/terminal-army-go/internal/svc"
)

// RoleAdmin is the canonical admin role string. RolePlayer is the default
// role assigned to fresh signups.
const (
	RoleAdmin  = "admin"
	RolePlayer = "player"
)

// openApp opens a pool, builds an App, and returns a closer the caller must
// defer. Each admin command opens its own short-lived pool because admin
// commands run from one-shot processes that have no scheduler or hub.
func openApp(ctx context.Context) (*svc.App, func(), error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, err
	}
	pool, err := store.OpenPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, nil, err
	}
	queries := store.New(pool)
	tokens := auth.NewSigner(cfg.JWTSecret, cfg.JWTTTL)
	app := svc.NewApp(cfg, pool, queries, tokens)
	return app, func() { pool.Close() }, nil
}

// SeedDefaultUniverse creates the universe described in TARMY_DEFAULT_UNIVERSE_*
// environment variables. Idempotent: if a universe with that name already
// exists, the function reports its id and exits without touching it.
func SeedDefaultUniverse(ctx context.Context) error {
	app, closer, err := openApp(ctx)
	if err != nil {
		return err
	}
	defer closer()

	u, err := app.Universe.EnsureDefaultUniverse(ctx)
	if err != nil {
		return fmt.Errorf("seed default universe: %w", err)
	}
	fmt.Printf("ok: universe %q (id=%d) ready: %dx%d galaxies, speed eco=%d fleet=%d research=%d\n",
		u.Name, u.ID, u.GalaxiesCount, u.SystemsCount,
		u.SpeedEconomy, u.SpeedFleet, u.SpeedResearch)
	return nil
}

// PromoteUser grants the admin role to the given username. Lookup is exact
// match (case-sensitive) to match the signup constraint.
func PromoteUser(ctx context.Context, username string) error {
	return setRole(ctx, username, RoleAdmin)
}

// DemoteUser strips the admin role and returns the user to player.
func DemoteUser(ctx context.Context, username string) error {
	return setRole(ctx, username, RolePlayer)
}

func setRole(ctx context.Context, username, role string) error {
	app, closer, err := openApp(ctx)
	if err != nil {
		return err
	}
	defer closer()

	username = strings.TrimSpace(username)
	if username == "" {
		return errors.New("username is required")
	}
	u, err := app.Queries.GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("user %q not found", username)
		}
		return err
	}
	if u.Role == role {
		fmt.Printf("noop: %s already has role %q\n", username, role)
		return nil
	}
	if err := app.Queries.SetUserRole(ctx, u.ID, role); err != nil {
		return err
	}
	fmt.Printf("ok: %s role changed from %q to %q\n", username, u.Role, role)
	return nil
}

// PrintStats dumps high-level counters to stdout in a stable text format
// suitable for grep / monitoring scripts. The numbers are the same ones the
// public /api/v1/stats endpoint exposes, plus active session count which is
// admin-only.
func PrintStats(ctx context.Context) error {
	app, closer, err := openApp(ctx)
	if err != nil {
		return err
	}
	defer closer()

	overview, err := app.Stats.Overview(ctx)
	if err != nil {
		return err
	}
	sessions, err := app.Queries.CountActiveSessions(ctx)
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "metric\tvalue")
	fmt.Fprintln(tw, "-------\t-----")
	fmt.Fprintf(tw, "universes\t%d\n", overview.Universes)
	fmt.Fprintf(tw, "players\t%d\n", overview.Players)
	fmt.Fprintf(tw, "planets\t%d\n", overview.Planets)
	fmt.Fprintf(tw, "online_players\t%d\n", overview.OnlinePlayers)
	fmt.Fprintf(tw, "fleets_in_flight\t%d\n", overview.FleetsInFlight)
	fmt.Fprintf(tw, "active_sessions\t%d\n", sessions)
	fmt.Fprintf(tw, "uptime_seconds\t%d\n", overview.UptimeSeconds)
	return tw.Flush()
}

// ListUsers prints up to `limit` users in id order, starting from `offset`.
// Output is tab-aligned so eyeballing works in any terminal.
func ListUsers(ctx context.Context, limit, offset int) error {
	app, closer, err := openApp(ctx)
	if err != nil {
		return err
	}
	defer closer()

	users, err := app.Queries.ListUsers(ctx, limit, offset)
	if err != nil {
		return err
	}
	if len(users) == 0 {
		fmt.Println("no users")
		return nil
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "id\tusername\temail\trole\tcreated_at\tlast_seen")
	for _, u := range users {
		lastSeen := "-"
		if u.LastSeenAt != nil {
			lastSeen = u.LastSeenAt.UTC().Format(time.RFC3339)
		}
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s\n",
			u.ID, u.Username, u.Email, u.Role,
			u.CreatedAt.UTC().Format(time.RFC3339), lastSeen,
		)
	}
	return tw.Flush()
}
