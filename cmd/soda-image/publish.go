package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func publishCommand(specPath, architecture *string, connect imageFactory) *cobra.Command {
	var archive string
	command := &cobra.Command{
		Use:   "publish",
		Short: "publish one native OCI archive and advance its development tag",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			builder, err := connect(*specPath, *architecture, command.OutOrStdout(), command.ErrOrStderr())
			if err != nil {
				return err
			}
			image, err := builder.PublishImage(command.Context(), archive)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(command.OutOrStdout(), "Published %s from archive source %s\n", image.Reference(), image.Revision)
			return err
		},
	}
	flags := command.Flags()
	flags.StringVar(&archive, "archive", "", "local matching-native OCI archive")
	_ = command.MarkFlagRequired("archive")
	return command
}
