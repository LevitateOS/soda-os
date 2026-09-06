package main

import (
	"fmt"

	"github.com/LevitateOS/soda-os/internal/build/release"
	"github.com/spf13/cobra"
)

func uploadCommand(specPath *string, connect publicationFactory) *cobra.Command {
	var options release.UploadOptions
	command := &cobra.Command{
		Use:   "upload",
		Short: "validate and upload one matching-native installer asset set",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			publication, err := connect(*specPath, command.OutOrStdout(), command.ErrOrStderr())
			if err != nil {
				return err
			}
			result, err := publication.Upload(command.Context(), options)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(command.OutOrStdout(), "Uploaded %s assets to GitHub draft %s\n", options.Architecture, result.Tag)
			return err
		},
	}
	flags := command.Flags()
	flags.StringVar(&options.Architecture, "architecture", "", "matching-native Soda architecture (aarch64 or x86_64)")
	flags.StringVar(&options.ISOPath, "iso", "", "matching-native Soda installer ISO")
	flags.StringVar(&options.QCOW2ZSTPath, "qcow2-zst", "", "matching-native compressed Soda QCOW2 download")
	flags.StringVar(&options.RecordPath, "record", "", "matching-native Soda release record")
	flags.StringVar(&options.RecordBundlePath, "record-bundle", "", "keyless-signed bundle for the matching-native release record")
	for _, name := range []string{"architecture", "iso", "qcow2-zst", "record", "record-bundle"} {
		_ = command.MarkFlagRequired(name)
	}
	return command
}
