package main

import (
	"fmt"

	"github.com/LevitateOS/soda-os/internal/build/release"
	"github.com/spf13/cobra"
)

func recordSignCommand(specPath *string, connect publicationFactory) *cobra.Command {
	var options release.RecordSignOptions
	command := &cobra.Command{
		Use:   "record-sign",
		Short: "keylessly sign one matching-native Soda release record",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			publication, err := connect(*specPath, command.OutOrStdout(), command.ErrOrStderr())
			if err != nil {
				return err
			}
			result, err := publication.SignRecord(command.Context(), options)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(command.OutOrStdout(), "Signed %s record: %s\n", result.Architecture, result.BundlePath)
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
