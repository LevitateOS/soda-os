package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunFlagsRequireOnlyConsumedCredentialInputs(t *testing.T) {
	command := runCommand()
	require.Nil(t, command.Flags().Lookup("tailscale-auth-key-file"))
	for _, flag := range []string{
		"evidence", "candidate-record", "candidate-oci", "candidate-iso", "candidate-qcow2",
		"fallback-record", "fallback-oci", "administrator-private-key", "administrator-public-key",
		"administrator-password-file",
	} {
		require.NoError(t, command.Flags().Set(flag, "fixture"))
	}
	// Only validate the CLI boundary; do not run acceptance or touch a host.
	require.NoError(t, command.ValidateRequiredFlags())
	require.ErrorContains(t, runCommand().ParseFlags([]string{"--tailscale-auth-key-file", "unused"}), "unknown flag")
}

func TestRunPortHelpDoesNotClaimLANEvidence(t *testing.T) {
	command := runCommand()
	for _, name := range []string{"ssh-port", "cockpit-port", "forgejo-port"} {
		require.Contains(t, command.Flags().Lookup(name).Usage, "loopback-forwarded")
		require.Contains(t, command.Flags().Lookup(name).Usage, "not independent LAN evidence")
	}
}
