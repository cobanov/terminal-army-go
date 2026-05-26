package main

import (
	"github.com/cobanov/terminal-army-go/internal/tui"
	"github.com/spf13/cobra"
)

func newPlayCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "play",
		Short: "Launch the terminal client (Bubble Tea)",
		RunE: func(cmd *cobra.Command, args []string) error {
			serverURL, _ := cmd.Flags().GetString("server")
			return tui.Run(cmd.Context(), serverURL)
		},
	}
	cmd.Flags().StringP("server", "s", "http://localhost:8080", "tarmy server base URL")
	return cmd
}
