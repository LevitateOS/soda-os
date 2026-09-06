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
	calls  []string
	ctx    context.Context
	output io.Writer
	err    error
}

func (fake *recordingUpdates) Status(ctx context.Context) (updates.Host, error) {
	fake.calls = append(fake.calls, "status")
	fake.ctx = ctx
	return updates.Host{Kind: "BootcHost"}, fake.err
}

func (fake *recordingUpdates) Check(ctx context.Context) (updates.Host, error) {
	err := fake.progress(ctx, "check")
	return updates.Host{Kind: "BootcHost"}, err
}

func (fake *recordingUpdates) Update(ctx context.Context) error {
	return fake.progress(ctx, "update")
}

func (fake *recordingUpdates) progress(ctx context.Context, action string) error {
	fake.calls = append(fake.calls, action)
	fake.ctx = ctx
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
		{"status authorization", "x86_64", 1000, []string{"status"}, "administrative access"},
		{"check authorization", "aarch64", 1000, []string{"check"}, "administrative access"},
		{"update authorization", "x86_64", 1000, []string{"update"}, "administrative access"},
		{"architecture", "", 0, []string{"status"}, "requires x86_64 or aarch64"},
		{"old download", "x86_64", 0, []string{"download"}, "unknown command"},
		{"old apply", "aarch64", 0, []string{"apply"}, "unknown command"},
		{"version", "x86_64", 0, []string{"update", "--version", "0.6.3"}, "unknown flag"},
		{"reference", "x86_64", 0, []string{"update", "--reference", "arbitrary"}, "unknown flag"},
		{"positional", "aarch64", 0, []string{"update", "extra"}, "unknown command"},
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
	var names []string
	for _, child := range command.Commands() {
		require.NotEmpty(t, child.Short)
		names = append(names, child.Name())
	}
	require.ElementsMatch(t, []string{"status", "check", "update", "help"}, names)
}

func TestReadsWriteExactlyOneJSONValueSeparateFromCheckProgress(t *testing.T) {
	for _, action := range []string{"status", "check"} {
		t.Run(action, func(t *testing.T) {
			fake := &recordingUpdates{}
			var output, diagnostic bytes.Buffer
			command := newCommand("aarch64", 0, func(stdout, stderr io.Writer) updateOperations {
				fake.output = stdout
				require.Same(t, &diagnostic, stderr)
				return fake
			})
			command.SetOut(&output)
			command.SetErr(&diagnostic)
			command.SetArgs([]string{action})
			require.NoError(t, command.ExecuteContext(t.Context()))
			require.Equal(t, []string{action}, fake.calls)
			decoder := json.NewDecoder(&output)
			var document updates.Host
			require.NoError(t, decoder.Decode(&document))
			require.Equal(t, "BootcHost", document.Kind)
			require.ErrorIs(t, decoder.Decode(&document), io.EOF)
			if action == "check" {
				_, bounded := fake.ctx.Deadline()
				require.True(t, bounded)
				require.Equal(t, "native progress\n", diagnostic.String())
			} else {
				require.Equal(t, t.Context(), fake.ctx)
				require.Empty(t, diagnostic.String())
			}
		})
	}
}

func TestUpdateUsesSuppliedOperationContextAndStreams(t *testing.T) {
	fake := &recordingUpdates{}
	var output, diagnostic bytes.Buffer
	command := newCommand("x86_64", 0, func(stdout, stderr io.Writer) updateOperations {
		require.Same(t, &diagnostic, stderr)
		fake.output = stdout
		return fake
	})
	command.SetOut(&output)
	command.SetErr(&diagnostic)
	command.SetArgs([]string{"update"})
	require.NoError(t, command.ExecuteContext(t.Context()))
	require.Equal(t, []string{"update"}, fake.calls)
	require.Equal(t, t.Context(), fake.ctx)
	require.Equal(t, "native progress\n", output.String())
	fake.err = errors.New("native failure")
	require.ErrorIs(t, command.Execute(), fake.err)
	fake.err = nil
	command.SetOut(failingWriter{})
	require.ErrorIs(t, command.Execute(), io.ErrClosedPipe)
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestCheckProgressWriterFailureDoesNotEmitJSON(t *testing.T) {
	fake := &recordingUpdates{}
	command := newCommand("x86_64", 0, func(stdout, _ io.Writer) updateOperations {
		fake.output = stdout
		return fake
	})
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(failingWriter{})
	command.SetArgs([]string{"check"})
	require.ErrorIs(t, command.Execute(), io.ErrClosedPipe)
	require.Empty(t, output.String())
}

func TestReadFailuresDoNotEmitJSONOrDuplicateDiagnostics(t *testing.T) {
	failure := errors.New("native failure")
	for _, action := range []string{"status", "check"} {
		fake := &recordingUpdates{err: failure}
		command := newCommand("aarch64", 0, func(stdout, _ io.Writer) updateOperations {
			fake.output = stdout
			return fake
		})
		var output, diagnostic bytes.Buffer
		command.SetOut(&output)
		command.SetErr(&diagnostic)
		command.SetArgs([]string{action})
		require.ErrorIs(t, command.Execute(), failure)
		require.Empty(t, output.String())
		require.NotContains(t, diagnostic.String(), failure.Error())
		fake.err = nil
		command.SetOut(failingWriter{})
		require.ErrorIs(t, command.Execute(), io.ErrClosedPipe)
	}
}
