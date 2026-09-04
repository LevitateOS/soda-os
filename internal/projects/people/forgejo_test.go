package people

import (
	"context"
	"errors"
	"testing"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/stretchr/testify/require"
)

type forgejoRunner struct {
	result linuxhost.CommandResult
	err    error
	calls  []linuxhost.Command
}

func (runner *forgejoRunner) Run(_ context.Context, command linuxhost.Command) (linuxhost.CommandResult, error) {
	runner.calls = append(runner.calls, command)
	return runner.result, runner.err
}

func TestForgejoDeletesUserWithoutPurgingAuthoritativeRepositories(t *testing.T) {
	runner := &forgejoRunner{}
	err := (Forgejo{Runner: runner}).DeleteUser(context.Background(), "alice")
	require.NoError(t, err)
	require.Len(t, runner.calls, 1)
	require.Equal(t, "/usr/sbin/runuser", runner.calls[0].Name)
	require.Equal(t, []string{
		"--user", "git", "--", "/usr/bin/forgejo", "admin", "user", "delete",
		"--config", "/etc/forgejo/app.ini", "--username", "alice",
	}, runner.calls[0].Args)
	require.NotContains(t, runner.calls[0].Args, "--purge")
}

func TestForgejoRecognizesAnAlreadyRemovedUser(t *testing.T) {
	runner := &forgejoRunner{result: linuxhost.CommandResult{ExitCode: 1, Stderr: "user does not exist [name: alice]"}}
	err := (Forgejo{Runner: runner}).DeleteUser(context.Background(), "alice")
	require.ErrorIs(t, err, ErrForgejoUserNotFound)
}

func TestForgejoReportsNativeDeletionFailure(t *testing.T) {
	runner := &forgejoRunner{err: errors.New("exec unavailable")}
	err := (Forgejo{Runner: runner}).DeleteUser(context.Background(), "alice")
	require.ErrorContains(t, err, "exec unavailable")
}
