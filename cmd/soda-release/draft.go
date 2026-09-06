package main

import (
	"fmt"

	"github.com/LevitateOS/soda-os/internal/build/release"
	"github.com/spf13/cobra"
)

func draftCommand(specPath *string, connect publicationFactory) *cobra.Command {
	var options release.DraftOptions
	command := &cobra.Command{
		Use:   "draft",
		Short: "create an empty GitHub draft for the clean Soda source revision",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			publication, err := connect(*specPath, command.OutOrStdout(), command.ErrOrStderr())
			if err != nil {
				return err
			}
			result, err := publication.Draft(command.Context(), options)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(command.OutOrStdout(), "Created GitHub draft %s for %s\n", result.Tag, result.Revision)
			return err
		},
	}
	flags := command.Flags()
	flags.StringVar(&options.NotesPath, "notes-file", "", "regular file containing the release notes")
	flags.StringVar(&options.AArch64RecordPath, "aarch64-record", "", "matching-native AArch64 Soda release record")
	flags.StringVar(&options.X86RecordPath, "x86_64-record", "", "matching-native x86-64 Soda release record")
	for _, name := range []string{"notes-file", "aarch64-record", "x86_64-record"} {
		_ = command.MarkFlagRequired(name)
	}
	return command
}
