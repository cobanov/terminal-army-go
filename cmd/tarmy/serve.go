package main

import (
	"github.com/cobanov/terminal-army-go/internal/httpapi"
	"github.com/spf13/cobra"
)

func newServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run HTTP API server and queue scheduler",
		RunE: func(cmd *cobra.Command, args []string) error {
			return httpapi.Run(cmd.Context())
		},
	}
}
