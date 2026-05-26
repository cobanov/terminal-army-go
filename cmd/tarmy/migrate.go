package main

import (
	"errors"

	"github.com/cobanov/terminal-army-go/internal/config"
	"github.com/cobanov/terminal-army-go/internal/store"
	"github.com/spf13/cobra"
)

func newMigrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Manage database migrations",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "up",
			Short: "Apply all pending migrations",
			RunE: func(cmd *cobra.Command, args []string) error {
				cfg, err := config.Load()
				if err != nil {
					return err
				}
				return store.MigrateUp(cfg.DatabaseURL)
			},
		},
		&cobra.Command{
			Use:   "down",
			Short: "Roll back every migration",
			RunE: func(cmd *cobra.Command, args []string) error {
				cfg, err := config.Load()
				if err != nil {
					return err
				}
				return store.MigrateDown(cfg.DatabaseURL)
			},
		},
		&cobra.Command{
			Use:   "version",
			Short: "Show current migration version",
			RunE: func(cmd *cobra.Command, args []string) error {
				cfg, err := config.Load()
				if err != nil {
					return err
				}
				v, dirty, err := store.MigrationVersion(cfg.DatabaseURL)
				if err != nil {
					return err
				}
				if dirty {
					return errors.New("migration state is dirty: manual repair required")
				}
				cmd.Printf("migration version: %d\n", v)
				return nil
			},
		},
	)
	return cmd
}
