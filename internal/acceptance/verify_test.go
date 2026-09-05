package acceptance

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/process"
	"github.com/stretchr/testify/require"
)

func TestReleaseVerificationRequiresCompleteBoundSiblings(t *testing.T) {
	for _, change := range []string{"partial", "duplicate", "revision", "suite", "schema", "signer", "time", "bundle"} {
		t.Run(change, func(t *testing.T) {
			record := qualifiedRecordFixture()
			corruptAcceptanceRecord(&record, change)
			path := filepath.Join(t.TempDir(), "acceptance.json")
			require.NoError(t, writeNewJSON(path, record))
			if change != "bundle" {
				require.NoError(t, os.WriteFile(path+".sigstore.json", []byte("signature fixture"), 0o600))
			}
			runner := &verificationRunner{}
			err := VerifySignedRecord(context.Background(), path, strings.Repeat("a", 40), runner)
			require.Error(t, err)
			require.Empty(t, runner.commands)
		})
	}
}

func qualifiedRecordFixture() AcceptanceRecord {
	x86 := qualificationFixture()
	arm := qualificationFixture()
	arm.Architecture, arm.Platform = "aarch64", "linux/arm64"
	return combinedRecord([]RunSummary{x86, arm}, acceptanceSigner)
}

func TestReleaseVerificationRequiresCryptographicVerification(t *testing.T) {
	for _, signatureErr := range []error{nil, errors.New("invalid signature")} {
		path := filepath.Join(t.TempDir(), "acceptance.json")
		record := qualifiedRecordFixture()
		require.NoError(t, writeNewJSON(path, record))
		require.NoError(t, os.WriteFile(path+".sigstore.json", []byte("signature fixture"), 0o600))
		runner := &verificationRunner{err: signatureErr}
		err := VerifySignedRecord(context.Background(), path, record.SourceRevision, runner)
		require.ErrorIs(t, err, signatureErr)
		require.Len(t, runner.commands, 1)
		require.Contains(t, runner.commands[0].Args, acceptanceSigner)
		require.Contains(t, runner.commands[0].Args, acceptanceIssuer)
	}
}

func corruptAcceptanceRecord(record *AcceptanceRecord, change string) {
	switch change {
	case "partial":
		delete(record.Architectures[0].Scenarios, "public-ingress-rejection")
	case "duplicate":
		record.Architectures[1] = record.Architectures[0]
	case "revision":
		record.SourceRevision = strings.Repeat("b", 40)
	case "suite":
		record.Architectures[1].SuiteRevision = strings.Repeat("b", 40)
	case "schema":
		record.SchemaVersion = 1
	case "signer":
		record.ApprovedSigner = "unapproved signer"
	case "time":
		record.CompletedAt = "2020-01-01T00:00:00Z"
	}
}

type verificationRunner struct {
	recordRunner
	err error
}

func (runner *verificationRunner) Run(ctx context.Context, command process.Command) error {
	_ = runner.recordRunner.Run(ctx, command)
	return runner.err
}

func TestSummaryRejectsDuplicateFieldsAtBothLevels(t *testing.T) {
	contents, err := json.Marshal(qualificationFixture())
	require.NoError(t, err)
	for _, replacement := range []struct{ old, new string }{
		{`"schema_version":2`, `"schema_version":1,"schema_version":2`},
		{`"ssh-transports":"pass"`, `"ssh-transports":"fail","ssh-transports":"pass"`},
	} {
		var summary RunSummary
		err = json.Unmarshal([]byte(strings.Replace(string(contents), replacement.old, replacement.new, 1)), &summary)
		require.ErrorContains(t, err, "duplicate")
	}
}

func TestSigningBindsBothCandidateRecords(t *testing.T) {
	for _, architecture := range []string{"x86_64", "aarch64"} {
		t.Run(architecture, func(t *testing.T) {
			directory := t.TempDir()
			x86, arm, armRecord := writeRecordInputs(t, directory)
			path := filepath.Join(directory, architecture+".release.json")
			writeCandidateRecordFixture(t, path, architecture, strings.Repeat("a", 40), "sha256:"+strings.Repeat("f", 64))
			runner := &recordRunner{}
			_, err := CreateSignedRecord(context.Background(), recordOptions(x86, arm, armRecord, filepath.Join(directory, "acceptance.json")), runner)
			require.ErrorContains(t, err, "image digest differs")
			require.Empty(t, runner.commands)
		})
	}
}
