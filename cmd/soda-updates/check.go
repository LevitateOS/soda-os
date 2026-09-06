package main

import (
	"context"
	"encoding/json"
	"time"

	"github.com/spf13/cobra"
)

func checkCommand(connect updateFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Check the native image source and read deployment status",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(command.Context(), 3*time.Minute)
			defer cancel()
			// Native check progress belongs on stderr, never JSON stdout.
			host, err := connect(command.ErrOrStderr(), command.ErrOrStderr()).Check(ctx)
			if err != nil {
				return err
			}
			return json.NewEncoder(command.OutOrStdout()).Encode(host)
		},
	}
}
