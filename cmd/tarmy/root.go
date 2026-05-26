package main

import (
	"github.com/cobanov/terminal-army-go/internal/version"
	"github.com/spf13/cobra"
)

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "tarmy",
		Short:         "terminal-army: OGame-style strategy game for the terminal",
		Version:       version.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(
		newServeCmd(),
		newPlayCmd(),
		newMigrateCmd(),
		newAdminCmd(),
	)
	return root
}
