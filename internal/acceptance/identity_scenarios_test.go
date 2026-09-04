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
