package acceptance

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LevitateOS/soda-os/internal/build/release"
	"github.com/LevitateOS/soda-os/internal/config"
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
	x86, arm, armRecord := writeRecordInputs(t, directory)
	output := filepath.Join(directory, "acceptance.json")
	runner := &recordRunner{}

	result, err := CreateSignedRecord(context.Background(), RecordOptions{
		X86Summary: x86, ARM64Summary: arm, ARM64ReleaseRecord: armRecord,
		ARM64Spec: testARM64Spec(), ExpectedRevision: strings.Repeat("a", 40), Output: output,
		ApprovedSigner: "https://github.com/LevitateOS/soda-os/.github/workflows/native-acceptance-evidence.yml@refs/heads/main",
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
	x86, arm, armRecord := writeRecordInputs(t, directory)
	contents, err := os.ReadFile(arm)
	require.NoError(t, err)
	contents = []byte(strings.Replace(string(contents), strings.Repeat("a", 40), strings.Repeat("c", 40), 1))
	require.NoError(t, os.WriteFile(arm, contents, 0o600))

	_, err = CreateSignedRecord(context.Background(), recordOptions(x86, arm, armRecord, filepath.Join(directory, "record.json")), &recordRunner{})
	require.ErrorContains(t, err, "same source")
}

func TestCreateSignedRecordKeepsArchitectureSpecificFallbackDigests(t *testing.T) {
	directory := t.TempDir()
	x86, arm, armRecord := writeRecordInputs(t, directory)
	contents, err := os.ReadFile(arm)
	require.NoError(t, err)
	armDigest := "sha256:" + strings.Repeat("e", 64)
	contents = []byte(strings.Replace(string(contents), "sha256:"+strings.Repeat("d", 64), armDigest, 1))
	require.NoError(t, os.WriteFile(arm, contents, 0o600))
	output := filepath.Join(directory, "record.json")

	_, err = CreateSignedRecord(context.Background(), recordOptions(x86, arm, armRecord, output), &recordRunner{})
	require.NoError(t, err)
	record, err := os.ReadFile(output)
	require.NoError(t, err)
	require.Contains(t, string(record), armDigest)
	require.Contains(t, string(record), "sha256:"+strings.Repeat("d", 64))
}

func TestCreateSignedRecordRequiresExactWorkflowRevision(t *testing.T) {
	directory := t.TempDir()
	x86, arm, armRecord := writeRecordInputs(t, directory)
	options := recordOptions(x86, arm, armRecord, filepath.Join(directory, "record.json"))
	options.ExpectedRevision = strings.Repeat("f", 40)

	_, err := CreateSignedRecord(context.Background(), options, &recordRunner{})
	require.ErrorContains(t, err, "expected workflow revision")
}

func TestCreateSignedRecordBindsAArch64ReleaseRecord(t *testing.T) {
	directory := t.TempDir()
	x86, arm, armRecord := writeRecordInputs(t, directory)
	writeARMReleaseRecordFixture(t, armRecord, strings.Repeat("a", 40), "sha256:"+strings.Repeat("f", 64))

	_, err := CreateSignedRecord(context.Background(), recordOptions(x86, arm, armRecord, filepath.Join(directory, "record.json")), &recordRunner{})
	require.ErrorContains(t, err, "image digest differs")
}

func writeRecordInputs(t *testing.T, directory string) (string, string, string) {
	t.Helper()
	x86 := filepath.Join(directory, "x86.json")
	arm := filepath.Join(directory, "arm.json")
	armRecord := filepath.Join(directory, "aarch64.release.json")
	writeSummaryFixture(t, x86, "x86_64")
	writeSummaryFixture(t, arm, "aarch64")
	writeARMReleaseRecordFixture(t, armRecord, strings.Repeat("a", 40), "sha256:"+strings.Repeat("c", 64))
	return x86, arm, armRecord
}

func recordOptions(x86, arm, armRecord, output string) RecordOptions {
	return RecordOptions{
		X86Summary: x86, ARM64Summary: arm, ARM64ReleaseRecord: armRecord,
		ARM64Spec: testARM64Spec(), ExpectedRevision: strings.Repeat("a", 40), Output: output,
		ApprovedSigner: "signer", OIDCIssuer: "issuer",
	}
}

func testARM64Spec() config.DistroSpec {
	return config.DistroSpec{
		Identity: config.IdentitySpec{Version: "0.6.1"},
		Base: config.BaseSpec{
			Platform:  "linux/arm64",
			Reference: "quay.io/fedora/fedora-bootc@sha256:" + strings.Repeat("f", 64),
		},
		Platform: config.PlatformSpec{Release: config.PlatformRelease{Channel: "aarch64"}},
	}
}

func writeSummaryFixture(t *testing.T, path, architecture string) {
	t.Helper()
	platform := map[string]string{"x86_64": "linux/amd64", "aarch64": "linux/arm64"}[architecture]
	summary := RunSummary{
		SchemaVersion: 1, Architecture: architecture, Platform: platform,
		SourceRevision: strings.Repeat("a", 40), SuiteRevision: strings.Repeat("a", 40),
		CandidateDigest: "sha256:" + strings.Repeat("c", 64),
		FallbackDigest:  "sha256:" + strings.Repeat("d", 64),
		Scenarios:       passedScenarios(), CompletedAt: SummaryTime(time.Unix(1_700_000_000, 0)),
	}
	require.NoError(t, WriteRunSummary(path, summary))
}

func writeARMReleaseRecordFixture(t *testing.T, path, revision, candidateDigest string) {
	t.Helper()
	record := release.Record{
		SchemaVersion: 3, SodaVersion: "0.6.1", SourceRevision: revision,
		Platform: "linux/arm64", Channel: "aarch64",
		FedoraBaseReference: "quay.io/fedora/fedora-bootc@sha256:" + strings.Repeat("f", 64),
		SodaImageReference:  release.Repository + "@" + candidateDigest,
		ArtifactChecksums: release.ArtifactChecksums{
			RPMInventorySHA256: strings.Repeat("a", 64), ISOChecksum: strings.Repeat("b", 64),
			QCOW2Checksum: strings.Repeat("c", 64), QCOW2ZSTChecksum: strings.Repeat("d", 64),
		},
	}
	contents, err := json.Marshal(record)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, contents, 0o600))
}
