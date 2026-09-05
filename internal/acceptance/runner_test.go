package acceptance

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFallbackVerificationUsesReleaseIdentityAndExactDigest(t *testing.T) {
	reference := "ghcr.io/levitateos/soda-os@sha256:" + strings.Repeat("a", 64)
	command := fallbackVerificationCommand(reference)
	require.Equal(t, "cosign", command.Name)
	require.Equal(t, reference, command.Args[len(command.Args)-1])
	require.Contains(t, command.Args, "https://github.com/LevitateOS/soda-os/.github/workflows/release.yml@refs/heads/production")
	require.Contains(t, command.Args, "https://token.actions.githubusercontent.com")
}

func TestAcceptanceHostPortsAreDistinct(t *testing.T) {
	require.NoError(t, validateHostPorts(HostPorts{SSH: 2222, Cockpit: 19090, Forgejo: 13000, Registry: 5001}))
	require.ErrorContains(t, validateHostPorts(HostPorts{SSH: 2222, Cockpit: 2222, Forgejo: 13000, Registry: 5001}), "distinct")
}
