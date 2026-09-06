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

func recordArguments() []string {
	args := []string{"record"}
	for _, flag := range []string{"x86-summary", "aarch64-summary", "x86-release-record", "aarch64-release-record", "expected-revision", "output", "approved-signer", "oidc-issuer"} {
		args = append(args, "--"+flag, flag+"-fixture")
	}
	return args
}

func TestRecordPassesSelectionAndStreamsToSigner(t *testing.T) {
	var output, diagnostic bytes.Buffer
	calls := 0
	command := newCommand(nil, func(ctx context.Context, spec string, options acceptance.RecordOptions, stdout, stderr io.Writer) (acceptance.RecordResult, error) {
		calls++
		require.Equal(t, t.Context(), ctx)
		require.Equal(t, "chosen.toml", spec)
		require.Equal(t, "x86-summary-fixture", options.X86Summary)
		require.Equal(t, "aarch64-summary-fixture", options.ARM64Summary)
		require.Equal(t, "x86-release-record-fixture", options.X86ReleaseRecord)
		require.Equal(t, "aarch64-release-record-fixture", options.ARM64ReleaseRecord)
		require.Equal(t, "expected-revision-fixture", options.ExpectedRevision)
		require.Equal(t, "output-fixture", options.Output)
		require.Equal(t, "approved-signer-fixture", options.ApprovedSigner)
		require.Equal(t, "oidc-issuer-fixture", options.OIDCIssuer)
		require.Same(t, &output, stdout)
		require.Same(t, &diagnostic, stderr)
		return acceptance.RecordResult{RecordPath: "record.json", BundlePath: "bundle.json"}, nil
	}, nil)
	command.SetOut(&output)
	command.SetErr(&diagnostic)
	command.SetArgs(append(recordArguments(), "--spec", "chosen.toml"))
	require.NoError(t, command.ExecuteContext(t.Context()))
	require.Equal(t, 1, calls)
	require.Equal(t, "Acceptance record: record.json\nSignature bundle: bundle.json\n", output.String())
}

func TestRecordReportsSigningAndOutputFailures(t *testing.T) {
	failure := errors.New("signing failed")
	command := newCommand(nil, func(context.Context, string, acceptance.RecordOptions, io.Writer, io.Writer) (acceptance.RecordResult, error) {
		return acceptance.RecordResult{}, failure
	}, nil)
	command.SetArgs(recordArguments())
	var output bytes.Buffer
	command.SetOut(&output)
	require.ErrorIs(t, command.Execute(), failure)
	require.Empty(t, output.String())
	failure = nil
	command.SetOut(failingWriter{})
	require.ErrorIs(t, command.Execute(), io.ErrClosedPipe)
}
