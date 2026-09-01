package projects

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

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

func TestTerminateLogindUserSkipsOnlyAConfirmedInactiveAccount(t *testing.T) {
	runner := &logindRunner{list: CommandResult{Stdout: "42 gdm no active\n"}}
	platform := &NativePlatform{Runner: runner}

	require.NoError(t, platform.terminateLogindUser(context.Background(), Account{Username: "alice", UID: 1000}))
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
			err := platform.terminateLogindUser(context.Background(), Account{Username: "alice", UID: 1000})
			require.Error(t, err)
		})
	}
}

func TestLogindUserRecordMustMatchBothUIDAndUsername(t *testing.T) {
	for _, record := range []string{"1001 alice yes active\n", "1000 bob yes active\n", "not-a-uid alice\n", "1000\n"} {
		_, err := logindUserIsActive(record, Account{Username: "alice", UID: 1000})
		require.Error(t, err, record)
	}
}
