package main

import "github.com/spf13/cobra"

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
	return &cobra.Command{
		Use:   "oci",
		Short: "build the Soda bootc OCI archive without loading or publishing it",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			builder, err := connect(*specPath, *architecture, command.OutOrStdout(), command.ErrOrStderr())
			if err != nil {
				return err
			}
			return builder.BuildImage(command.Context())
		},
	}
}
