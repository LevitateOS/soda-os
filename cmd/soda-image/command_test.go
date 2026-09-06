package main

import (
	"bytes"
	"errors"
	"io"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCommandInventoryAndHelpRequireNoBuildInputs(t *testing.T) {
	command := newCommand(func(string, string, io.Writer, io.Writer) (imageOperations, error) {
		t.Fatal("construction and help must not load a builder")
		return nil, nil
	})
	var names []string
	for _, child := range command.Commands() {
		names = append(names, child.Name())
	}
	sort.Strings(names)
	require.Equal(t, []string{"check", "iso", "oci", "qcow2", "record", "rpm"}, names)
	command.SetOut(io.Discard)
	command.SetArgs([]string{"--help"})
	require.NoError(t, command.Execute())
	require.False(t, command.CompletionOptions.DisableDefaultCmd)
}

func TestEveryRequiredFlagIsValidatedBeforeBuilding(t *testing.T) {
	for _, test := range imageCases() {
		t.Run(test.action, func(t *testing.T) {
			args := append([]string{test.action, "--architecture", "aarch64"}, test.flags...)
			for index := 1; index < len(args); index += 2 {
				command := newCommand(nil)
				without := append([]string(nil), args[:index]...)
				command.SetArgs(append(without, args[index+2:]...))
				require.ErrorContains(t, command.Execute(), "required flag")
			}
		})
	}
}

func TestImageDispatchUsesSuppliedBuilders(t *testing.T) {
	for _, architecture := range []string{"aarch64", "x86_64"} {
		for _, test := range imageCases() {
			t.Run(architecture+"/"+test.action, func(t *testing.T) {
				fake := &recordingImage{}
				var output, diagnostic bytes.Buffer
				command := newCommand(func(spec, arch string, _, _ io.Writer) (imageOperations, error) {
					require.Equal(t, "chosen.toml", spec)
					require.Equal(t, architecture, arch)
					return fake, nil
				})
				command.SetOut(&output)
				command.SetErr(&diagnostic)
				command.SetArgs(append([]string{"--spec", "chosen.toml", "--architecture", architecture, test.action}, test.flags...))
				require.NoError(t, command.ExecuteContext(t.Context()))
				require.Equal(t, []string{test.action}, fake.calls)
				require.Equal(t, test.options, fake.options)
				require.Equal(t, t.Context(), fake.ctx)
				require.Contains(t, output.String(), test.message)
				require.Empty(t, diagnostic.String())
			})
		}
	}
}

func TestImageProgressStreamRouting(t *testing.T) {
	for _, test := range imageCases() {
		var output, diagnostic bytes.Buffer
		command := newCommand(func(_, _ string, stdout, stderr io.Writer) (imageOperations, error) {
			if test.action == "oci" {
				require.Same(t, &diagnostic, stdout)
			} else {
				require.Same(t, &output, stdout)
			}
			require.Same(t, &diagnostic, stderr)
			return &recordingImage{}, nil
		})
		command.SetOut(&output)
		command.SetErr(&diagnostic)
		command.SetArgs(append([]string{test.action, "--architecture", "x86_64"}, test.flags...))
		require.NoError(t, command.Execute())
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestImageFailuresAreReturnedWithoutSuccessOutput(t *testing.T) {
	failure := errors.New("build failed")
	for _, test := range imageCases() {
		fake := &recordingImage{err: failure}
		command := newCommand(func(string, string, io.Writer, io.Writer) (imageOperations, error) { return fake, nil })
		var output bytes.Buffer
		command.SetOut(&output)
		command.SetArgs(append([]string{test.action, "--architecture", "aarch64"}, test.flags...))
		require.ErrorIs(t, command.Execute(), failure)
		require.Empty(t, output.String())
		if test.message != "" {
			fake.err, fake.calls = nil, nil
			command.SetOut(failingWriter{})
			require.ErrorIs(t, command.Execute(), io.ErrClosedPipe)
			require.Equal(t, []string{test.action}, fake.calls)
		}
	}
}

func TestImageFactoryFailureAndInvalidArguments(t *testing.T) {
	failure := errors.New("invalid specification")
	for _, test := range imageCases() {
		command := newCommand(func(string, string, io.Writer, io.Writer) (imageOperations, error) { return nil, failure })
		command.SetArgs(append([]string{test.action, "--architecture", "aarch64"}, test.flags...))
		require.ErrorIs(t, command.Execute(), failure)
	}
	for _, args := range [][]string{{"installer-input"}, {"cloud-input"}, {"check", "extra"}, {"oci", "--unknown"}} {
		command := newCommand(nil)
		command.SetArgs(args)
		require.Error(t, command.Execute())
	}
}
