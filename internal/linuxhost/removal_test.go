package linuxhost

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

type processExitRunner struct {
	remaining int
}

func (runner *processExitRunner) Run(_ context.Context, request Command) (CommandResult, error) {
	if request.Name != "/usr/bin/pgrep" {
		return CommandResult{}, errors.New("unexpected command")
	}
	if runner.remaining > 0 {
		runner.remaining--
		return CommandResult{ExitCode: 0}, nil
	}
	return CommandResult{ExitCode: 1}, nil
}

type logindRunner struct {
	list      CommandResult
	terminate CommandResult
	calls     []Command
}

func (runner *logindRunner) Run(_ context.Context, request Command) (CommandResult, error) {
	runner.calls = append(runner.calls, request)
	if request.Name != "/usr/bin/loginctl" {
		return CommandResult{}, errors.New("unexpected command")
	}
	if len(request.Args) != 0 && request.Args[0] == "list-users" {
		return runner.list, nil
	}
	return runner.terminate, nil
}

type exactCommandRunner struct {
	results []CommandResult
	calls   []Command
}

func (runner *exactCommandRunner) Run(_ context.Context, request Command) (CommandResult, error) {
	runner.calls = append(runner.calls, request)
	if len(runner.results) == 0 {
		return CommandResult{}, nil
	}
	result := runner.results[0]
	runner.results = runner.results[1:]
	return result, nil
}

func TestNativeTerminatesOnlyConfirmedActiveLogindUser(t *testing.T) {
	inactive := &logindRunner{list: CommandResult{Stdout: "42 gdm no active\n"}}
	native := NewNative()
	native.Runner = inactive
	terminated, err := native.terminateLogindUser(context.Background(), Account{Username: "alice", UID: 1000})
	require.NoError(t, err)
	require.False(t, terminated)
	require.Len(t, inactive.calls, 1)
	require.Equal(t, []string{"list-users", "--no-legend", "--no-pager"}, inactive.calls[0].Args)

	active := &logindRunner{list: CommandResult{Stdout: "1000 alice yes active\n"}}
	native.Runner = active
	terminated, err = native.terminateLogindUser(context.Background(), Account{Username: "alice", UID: 1000})
	require.NoError(t, err)
	require.True(t, terminated)
	require.Len(t, active.calls, 2)
}

func TestNativeLogindDeletionFailsClosedOnMismatchAndCommandFailures(t *testing.T) {
	for _, record := range []string{"1001 alice yes active\n", "1000 bob yes active\n", "not-a-uid alice\n", "1000\n"} {
		_, err := logindUserIsActive(record, Account{Username: "alice", UID: 1000})
		require.Error(t, err, record)
	}

	for name, runner := range map[string]*logindRunner{
		"list failure": {list: CommandResult{ExitCode: 1, Stderr: "Failed to connect to bus"}},
		"termination failure": {
			list:      CommandResult{Stdout: "1000 alice yes active\n"},
			terminate: CommandResult{ExitCode: 1, Stderr: "Failed to terminate user"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			native := NewNative()
			native.Runner = runner
			_, err := native.terminateLogindUser(context.Background(), Account{Username: "alice", UID: 1000})
			require.Error(t, err)
		})
	}
}

func TestNativeWaitsForOwnedProcessesToExit(t *testing.T) {
	runner := &processExitRunner{remaining: 2}
	native := NewNative()
	native.Runner = runner

	require.NoError(t, native.verifyNoOwnedProcesses(context.Background(), Account{Username: "alice", UID: 1000}))
	require.Zero(t, runner.remaining)
}

func TestNativeResetsOnlyTheValidatedUserManager(t *testing.T) {
	runner := &exactCommandRunner{results: []CommandResult{{Stdout: "failed\n"}, {}}}
	native := NewNative()
	native.Runner = runner

	require.NoError(t, native.resetFailedUserManager(context.Background(), Account{UID: 1008}))
	require.Len(t, runner.calls, 2)
	require.Equal(t, []string{"show", "--property=ActiveState", "--value", "user@1008.service"}, runner.calls[0].Args)
	require.Equal(t, []string{"reset-failed", "user@1008.service"}, runner.calls[1].Args)
}

func TestNativeUserManagerResetHandlesUnloadAndRejectsUnknownState(t *testing.T) {
	t.Run("already inactive", func(t *testing.T) {
		runner := &exactCommandRunner{results: []CommandResult{{Stdout: "inactive\n"}}}
		native := NewNative()
		native.Runner = runner
		require.NoError(t, native.resetFailedUserManager(context.Background(), Account{UID: 1008}))
		require.Len(t, runner.calls, 1)
	})

	t.Run("unloaded during reset", func(t *testing.T) {
		runner := &exactCommandRunner{results: []CommandResult{
			{Stdout: "failed\n"},
			{ExitCode: 1, Stderr: "Unit user@1008.service not loaded."},
			{Stdout: "inactive\n"},
		}}
		native := NewNative()
		native.Runner = runner
		require.NoError(t, native.resetFailedUserManager(context.Background(), Account{UID: 1008}))
		require.Len(t, runner.calls, 3)
	})

	t.Run("unexpected state", func(t *testing.T) {
		runner := &exactCommandRunner{results: []CommandResult{{Stdout: "deactivating\n"}}}
		native := NewNative()
		native.Runner = runner
		err := native.resetFailedUserManager(context.Background(), Account{UID: 1008})
		require.ErrorContains(t, err, `unexpected active state "deactivating"`)
	})
}

func TestResetFailedUserManagerReportsNativeFailure(t *testing.T) {
	runner := &exactCommandRunner{results: []CommandResult{
		{Stdout: "failed\n"},
		{ExitCode: 1, Stderr: "unit failure"},
		{Stdout: "failed\n"},
	}}
	native := NewNative()
	native.Runner = runner

	err := native.resetFailedUserManager(context.Background(), Account{UID: 1008})
	require.ErrorContains(t, err, "reset user@1008.service failure state: unit failure")
}

func TestSameAccountRequiresEveryDeletionRelevantLinuxFact(t *testing.T) {
	expected := Account{
		Username: "alice", UID: 1000, GID: 1000, PrimaryGroup: "alice", GECOS: "Alice",
		Home: "/home/alice", Shell: "/bin/bash", Groups: map[string]bool{"alice": true},
	}
	require.True(t, sameAccount(expected, expected))

	changed := expected
	changed.UID++
	require.False(t, sameAccount(changed, expected))
	changed = expected
	changed.Groups = map[string]bool{"alice": true, "wheel": true}
	require.False(t, sameAccount(changed, expected))
}

func TestNativeDeleteAccountRevalidatesBeforeAndAfterProcessTermination(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "alice")
	require.NoError(t, os.Mkdir(home, 0o700))
	account := Account{
		Username: "alice", UID: os.Getuid(), GID: os.Getgid(), PrimaryGroup: "alice", GECOS: "Alice",
		Home: home, Shell: "/bin/bash", Groups: map[string]bool{"alice": true},
	}
	passwd := fmt.Sprintf("alice:x:%d:%d:Alice:%s:/bin/bash\n", account.UID, account.GID, home)
	runner := &identityRunner{results: map[string]CommandResult{
		commandKey("/usr/bin/getent", "passwd", "alice"):                                   {Stdout: passwd},
		commandKey("/usr/bin/id", "--name", "--groups", "alice"):                           {Stdout: "alice\n"},
		commandKey("/usr/bin/id", "--name", "--group", "alice"):                            {Stdout: "alice\n"},
		commandKey("/usr/bin/loginctl", "list-users", "--no-legend", "--no-pager"):         {},
		commandKey("/usr/bin/pkill", "--signal", "TERM", "--uid", fmt.Sprint(account.UID)): {},
		commandKey("/usr/bin/pkill", "--signal", "KILL", "--uid", fmt.Sprint(account.UID)): {},
		commandKey("/usr/bin/pgrep", "--uid", fmt.Sprint(account.UID)):                     {ExitCode: 1},
		commandKey("/usr/sbin/userdel", "--remove", "alice"):                               {},
	}}
	native := NewNative()
	native.Runner = runner
	native.HomeRoot = root

	require.NoError(t, native.DeleteAccount(context.Background(), account))
	var lookups int
	for _, call := range runner.calls {
		if commandKey(call.Name, call.Args...) == commandKey("/usr/bin/getent", "passwd", "alice") {
			lookups++
		}
	}
	require.Equal(t, 2, lookups, "account identity must be revalidated after process termination")
	require.Equal(t, "/usr/sbin/userdel", runner.calls[len(runner.calls)-1].Name)
	require.Equal(t, []string{"--remove", "alice"}, runner.calls[len(runner.calls)-1].Args)
}
