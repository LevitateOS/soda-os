package main

import (
	"io"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCommandInventoryAndHelpNeedNoHost(t *testing.T) {
	command := newCommand(nil)
	var names []string
	for _, child := range command.Commands() {
		names = append(names, child.Name())
	}
	sort.Strings(names)
	require.Equal(t, []string{"run"}, names)
	command.SetOut(io.Discard)
	command.SetArgs([]string{"--help"})
	require.NoError(t, command.Execute())
	require.False(t, command.CompletionOptions.DisableDefaultCmd)
}

func TestEveryRequiredFlagIsValidatedBeforeExecution(t *testing.T) {
	for _, args := range [][]string{runArguments()} {
		t.Run(args[0], func(t *testing.T) {
			for index := 1; index < len(args); index += 2 {
				command := newCommand(nil)
				without := append([]string(nil), args[:index]...)
				command.SetArgs(append(without, args[index+2:]...))
				require.ErrorContains(t, command.Execute(), "required flag")
			}
		})
	}
}

func TestUnknownAndPositionalArgumentsDoNotExecute(t *testing.T) {
	for _, args := range [][]string{{"unknown"}, {"run", "extra"}, {"run", "--candidate-record", "obsolete"}, {"run", "--fallback-record", "obsolete"}, {"record", "--obsolete"}, {"verify", "extra"}} {
		command := newCommand(nil)
		command.SetArgs(args)
		require.Error(t, command.Execute())
	}
}
