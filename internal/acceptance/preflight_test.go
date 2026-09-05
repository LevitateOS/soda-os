package acceptance

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestOperatorPromptCancellationClosesItsOwnedReader(t *testing.T) {
	reader, writer := io.Pipe()
	defer writer.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- awaitEnter(ctx, reader) }()
	cancel()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("prompt ignored cancellation")
	}
	_, err := writer.Write([]byte("late input\n"))
	require.ErrorIs(t, err, io.ErrClosedPipe)
}

func TestOperatorPromptRequiresAnActualLine(t *testing.T) {
	require.NoError(t, awaitEnter(context.Background(), io.NopCloser(strings.NewReader("\n"))))
	require.ErrorIs(t, awaitEnter(context.Background(), io.NopCloser(strings.NewReader(""))), io.EOF)
}
