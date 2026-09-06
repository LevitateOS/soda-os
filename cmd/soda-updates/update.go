package main

import "github.com/spf13/cobra"

func updateCommand(connect updateFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update from the native image source and restart when needed",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return connect(command.OutOrStdout(), command.ErrOrStderr()).Update(command.Context())
		},
	}
}
