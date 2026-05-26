package main

import (
	"github.com/cobanov/terminal-army-go/internal/svc/admin"
	"github.com/spf13/cobra"
)

// newAdminCmd assembles the `tarmy admin ...` subcommand tree. Each leaf opens
// its own short-lived pool inside the admin package, so the commands work
// without a running serve process and exit cleanly when done.
func newAdminCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "admin",
		Short: "Administrative commands (seed-universe, promote, stats, list-users)",
	}

	var listLimit, listOffset int

	cmd.AddCommand(
		&cobra.Command{
			Use:   "seed-universe",
			Short: "Create the default universe from TARMY_DEFAULT_UNIVERSE_* settings (idempotent)",
			RunE: func(cmd *cobra.Command, args []string) error {
				return admin.SeedDefaultUniverse(cmd.Context())
			},
		},
		&cobra.Command{
			Use:   "promote [username]",
			Short: "Grant the admin role to a user",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return admin.PromoteUser(cmd.Context(), args[0])
			},
		},
		&cobra.Command{
			Use:   "demote [username]",
			Short: "Revoke the admin role and restore the player role",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return admin.DemoteUser(cmd.Context(), args[0])
			},
		},
		&cobra.Command{
			Use:   "stats",
			Short: "Print server-wide counters (universes, players, planets, sessions, uptime)",
			RunE: func(cmd *cobra.Command, args []string) error {
				return admin.PrintStats(cmd.Context())
			},
		},
		func() *cobra.Command {
			c := &cobra.Command{
				Use:   "list-users",
				Short: "List user accounts in id order",
				RunE: func(cmd *cobra.Command, args []string) error {
					return admin.ListUsers(cmd.Context(), listLimit, listOffset)
				},
			}
			c.Flags().IntVar(&listLimit, "limit", 100, "max rows to return")
			c.Flags().IntVar(&listOffset, "offset", 0, "id-order offset")
			return c
		}(),
	)
	return cmd
}
