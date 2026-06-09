package main

import (
	"github.com/cobanov/terminal-army-go/internal/tui"
	"github.com/cobanov/terminal-army-go/internal/version"
	"github.com/spf13/cobra"
)

func newRootCmd() *cobra.Command {
	var remote string
	var logout bool
	root := &cobra.Command{
		Use:           "tarmy",
		Short:         "terminal-army: OGame-style strategy game for the terminal",
		Version:       version.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return tui.RunREPL(cmd.Context(), remote, logout)
		},
	}
	root.Flags().StringVarP(&remote, "remote", "r", tui.DefaultServerURL, "terminal.army server URL")
	root.Flags().BoolVar(&logout, "logout", false, "delete saved key for this server and exit")
	root.AddCommand(
		newServeCmd(),
		newPlayCmd(),
		newMigrateCmd(),
		newAdminCmd(),
	)
	return root
}
