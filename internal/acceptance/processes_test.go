package acceptance

import (
	"context"
	"os/exec"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestProcessStopTerminatesOnlyItsProcessGroup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix process groups are required")
	}
	command := exec.Command("sleep", "60")
	command.SysProcAttr = nil
	require.NoError(t, command.Start())
	t.Cleanup(func() { _ = command.Process.Kill() })

	managed, err := StartProcess(context.Background(), ProcessSpec{Name: "sh", Args: []string{"-c", "sleep 60 & wait"}})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, managed.Stop(ctx))
	require.NoError(t, syscall.Kill(command.Process.Pid, 0))
}
