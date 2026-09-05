package main

import (
	"bytes"
	"context"
	"testing"

	"github.com/LevitateOS/soda-os/internal/process"
	"github.com/stretchr/testify/require"
)

type statusRunner struct{ calls int }

func (r *statusRunner) Run(context.Context, process.Command) error { r.calls++; return nil }
func (r *statusRunner) Output(_ context.Context, c process.Command) (string, error) {
	r.calls++
	return `{"apiVersion":"org.containers.bootc/v1","kind":"BootcHost","status":{"booted":{"image":{"version":"0.6.3"}}}}`, nil
}
func TestStatusRequiresAdministrator(t *testing.T) {
	runner := &statusRunner{}
	command := newCommand(runner, "x86_64", 1000)
	command.SetArgs([]string{"status"})
	require.ErrorContains(t, command.Execute(), "administrative access")
	require.Zero(t, runner.calls)
}
func TestStatusWritesOnlyJSON(t *testing.T) {
	runner := &statusRunner{}
	command := newCommand(runner, "x86_64", 0)
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetArgs([]string{"status"})
	require.NoError(t, command.Execute())
	require.Contains(t, output.String(), `"version":"0.6.3"`)
	require.Equal(t, 1, runner.calls)
}
func TestMutationRequiresExplicitSelection(t *testing.T) {
	for _, operation := range []string{"download", "apply"} {
		command := newCommand(&statusRunner{}, "x86_64", 0)
		command.SetArgs([]string{operation})
		require.ErrorContains(t, command.Execute(), "required flag")
	}
}
