package acceptance

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSigningRequiresSameCoverageOnBothArchitectures(t *testing.T) {
	for _, architecture := range []string{"x86_64", "aarch64"} {
		t.Run(architecture, func(t *testing.T) {
			directory := t.TempDir()
			x86, arm, armRecord := writeRecordInputs(t, directory)
			path := map[string]string{"x86_64": x86, "aarch64": arm}[architecture]
			summary, err := readRunSummary(path)
			require.NoError(t, err)
			delete(summary.Scenarios, "public-ingress-rejection")
			require.NoError(t, WriteRunSummary(path, summary))
			runner := &recordRunner{}
			output := filepath.Join(directory, "acceptance.json")
			_, err = CreateSignedRecord(context.Background(), recordOptions(x86, arm, armRecord, output), runner)
			require.ErrorContains(t, err, "qualify "+architecture)
			require.ErrorContains(t, err, "public-ingress-rejection")
			require.Empty(t, runner.commands)
			require.NoFileExists(t, output)
			require.NoFileExists(t, output+".sigstore.json")
		})
	}
}

func TestSigningRejectsLegacyAndUnknownClaims(t *testing.T) {
	for _, invalid := range []string{"legacy", "unknown"} {
		t.Run(invalid, func(t *testing.T) {
			directory := t.TempDir()
			x86, arm, armRecord := writeRecordInputs(t, directory)
			summary := qualificationFixture()
			if invalid == "legacy" {
				summary.SchemaVersion = 1
			} else {
				summary.Scenarios["unreviewed-claim"] = "pass"
			}
			contents, err := json.Marshal(summary)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(x86, contents, 0o600))
			runner := &recordRunner{}
			output := filepath.Join(directory, "acceptance.json")
			_, err = CreateSignedRecord(context.Background(), recordOptions(x86, arm, armRecord, output), runner)
			require.Error(t, err)
			require.Empty(t, runner.commands)
			require.NoFileExists(t, output)
		})
	}
}
