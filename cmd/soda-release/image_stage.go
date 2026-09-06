package main

import (
	"fmt"

	"github.com/LevitateOS/soda-os/internal/build/release"
	"github.com/spf13/cobra"
)

func imageStageCommand(specPath *string, connect publicationFactory) *cobra.Command {
	var options release.ImageStageOptions
	command := &cobra.Command{
		Use:   "image-stage",
		Short: "publish and verify one immutable matching-native GHCR candidate image",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			publication, err := connect(*specPath, command.OutOrStdout(), command.ErrOrStderr())
			if err != nil {
				return err
			}
			result, err := publication.ImageStage(command.Context(), options)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(command.OutOrStdout(), "Published %s candidate: %s\n", result.Architecture, result.Reference)
			return err
		},
	}
	flags := command.Flags()
	flags.StringVar(&options.Architecture, "architecture", "", "matching-native Soda architecture")
	flags.StringVar(&options.ArchivePath, "archive", "", "matching-native local Soda OCI archive")
	_ = command.MarkFlagRequired("architecture")
	_ = command.MarkFlagRequired("archive")
	return command
}
