package acceptance

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFixturePortsAndNamesCannotCollide(t *testing.T) {
	for _, port := range []int{18080, 18081} {
		ports := defaultRunOptions(RunOptions{}).Ports
		ports.Registry = port
		require.ErrorContains(t, validateHostPorts(ports), "distinct")
	}
	for _, username := range []string{"alice", "bob", "obsolete", externalGitFixtureUsername} {
		require.ErrorContains(t, validateAdministratorInput(AdministratorInput{Username: username}), "collides")
	}
}

func TestQEMUPreflightChecksInputsWithoutCreatingDisks(t *testing.T) {
	installAcceptanceCommand(t, "fixture-qemu", "exit 0")
	t.Setenv("SODA_QEMU", "fixture-qemu")
	path := filepath.Join(t.TempDir(), "firmware")
	t.Setenv("SODA_QEMU_FIRMWARE", path)
	t.Setenv("SODA_QEMU_VARS", path)
	require.ErrorContains(t, requireQEMUInputs(), "firmware")
	require.NoError(t, os.WriteFile(path, []byte("firmware"), 0o600))
	require.NoError(t, requireQEMUInputs())
	require.NoFileExists(t, path+".OVMF_VARS.fd")
	t.Setenv("SODA_QEMU", filepath.Join(t.TempDir(), "missing-executable"))
	require.ErrorContains(t, requireQEMUInputs(), "unavailable")
}
