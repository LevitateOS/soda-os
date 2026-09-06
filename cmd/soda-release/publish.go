package main

import (
	"fmt"

	"github.com/LevitateOS/soda-os/internal/build/release"
	"github.com/spf13/cobra"
)

func publishCommand(specPath *string, connect publicationFactory) *cobra.Command {
	var options release.PublishOptions
	command := &cobra.Command{
		Use:   "publish",
		Short: "publish a complete validated Soda GitHub draft",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			publication, err := connect(*specPath, command.OutOrStdout(), command.ErrOrStderr())
			if err != nil {
				return err
			}
			result, err := publication.Publish(command.Context(), options)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(command.OutOrStdout(), "Published GitHub release %s for %s\n", result.Tag, result.Revision)
			return err
		},
	}
	flags := command.Flags()
	flags.StringVar(&options.AArch64RecordPath, "aarch64-record", "", "matching-native AArch64 signed Soda release record")
	flags.StringVar(&options.X86RecordPath, "x86_64-record", "", "matching-native x86-64 signed Soda release record")
	for _, name := range []string{"aarch64-record", "x86_64-record"} {
		_ = command.MarkFlagRequired(name)
	}
	return command
}
