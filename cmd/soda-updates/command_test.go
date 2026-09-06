package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/LevitateOS/soda-os/internal/updates"
	"github.com/stretchr/testify/require"
)

type recordingUpdates struct {
	calls     []string
	ctx       context.Context
	selection updates.Selection
	output    io.Writer
	err       error
}

func (fake *recordingUpdates) Status(ctx context.Context) (updates.Host, error) {
	fake.calls = append(fake.calls, "status")
	fake.ctx = ctx
	return updates.Host{Status: updates.HostStatus{Booted: &updates.Deployment{Image: &updates.ImageStatus{Version: "0.6.3"}}}}, fake.err
}

func (fake *recordingUpdates) Check(ctx context.Context) (updates.Release, error) {
	fake.calls = append(fake.calls, "check")
	fake.ctx = ctx
	return updates.Release{Version: "0.6.4"}, fake.err
}

func (fake *recordingUpdates) Download(ctx context.Context, selection updates.Selection) error {
	return fake.progress(ctx, selection, "download")
}

func (fake *recordingUpdates) Apply(ctx context.Context, selection updates.Selection) error {
	return fake.progress(ctx, selection, "apply")
}

func (fake *recordingUpdates) progress(ctx context.Context, selection updates.Selection, action string) error {
	fake.calls = append(fake.calls, action)
	fake.ctx, fake.selection = ctx, selection
	_, err := io.WriteString(fake.output, "native progress\n")
	return errors.Join(fake.err, err)
}

func TestCommandValidationDoesNotConnect(t *testing.T) {
	for _, test := range []struct {
		name         string
		architecture string
		euid         int
		args         []string
		message      string
	}{
		{"administrator", "x86_64", 1000, []string{"status"}, "administrative access"},
		{"architecture", "", 0, []string{"status"}, "requires x86_64 or aarch64"},
		{"download selection", "x86_64", 0, []string{"download"}, "required flag"},
		{"apply selection", "aarch64", 0, []string{"apply"}, "required flag"},
		{"positional", "aarch64", 0, []string{"status", "extra"}, "unknown command"},
	} {
		t.Run(test.name, func(t *testing.T) {
			command := newCommand(test.architecture, test.euid, func(io.Writer, io.Writer) updateOperations {
				t.Fatal("validation must not construct native operations")
				return nil
			})
			command.SetArgs(test.args)
			require.ErrorContains(t, command.Execute(), test.message)
		})
	}
}

func TestCommandHelpDoesNotConnect(t *testing.T) {
	command := newCommand("", 1000, func(io.Writer, io.Writer) updateOperations {
		t.Fatal("help must not construct native operations")
		return nil
	})
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetArgs([]string{"--help"})
	require.NoError(t, command.Execute())
	for _, child := range command.Commands() {
		require.NotEmpty(t, child.Short)
		require.NotEqual(t, "completion", child.Name())
	}
}

func TestReadsWriteExactlyOneJSONValue(t *testing.T) {
	for _, action := range []string{"status", "check"} {
		t.Run(action, func(t *testing.T) {
			fake := &recordingUpdates{}
			command := newCommand("aarch64", 0, func(io.Writer, io.Writer) updateOperations { return fake })
			var output, diagnostic bytes.Buffer
			command.SetOut(&output)
			command.SetErr(&diagnostic)
			command.SetArgs([]string{action})
			require.NoError(t, command.ExecuteContext(t.Context()))
			require.Equal(t, []string{action}, fake.calls)
			decoder := json.NewDecoder(&output)
			var document map[string]any
			require.NoError(t, decoder.Decode(&document))
			require.ErrorIs(t, decoder.Decode(&document), io.EOF)
			require.Empty(t, diagnostic.String())
			if action == "check" {
				_, bounded := fake.ctx.Deadline()
				require.True(t, bounded)
			} else {
				require.Equal(t, t.Context(), fake.ctx)
			}
		})
	}
}

func TestMutationsUseSuppliedOperationsAndStreams(t *testing.T) {
	for _, action := range []string{"download", "apply"} {
		t.Run(action, func(t *testing.T) {
			fake := &recordingUpdates{}
			var output, diagnostic bytes.Buffer
			command := newCommand("x86_64", 0, func(stdout, stderr io.Writer) updateOperations {
				require.Same(t, &diagnostic, stderr)
				fake.output = stdout
				return fake
			})
			command.SetOut(&output)
			command.SetErr(&diagnostic)
			command.SetArgs([]string{action, "--version", "0.6.4", "--reference", "confirmed-digest"})
			require.NoError(t, command.ExecuteContext(t.Context()))
			require.Equal(t, []string{action}, fake.calls)
			require.Equal(t, t.Context(), fake.ctx)
			require.Equal(t, updates.Selection{Version: "0.6.4", Reference: "confirmed-digest"}, fake.selection)
			require.Equal(t, "native progress\n", output.String())
		})
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestReadFailuresAreReturnedWithoutDiagnostics(t *testing.T) {
	failure := errors.New("native failure")
	for _, action := range []string{"status", "check"} {
		fake := &recordingUpdates{err: failure}
		command := newCommand("aarch64", 0, func(io.Writer, io.Writer) updateOperations { return fake })
		var output, diagnostic bytes.Buffer
		command.SetOut(&output)
		command.SetErr(&diagnostic)
		command.SetArgs([]string{action})
		require.ErrorIs(t, command.Execute(), failure)
		require.Empty(t, output.String())
		require.Empty(t, diagnostic.String())
		fake.err = nil
		command.SetOut(failingWriter{})
		require.ErrorIs(t, command.Execute(), io.ErrClosedPipe)
	}
}
