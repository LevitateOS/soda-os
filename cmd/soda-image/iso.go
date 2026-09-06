package main

import (
	"fmt"

	"github.com/LevitateOS/soda-os/internal/build/installer"
	"github.com/spf13/cobra"
)

func isoCommand(specPath, architecture *string, connect imageFactory) *cobra.Command {
	var options installer.Options
	command := &cobra.Command{
		Use:   "iso",
		Short: "build a platform-matched installer ISO from a local Soda OCI archive",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			builder, err := connect(*specPath, *architecture, command.OutOrStdout(), command.ErrOrStderr())
			if err != nil {
				return err
			}
			path, err := builder.BuildISO(command.Context(), options)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(command.OutOrStdout(), "Built installer ISO: %s\nChecksum: %s.sha256\n", path, path)
			return err
		},
	}
	flags := command.Flags()
	flags.StringVar(&options.ArchivePath, "archive", "", "local single-platform Soda OCI archive")
	flags.StringVar(&options.ToolLock, "tool-lock", "", "pinned Image Builder tool contract (defaults to the selected platform lock)")
	flags.StringVar(&options.OutputDir, "output-dir", ".artifacts/images", "installer artifact directory")
	_ = command.MarkFlagRequired("archive")
	return command
}
