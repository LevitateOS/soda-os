package main

import (
	"fmt"

	"github.com/LevitateOS/soda-os/internal/build/release"
	"github.com/spf13/cobra"
)

func recordCommand(specPath, architecture *string, connect imageFactory) *cobra.Command {
	var options release.RecordOptions
	command := &cobra.Command{
		Use:   "record",
		Short: "inspect a local OCI archive and installer ISO and write release metadata",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			builder, err := connect(*specPath, *architecture, command.OutOrStdout(), command.ErrOrStderr())
			if err != nil {
				return err
			}
			result, err := builder.CreateRecord(command.Context(), options)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(command.OutOrStdout(), "Recorded %s\nRelease record: %s\n", result.ImageReference, result.RecordPath)
			return err
		},
	}
	flags := command.Flags()
	flags.StringVar(&options.ArchivePath, "archive", "", "path to the selected-architecture Soda OCI archive")
	flags.StringVar(&options.ISOPath, "iso", "", "installer ISO built from the local OCI archive")
	flags.StringVar(&options.QCOW2Path, "qcow2", "", "raw QCOW2 built from the local OCI archive")
	flags.StringVar(&options.QCOW2ZSTPath, "qcow2-zst", "", "compressed QCOW2 download built from --qcow2")
	flags.StringVar(&options.OutputDir, "output-dir", ".artifacts/releases", "release record directory")
	flags.StringVar(&options.InstallerArchive, "installer-archive", "", "build-only installer environment used to inspect --iso (defaults to the selected architecture artifact)")
	flags.StringVar(&options.InstallerToolLock, "installer-tool-lock", "", "pinned Image Builder contract used to inspect --iso (defaults to the selected platform lock)")
	for _, name := range []string{"archive", "iso", "qcow2", "qcow2-zst"} {
		_ = command.MarkFlagRequired(name)
	}
	return command
}
