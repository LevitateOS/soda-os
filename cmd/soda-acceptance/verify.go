package main

import (
	"os"

	"github.com/LevitateOS/soda-os/internal/acceptance"
	"github.com/LevitateOS/soda-os/internal/process"
	"github.com/spf13/cobra"
)

func verifyCommand() *cobra.Command {
	var path, revision string
	command := &cobra.Command{
		Use: "verify", Short: "Verify signed complete qualification for the exact release revision",
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return acceptance.VerifySignedRecord(command.Context(), path, revision, process.OSRunner{Stdout: os.Stdout, Stderr: os.Stderr})
		},
	}
	command.Flags().StringVar(&path, "record", "", "acceptance JSON with adjacent .sigstore.json bundle")
	command.Flags().StringVar(&revision, "expected-revision", "", "exact production source revision")
	_ = command.MarkFlagRequired("record")
	_ = command.MarkFlagRequired("expected-revision")
	return command
}
