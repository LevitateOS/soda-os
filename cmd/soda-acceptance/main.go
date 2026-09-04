package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/LevitateOS/soda-os/internal/acceptance"
	"github.com/LevitateOS/soda-os/internal/process"
	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:           "soda-acceptance",
		Short:         "Run and record matching-native Soda OS product acceptance",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(runCommand(), recordCommand())
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "soda-acceptance:", err)
		os.Exit(1)
	}
}

func runCommand() *cobra.Command {
	var options acceptance.RunOptions
	command := &cobra.Command{
		Use:   "run",
		Short: "Run the product suite on this matching-native host",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			result, err := acceptance.Run(command.Context(), options, os.Stdout)
			if result.EvidenceDir != "" {
				fmt.Fprintln(os.Stdout, "Evidence:", result.EvidenceDir)
			}
			if err == nil {
				fmt.Fprintln(os.Stdout, "Run summary:", result.SummaryPath)
			}
			return err
		},
	}
	flags := command.Flags()
	flags.StringVar(&options.EvidenceDir, "evidence", "", "new directory for credential-free run evidence")
	flags.StringVar(&options.Candidate.Record, "candidate-record", "", "candidate architecture release record")
	flags.StringVar(&options.Candidate.OCI, "candidate-oci", "", "candidate architecture OCI archive")
	flags.StringVar(&options.Candidate.ISO, "candidate-iso", "", "candidate architecture network installer ISO")
	flags.StringVar(&options.Candidate.QCOW2, "candidate-qcow2", "", "candidate architecture reusable QCOW2")
	flags.StringVar(&options.Fallback.Record, "fallback-record", "", "previous published architecture release record")
	flags.StringVar(&options.Fallback.OCI, "fallback-oci", "", "previous published architecture OCI archive")
	flags.StringVar(&options.TailscaleKey, "tailscale-auth-key-file", "", "mode-0600 file containing one ephemeral one-use guest key")
	flags.StringVar(&options.Administrator.Username, "administrator", "soda-test", "temporary primary administrator username")
	flags.StringVar(&options.Administrator.PrivateKey, "administrator-private-key", "", "mode-0600 disposable administrator SSH private key")
	flags.StringVar(&options.Administrator.PublicKey, "administrator-public-key", "", "matching disposable administrator SSH public key")
	flags.StringVar(&options.Administrator.Password, "administrator-password-file", "", "mode-0600 file containing one disposable password line")
	flags.StringVar(&options.TempDir, "temp-dir", "", "host directory for disposable VM state; defaults to RUNNER_TEMP")
	flags.StringVar(&options.DiskSize, "disk-size", "40G", "installed test disk size")
	flags.IntVar(&options.Ports.SSH, "ssh-port", 2222, "host-forwarded SSH port for the LAN-only QCOW2")
	flags.IntVar(&options.Ports.Cockpit, "cockpit-port", 19090, "host-forwarded Cockpit port for the LAN-only QCOW2")
	flags.IntVar(&options.Ports.Registry, "registry-port", 5001, "loopback port for the disposable OCI registry")
	flags.StringVar(&options.RepositoryRoot, "repository", ".", "clean acceptance-suite checkout")
	for _, name := range []string{
		"evidence", "candidate-record", "candidate-oci", "candidate-iso", "candidate-qcow2",
		"fallback-record", "fallback-oci", "tailscale-auth-key-file", "administrator-private-key",
		"administrator-public-key", "administrator-password-file",
	} {
		_ = command.MarkFlagRequired(name)
	}
	return command
}

func recordCommand() *cobra.Command {
	var options acceptance.RecordOptions
	command := &cobra.Command{
		Use:   "record",
		Short: "Combine and sign matching x86-64 and AArch64 run summaries",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			runner := process.OSRunner{Stdout: os.Stdout, Stderr: os.Stderr}
			result, err := acceptance.CreateSignedRecord(command.Context(), options, runner)
			if err == nil {
				fmt.Fprintf(os.Stdout, "Acceptance record: %s\nSignature bundle: %s\n", result.RecordPath, result.BundlePath)
			}
			return err
		},
	}
	flags := command.Flags()
	flags.StringVar(&options.X86Summary, "x86-summary", "", "passing x86-64 run summary")
	flags.StringVar(&options.ARM64Summary, "aarch64-summary", "", "passing AArch64 run summary")
	flags.StringVar(&options.Output, "output", "", "new strict JSON acceptance record")
	flags.StringVar(&options.ApprovedSigner, "approved-signer", "", "expected Sigstore certificate identity")
	flags.StringVar(&options.OIDCIssuer, "oidc-issuer", "", "expected Sigstore certificate OIDC issuer")
	for _, name := range []string{"x86-summary", "aarch64-summary", "output", "approved-signer", "oidc-issuer"} {
		_ = command.MarkFlagRequired(name)
	}
	return command
}
