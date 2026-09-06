package main

import (
	"io"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCommandInventoryAndHelpNeedNoHost(t *testing.T) {
	command := newCommand(nil, nil, nil)
	var names []string
	for _, child := range command.Commands() {
		names = append(names, child.Name())
	}
	sort.Strings(names)
	require.Equal(t, []string{"record", "run", "verify"}, names)
	command.SetOut(io.Discard)
	command.SetArgs([]string{"--help"})
	require.NoError(t, command.Execute())
	require.False(t, command.CompletionOptions.DisableDefaultCmd)
}

func TestEveryRequiredFlagIsValidatedBeforeExecution(t *testing.T) {
	for _, args := range [][]string{runArguments(), recordArguments(), {"verify", "--record", "record.json", "--expected-revision", "revision"}} {
		t.Run(args[0], func(t *testing.T) {
			for index := 1; index < len(args); index += 2 {
				command := newCommand(nil, nil, nil)
				without := append([]string(nil), args[:index]...)
				command.SetArgs(append(without, args[index+2:]...))
				require.ErrorContains(t, command.Execute(), "required flag")
			}
		})
	}
}

func TestUnknownAndPositionalArgumentsDoNotExecute(t *testing.T) {
	for _, args := range [][]string{{"unknown"}, {"run", "extra"}, {"record", "--obsolete"}, {"verify", "extra"}} {
		command := newCommand(nil, nil, nil)
		command.SetArgs(args)
		require.Error(t, command.Execute())
	}
}
