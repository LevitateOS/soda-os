package main

import (
	"fmt"

	"github.com/LevitateOS/soda-os/internal/build/installer"
	"github.com/spf13/cobra"
)

func qcow2Command(specPath, architecture *string, connect imageFactory) *cobra.Command {
	var options installer.QCOW2Options
	command := &cobra.Command{
		Use:   "qcow2",
		Short: "build and compress a reusable QCOW2 from a local Soda OCI archive",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			builder, err := connect(*specPath, *architecture, command.OutOrStdout(), command.ErrOrStderr())
			if err != nil {
				return err
			}
			result, err := builder.BuildQCOW2(command.Context(), options)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(command.OutOrStdout(), "Built QCOW2: %s\nChecksum: %s\nCompressed: %s\nChecksum: %s.sha256\n", result.Path, result.SHA256, result.CompressedPath, result.CompressedPath)
			return err
		},
	}
	flags := command.Flags()
	flags.StringVar(&options.ArchivePath, "archive", "", "local single-platform Soda OCI archive")
	flags.StringVar(&options.ToolLock, "tool-lock", "", "pinned Image Builder tool contract (defaults to the selected platform lock)")
	flags.StringVar(&options.OutputDir, "output-dir", ".artifacts/images", "QCOW2 artifact directory")
	_ = command.MarkFlagRequired("archive")
	return command
}
