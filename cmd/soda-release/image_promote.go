package main

import (
	"fmt"

	"github.com/LevitateOS/soda-os/internal/build/release"
	"github.com/spf13/cobra"
)

func imagePromoteCommand(specPath *string, connect publicationFactory) *cobra.Command {
	var options release.ImagePromoteOptions
	command := &cobra.Command{
		Use:   "image-promote",
		Short: "promote one verified immutable candidate to its versioned GHCR tag",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			publication, err := connect(*specPath, command.OutOrStdout(), command.ErrOrStderr())
			if err != nil {
				return err
			}
			result, err := publication.ImagePromote(command.Context(), options)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(command.OutOrStdout(), "Promoted %s image: %s\n", result.Architecture, result.Reference)
			return err
		},
	}
	flags := command.Flags()
	flags.StringVar(&options.Architecture, "architecture", "", "matching-native Soda architecture")
	flags.StringVar(&options.RecordPath, "record", "", "matching-native schema-3 Soda release record")
	_ = command.MarkFlagRequired("architecture")
	_ = command.MarkFlagRequired("record")
	return command
}
