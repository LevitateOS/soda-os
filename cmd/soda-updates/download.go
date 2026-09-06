package main

import (
	"github.com/LevitateOS/soda-os/internal/updates"
	"github.com/spf13/cobra"
)

func downloadCommand(connect updateFactory) *cobra.Command {
	var selection updates.Selection
	command := &cobra.Command{
		Use:   "download",
		Short: "Download the confirmed verified image without restarting",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return connect(command.OutOrStdout(), command.ErrOrStderr()).Download(command.Context(), selection)
		},
	}
	flags := command.Flags()
	flags.StringVar(&selection.Version, "version", "", "confirmed published Soda version")
	flags.StringVar(&selection.Reference, "reference", "", "confirmed exact Soda image digest")
	_ = command.MarkFlagRequired("version")
	_ = command.MarkFlagRequired("reference")
	return command
}
