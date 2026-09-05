package acceptance

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLocalForwardedChecksKeepTransportAndEvidenceExplicit(t *testing.T) {
	installAcceptanceCommand(t, "ssh-keyscan", "printf 'fixture-host-key\\n'\n")
	installAcceptanceCommand(t, "ssh", "cat >/dev/null\n")
	installAcceptanceCommand(t, "curl", "exit 0\n")
	evidence, err := CreateEvidence(filepath.Join(t.TempDir(), "evidence"))
	require.NoError(t, err)
	state := runnerState{
		options: defaultRunOptions(RunOptions{}), paths: runPaths{work: t.TempDir()},
		evidence: evidence, checks: checkResults{},
	}
	local := state.localForwardedRemote()
	require.Equal(t, "127.0.0.1", local.Host)
	require.Equal(t, state.options.Ports.SSH, local.Port)
	require.Equal(t, state.options.Ports.Cockpit, local.CockpitPort)
	require.NoError(t, state.verifyInitialLocalForwardedAccess(context.Background(), []byte("fixture-password")))
	tailnet := local
	tailnet.Host = "100.64.0.1"
	tailnet.KnownHosts = filepath.Join(state.paths.work, "tailnet-known-hosts")
	_, err = state.verifyLocalForwardedAccess(context.Background(), tailnet, []byte("fixture-password"))
	require.NoError(t, err)
	for _, name := range []string{
		"iso/local-forwarded-before-enrollment.stdout",
		"iso/local-forwarded-forgejo-before-enrollment.txt",
		"iso/local-forwarded-after-tailscale.stdout",
		"iso/tailnet-after-local-forwarded.stdout",
		"iso/tailnet-forgejo-after-local-forwarded.txt",
	} {
		require.FileExists(t, filepath.Join(evidence.Root, name))
	}
	require.Equal(t, checkResults{"tailnet-access": "pass"}, state.checks)
}
