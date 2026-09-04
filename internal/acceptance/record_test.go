package acceptance

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LevitateOS/soda-os/internal/process"
	"github.com/stretchr/testify/require"
)

type recordRunner struct {
	commands []process.Command
}

func (runner *recordRunner) Run(_ context.Context, command process.Command) error {
	runner.commands = append(runner.commands, command)
	return nil
}

func (runner *recordRunner) Output(context.Context, process.Command) (string, error) {
	return "", nil
}

func TestCreateSignedRecordCombinesExactSiblingRunsAndUsesCosign(t *testing.T) {
	directory := t.TempDir()
	x86 := filepath.Join(directory, "x86.json")
	arm := filepath.Join(directory, "arm.json")
	writeSummaryFixture(t, x86, "x86_64")
	writeSummaryFixture(t, arm, "aarch64")
	output := filepath.Join(directory, "acceptance.json")
	runner := &recordRunner{}

	result, err := CreateSignedRecord(context.Background(), RecordOptions{
		X86Summary: x86, ARM64Summary: arm, Output: output,
		ApprovedSigner: "https://github.com/LevitateOS/soda-os/.github/workflows/acceptance.yml@refs/heads/main",
		OIDCIssuer:     "https://token.actions.githubusercontent.com",
	}, runner)
	require.NoError(t, err)
	require.Equal(t, output+".sigstore.json", result.BundlePath)
	require.Len(t, runner.commands, 2)
	require.Equal(t, "sign-blob", runner.commands[0].Args[0])
	require.Contains(t, runner.commands[1].Args, "--certificate-identity")
	contents, err := os.ReadFile(output)
	require.NoError(t, err)
	require.Contains(t, string(contents), `"architecture": "aarch64"`)
	require.Contains(t, string(contents), `"architecture": "x86_64"`)
}

func TestCreateSignedRecordRejectsMismatchedSiblingSource(t *testing.T) {
	directory := t.TempDir()
	x86 := filepath.Join(directory, "x86.json")
	arm := filepath.Join(directory, "arm.json")
	writeSummaryFixture(t, x86, "x86_64")
	writeSummaryFixture(t, arm, "aarch64")
	contents, err := os.ReadFile(arm)
	require.NoError(t, err)
	contents = []byte(strings.Replace(string(contents), strings.Repeat("a", 40), strings.Repeat("c", 40), 1))
	require.NoError(t, os.WriteFile(arm, contents, 0o600))

	_, err = CreateSignedRecord(context.Background(), RecordOptions{
		X86Summary: x86, ARM64Summary: arm, Output: filepath.Join(directory, "record.json"),
		ApprovedSigner: "signer", OIDCIssuer: "issuer",
	}, &recordRunner{})
	require.ErrorContains(t, err, "same source")
}

func TestCreateSignedRecordKeepsArchitectureSpecificFallbackDigests(t *testing.T) {
	directory := t.TempDir()
	x86 := filepath.Join(directory, "x86.json")
	arm := filepath.Join(directory, "arm.json")
	writeSummaryFixture(t, x86, "x86_64")
	writeSummaryFixture(t, arm, "aarch64")
	contents, err := os.ReadFile(arm)
	require.NoError(t, err)
	armDigest := "sha256:" + strings.Repeat("e", 64)
	contents = []byte(strings.Replace(string(contents), "sha256:"+strings.Repeat("d", 64), armDigest, 1))
	require.NoError(t, os.WriteFile(arm, contents, 0o600))
	output := filepath.Join(directory, "record.json")

	_, err = CreateSignedRecord(context.Background(), RecordOptions{
		X86Summary: x86, ARM64Summary: arm, Output: output,
		ApprovedSigner: "signer", OIDCIssuer: "issuer",
	}, &recordRunner{})
	require.NoError(t, err)
	record, err := os.ReadFile(output)
	require.NoError(t, err)
	require.Contains(t, string(record), armDigest)
	require.Contains(t, string(record), "sha256:"+strings.Repeat("d", 64))
}

func writeSummaryFixture(t *testing.T, path, architecture string) {
	t.Helper()
	platform := map[string]string{"x86_64": "linux/amd64", "aarch64": "linux/arm64"}[architecture]
	summary := RunSummary{
		SchemaVersion: 1, Architecture: architecture, Platform: platform,
		SourceRevision: strings.Repeat("a", 40), SuiteRevision: strings.Repeat("b", 40),
		CandidateDigest: "sha256:" + strings.Repeat("c", 64),
		FallbackDigest:  "sha256:" + strings.Repeat("d", 64),
		Scenarios:       passedScenarios(), CompletedAt: SummaryTime(time.Unix(1_700_000_000, 0)),
	}
	require.NoError(t, WriteRunSummary(path, summary))
}
