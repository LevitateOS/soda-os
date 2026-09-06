package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func checkCommand(specPath, architecture *string, connect imageFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "validate the pinned Fedora bootc image contract",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			builder, err := connect(*specPath, *architecture, command.OutOrStdout(), command.ErrOrStderr())
			if err != nil {
				return err
			}
			return builder.Check(command.Context())
		},
	}
}

func rpmCommand(specPath, architecture *string, connect imageFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "rpm",
		Short: "build the locked Soda RPM inputs",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			builder, err := connect(*specPath, *architecture, command.OutOrStdout(), command.ErrOrStderr())
			if err != nil {
				return err
			}
			return builder.BuildRPMs(command.Context())
		},
	}
}

func ociCommand(specPath, architecture *string, connect imageFactory) *cobra.Command {
	var outputDir string
	command := &cobra.Command{
		Use:   "oci",
		Short: "build and lint the Soda bootc OCI archive without publishing it",
		Long:  "Build the OCI archive and load it locally for bootc lint. Print the resulting absolute archive path on stdout; build progress goes to stderr.",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			builder, err := connect(*specPath, *architecture, command.ErrOrStderr(), command.ErrOrStderr())
			if err != nil {
				return err
			}
			archive, err := builder.BuildImage(command.Context(), outputDir)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(command.OutOrStdout(), archive)
			return err
		},
	}
	flags := command.Flags()
	flags.StringVar(&outputDir, "output-dir", ".artifacts/images", "OCI artifact directory (relative to the workspace or absolute)")
	return command
}
