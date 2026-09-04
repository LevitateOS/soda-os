package acceptance

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const testPublicKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKJpV7x5Ay34Nh0wiB89JgVG5ZrOxz2SeNUylLBzmrkS"

func TestContainsPublicKeyIgnoresForgejoAndLocalComments(t *testing.T) {
	keys := []forgejoKey{{Key: testPublicKey + " Forgejo comment"}}
	found, err := containsPublicKey(keys, []byte(testPublicKey+" local comment\n"))
	require.NoError(t, err)
	require.True(t, found)
}

func TestWorkspacePublicKeyFromDiagnosticExtractsReportedKey(t *testing.T) {
	diagnostic := []byte(`workspace soda-w-example and its outbound Git key were retained; register public key "` + testPublicKey + ` soda-workspace=soda-w-example" with the authoritative Git host and retry setup`)
	key, err := workspacePublicKeyFromDiagnostic(diagnostic)
	require.NoError(t, err)
	require.Equal(t, testPublicKey, string(key))
}

func TestWorkspacePublicKeyFromDiagnosticRequiresAValidKey(t *testing.T) {
	_, err := workspacePublicKeyFromDiagnostic([]byte("workspace was retained; retry setup"))
	require.ErrorContains(t, err, "did not report its outbound Git public key")

	_, err = workspacePublicKeyFromDiagnostic([]byte("workspace was retained; public key ssh-ed25519 invalid; retry setup"))
	require.ErrorContains(t, err, "validate reported workspace public key")
}
