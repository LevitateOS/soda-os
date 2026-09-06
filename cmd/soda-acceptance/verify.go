package main

import "github.com/spf13/cobra"

func verifyCommand(verify recordVerifier) *cobra.Command {
	var path, revision string
	command := &cobra.Command{
		Use:   "verify",
		Short: "Verify signed complete qualification for the exact release revision",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return verify(command.Context(), path, revision, command.OutOrStdout(), command.ErrOrStderr())
		},
	}
	flags := command.Flags()
	flags.StringVar(&path, "record", "", "acceptance JSON with adjacent .sigstore.json bundle")
	flags.StringVar(&revision, "expected-revision", "", "exact production source revision")
	_ = command.MarkFlagRequired("record")
	_ = command.MarkFlagRequired("expected-revision")
	return command
}
