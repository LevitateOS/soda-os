package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/LevitateOS/soda-os/internal/acceptance"
	"github.com/spf13/cobra"
)

func runCommand(suite suiteRunner) *cobra.Command {
	var options acceptance.RunOptions
	command := &cobra.Command{
		Use:   "run",
		Short: "Run the product suite on this matching-native host",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			result, err := suite(command.Context(), options, command.OutOrStdout())
			return errors.Join(err, writeRunResult(command.OutOrStdout(), result))
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
	flags.StringVar(&options.Administrator.Username, "administrator", "soda-test", "temporary primary administrator username")
	flags.StringVar(&options.Administrator.PrivateKey, "administrator-private-key", "", "mode-0600 disposable administrator SSH private key")
	flags.StringVar(&options.Administrator.PublicKey, "administrator-public-key", "", "matching disposable administrator SSH public key")
	flags.StringVar(&options.Administrator.Password, "administrator-password-file", "", "mode-0600 file containing one disposable password line")
	flags.StringVar(&options.TempDir, "temp-dir", "", "host directory for disposable VM state; defaults to RUNNER_TEMP")
	flags.StringVar(&options.DiskSize, "disk-size", "40G", "installed test disk size")
	flags.IntVar(&options.Ports.SSH, "ssh-port", 2222, "loopback-forwarded SSH port (not independent LAN evidence)")
	flags.IntVar(&options.Ports.Cockpit, "cockpit-port", 19090, "loopback-forwarded Cockpit port (not independent LAN evidence)")
	flags.IntVar(&options.Ports.Forgejo, "forgejo-port", 13000, "loopback-forwarded Forgejo port (not independent LAN evidence)")
	flags.IntVar(&options.Ports.Registry, "registry-port", 5001, "loopback port for the disposable OCI registry")
	flags.StringVar(&options.RepositoryRoot, "repository", ".", "clean acceptance-suite checkout")
	for _, name := range []string{
		"evidence", "candidate-record", "candidate-oci", "candidate-iso", "candidate-qcow2",
		"fallback-record", "fallback-oci", "administrator-private-key",
		"administrator-public-key", "administrator-password-file",
	} {
		_ = command.MarkFlagRequired(name)
	}
	return command
}

func writeRunResult(output io.Writer, result acceptance.RunResult) error {
	if result.EvidenceDir != "" {
		if _, err := fmt.Fprintln(output, "Evidence:", result.EvidenceDir); err != nil {
			return err
		}
	}
	if result.SummaryPath != "" {
		_, err := fmt.Fprintln(output, "Run report (not release qualification):", result.SummaryPath)
		return err
	}
	return nil
}
