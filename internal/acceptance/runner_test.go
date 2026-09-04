package acceptance

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFallbackVMCleanupKeepsTailnetLogoutAheadOfPowerOff(t *testing.T) {
	events := []string{}
	state := runnerState{cleanup: &Cleanup{}, logout: func(context.Context) error {
		events = append(events, "logout")
		return nil
	}}
	require.NoError(t, state.registerVMCleanup("fallback/boot-candidate", &VM{}))
	require.Len(t, state.cleanup.actions, 2)
	state.cleanup.actions[0].Run = func(context.Context) error {
		events = append(events, "qemu-stop")
		return nil
	}
	require.NoError(t, state.cleanup.Run(context.Background()))
	require.Equal(t, []string{"logout", "qemu-stop"}, events)
}

func TestFallbackVerificationUsesReleaseIdentityAndExactDigest(t *testing.T) {
	reference := "ghcr.io/levitateos/soda-os@sha256:" + strings.Repeat("a", 64)
	command := fallbackVerificationCommand(reference)
	require.Equal(t, "cosign", command.Name)
	require.Equal(t, reference, command.Args[len(command.Args)-1])
	require.Contains(t, command.Args, "https://github.com/LevitateOS/soda-os/.github/workflows/release.yml@refs/heads/production")
	require.Contains(t, command.Args, "https://token.actions.githubusercontent.com")
}
