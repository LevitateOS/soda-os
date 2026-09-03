package projects

import (
	"context"
	"errors"
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

type exactCommandRunner struct {
	result CommandResult
	call   Command
}

func (runner *exactCommandRunner) Run(_ context.Context, request Command) (CommandResult, error) {
	runner.call = request
	return runner.result, nil
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

func TestTerminateLogindUserSkipsOnlyAConfirmedInactiveAccount(t *testing.T) {
	runner := &logindRunner{list: CommandResult{Stdout: "42 gdm no active\n"}}
	platform := &NativePlatform{Runner: runner}

	terminated, err := platform.terminateLogindUser(context.Background(), Account{Username: "alice", UID: 1000})
	require.NoError(t, err)
	require.False(t, terminated)
	require.Len(t, runner.calls, 1)
	require.Equal(t, []string{"list-users", "--no-legend", "--no-pager"}, runner.calls[0].Args)
}

func TestTerminateLogindUserFailsClosedOnNativeErrors(t *testing.T) {
	tests := map[string]*logindRunner{
		"list failure": {
			list: CommandResult{ExitCode: 1, Stderr: "Failed to connect to bus"},
		},
		"termination failure": {
			list:      CommandResult{Stdout: "1000 alice yes active\n"},
			terminate: CommandResult{ExitCode: 1, Stderr: "Failed to terminate user"},
		},
	}
	for name, runner := range tests {
		t.Run(name, func(t *testing.T) {
			platform := &NativePlatform{Runner: runner}
			_, err := platform.terminateLogindUser(context.Background(), Account{Username: "alice", UID: 1000})
			require.Error(t, err)
		})
	}
}

func TestTerminateLogindUserReportsAnActiveTermination(t *testing.T) {
	runner := &logindRunner{list: CommandResult{Stdout: "1000 alice yes active\n"}}
	platform := &NativePlatform{Runner: runner}

	terminated, err := platform.terminateLogindUser(context.Background(), Account{Username: "alice", UID: 1000})
	require.NoError(t, err)
	require.True(t, terminated)
}

func TestLogindUserRecordMustMatchBothUIDAndUsername(t *testing.T) {
	for _, record := range []string{"1001 alice yes active\n", "1000 bob yes active\n", "not-a-uid alice\n", "1000\n"} {
		_, err := logindUserIsActive(record, Account{Username: "alice", UID: 1000})
		require.Error(t, err, record)
	}
}

func TestProcessVerificationWaitsForKernelReaping(t *testing.T) {
	runner := &processExitRunner{remaining: 2}
	platform := &NativePlatform{Runner: runner}

	require.NoError(t, platform.verifyNoOwnedProcesses(context.Background(), Account{Username: "alice", UID: 1000}))
	require.Zero(t, runner.remaining)
}

func TestResetFailedUserManagerTargetsOnlyValidatedUID(t *testing.T) {
	runner := &exactCommandRunner{}
	platform := &NativePlatform{Runner: runner}

	require.NoError(t, platform.resetFailedUserManager(context.Background(), Account{UID: 1008}))
	require.Equal(t, "/usr/bin/systemctl", runner.call.Name)
	require.Equal(t, []string{"reset-failed", "user@1008.service"}, runner.call.Args)
}

func TestResetFailedUserManagerReportsNativeFailure(t *testing.T) {
	runner := &exactCommandRunner{result: CommandResult{ExitCode: 1, Stderr: "unit failure"}}
	platform := &NativePlatform{Runner: runner}

	err := platform.resetFailedUserManager(context.Background(), Account{UID: 1008})
	require.ErrorContains(t, err, "reset user@1008.service failure state: unit failure")
}
