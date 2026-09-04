package linuxhost

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type passwordRunner struct {
	result CommandResult
	calls  []Command
}

func (runner *passwordRunner) Run(_ context.Context, command Command) (CommandResult, error) {
	runner.calls = append(runner.calls, command)
	return runner.result, nil
}

func TestNativeReportsLinuxPasswordStatus(t *testing.T) {
	for _, test := range []struct {
		field string
		want  PasswordStatus
	}{
		{field: "L", want: PasswordLocked},
		{field: "LK", want: PasswordLocked},
		{field: "P", want: PasswordSet},
		{field: "PS", want: PasswordSet},
		{field: "NP", want: PasswordUnset},
	} {
		t.Run(test.field, func(t *testing.T) {
			runner := &passwordRunner{result: CommandResult{Stdout: "alice " + test.field + " 2026-09-01 0 99999 7 -1\n"}}
			native := NewNative()
			native.Runner = runner
			status, err := native.PasswordStatus(context.Background(), Account{Username: "alice"})
			require.NoError(t, err)
			require.Equal(t, test.want, status)
			require.Equal(t, []string{"--status", "alice"}, runner.calls[0].Args)
		})
	}
}

func TestNativeRejectsUnknownOrMismatchedPasswordStatus(t *testing.T) {
	for name, result := range map[string]CommandResult{
		"unknown status":  {Stdout: "alice XX 2026-09-01\n"},
		"different user":  {Stdout: "bob P 2026-09-01\n"},
		"command failure": {ExitCode: 1, Stderr: "shadow unavailable"},
	} {
		t.Run(name, func(t *testing.T) {
			runner := &passwordRunner{result: result}
			native := NewNative()
			native.Runner = runner
			_, err := native.PasswordStatus(context.Background(), Account{Username: "alice"})
			require.Error(t, err)
		})
	}
}
