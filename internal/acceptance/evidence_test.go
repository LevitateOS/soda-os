package acceptance

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEvidenceRetainsDiagnosticsAndRedactsSecrets(t *testing.T) {
	evidence, err := CreateEvidence(filepath.Join(t.TempDir(), "evidence"))
	require.NoError(t, err)
	require.NoError(t, evidence.Write("failed/guest.stderr", []byte("native failure with secret-value\n")))

	err = evidence.Sanitize([]Secret{{Label: "tailscale-auth-key", Value: []byte("secret-value\n")}})
	require.ErrorContains(t, err, "credential material reached evidence")
	contents, readErr := os.ReadFile(filepath.Join(evidence.Root, "failed/guest.stderr"))
	require.NoError(t, readErr)
	require.Equal(t, "native failure with [REDACTED]\n", string(contents))
	report, readErr := os.ReadFile(filepath.Join(evidence.Root, "secret-absence.txt"))
	require.NoError(t, readErr)
	require.Contains(t, string(report), "result=fail-redacted")
}

func TestEvidenceRejectsEscapingPaths(t *testing.T) {
	evidence, err := CreateEvidence(filepath.Join(t.TempDir(), "evidence"))
	require.NoError(t, err)
	require.ErrorContains(t, evidence.Write("../outside", []byte("no")), "escapes")
}
