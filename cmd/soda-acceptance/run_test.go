package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/LevitateOS/soda-os/internal/acceptance"
	"github.com/stretchr/testify/require"
)

func runArguments() []string {
	args := []string{"run"}
	for _, flag := range []string{
		"evidence", "candidate-oci", "candidate-iso", "candidate-qcow2",
		"fallback-oci", "administrator-private-key", "administrator-public-key",
		"administrator-password-file",
	} {
		args = append(args, "--"+flag, flag+"-fixture")
	}
	return args
}

func TestRunFlagsRequireOnlyConsumedCredentialInputs(t *testing.T) {
	command := runCommand(nil)
	require.Nil(t, command.Flags().Lookup("tailscale-auth-key-file"))
	require.NoError(t, command.ParseFlags(runArguments()[1:]))
	require.NoError(t, command.ValidateRequiredFlags())
	require.ErrorContains(t, runCommand(nil).ParseFlags([]string{"--tailscale-auth-key-file", "unused"}), "unknown flag")
}

func TestRunPortHelpDoesNotClaimLANEvidence(t *testing.T) {
	command := runCommand(nil)
	for _, name := range []string{"ssh-port", "cockpit-port", "forgejo-port"} {
		require.Contains(t, command.Flags().Lookup(name).Usage, "loopback-forwarded")
		require.Contains(t, command.Flags().Lookup(name).Usage, "not independent LAN evidence")
	}
}

func TestRunRetainsReportsOnFailureAndRoutesProgress(t *testing.T) {
	failure := errors.New("acceptance failed")
	var output bytes.Buffer
	calls := 0
	command := newCommand(func(ctx context.Context, options acceptance.RunOptions, stdout io.Writer) (acceptance.RunResult, error) {
		calls++
		require.Equal(t, t.Context(), ctx)
		require.Same(t, &output, stdout)
		require.Equal(t, "evidence-fixture", options.EvidenceDir)
		require.Equal(t, "candidate-iso-fixture", options.Candidate.ISO)
		require.Equal(t, "candidate-qcow2-fixture", options.Candidate.QCOW2)
		require.Equal(t, "fallback-oci-fixture", options.Fallback.OCI)
		require.Equal(t, "soda-test", options.Administrator.Username)
		require.Equal(t, "40G", options.DiskSize)
		require.Equal(t, 2222, options.Ports.SSH)
		_, err := io.WriteString(stdout, "suite progress\n")
		return acceptance.RunResult{EvidenceDir: "evidence", SummaryPath: "summary.json"}, errors.Join(failure, err)
	})
	command.SetOut(&output)
	command.SetArgs(runArguments())
	require.ErrorIs(t, command.ExecuteContext(t.Context()), failure)
	require.Equal(t, 1, calls)
	require.Equal(t, "suite progress\nEvidence: evidence\nRun report (not release qualification): summary.json\n", output.String())
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestRunPreservesCancellationAndReportWriteFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	command := newCommand(func(ctx context.Context, _ acceptance.RunOptions, _ io.Writer) (acceptance.RunResult, error) {
		return acceptance.RunResult{EvidenceDir: "partial"}, ctx.Err()
	})
	command.SetOut(failingWriter{})
	command.SetArgs(runArguments())
	err := command.ExecuteContext(ctx)
	require.ErrorIs(t, err, context.Canceled)
	require.ErrorIs(t, err, io.ErrClosedPipe)
}
