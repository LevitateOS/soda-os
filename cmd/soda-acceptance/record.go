package main

import (
	"fmt"

	"github.com/LevitateOS/soda-os/internal/acceptance"
	"github.com/spf13/cobra"
)

func recordCommand(sign recordSigner) *cobra.Command {
	var options acceptance.RecordOptions
	var specPath string
	command := &cobra.Command{
		Use:   "record",
		Short: "Qualify, combine, and sign complete x86-64 and AArch64 run reports",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			result, err := sign(command.Context(), specPath, options, command.OutOrStdout(), command.ErrOrStderr())
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(command.OutOrStdout(), "Acceptance record: %s\nSignature bundle: %s\n", result.RecordPath, result.BundlePath)
			return err
		},
	}
	flags := command.Flags()
	flags.StringVar(&specPath, "spec", "distro/soda.toml", "Soda distribution specification for both candidate records")
	flags.StringVar(&options.X86Summary, "x86-summary", "", "schema-2 x86-64 run report with complete qualification evidence")
	flags.StringVar(&options.ARM64Summary, "aarch64-summary", "", "schema-2 AArch64 run report with complete qualification evidence")
	flags.StringVar(&options.X86ReleaseRecord, "x86-release-record", "", "strict x86-64 candidate release record")
	flags.StringVar(&options.ARM64ReleaseRecord, "aarch64-release-record", "", "strict AArch64 candidate release record")
	flags.StringVar(&options.ExpectedRevision, "expected-revision", "", "exact main revision named by both runs")
	flags.StringVar(&options.Output, "output", "", "new strict JSON acceptance record")
	flags.StringVar(&options.ApprovedSigner, "approved-signer", "", "expected Sigstore certificate identity")
	flags.StringVar(&options.OIDCIssuer, "oidc-issuer", "", "expected Sigstore certificate OIDC issuer")
	for _, name := range []string{"x86-summary", "aarch64-summary", "x86-release-record", "aarch64-release-record", "expected-revision", "output", "approved-signer", "oidc-issuer"} {
		_ = command.MarkFlagRequired(name)
	}
	return command
}
