package main

import (
	"bytes"
	"errors"
	"io"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCommandExposesOnlyFixedPublicationOperations(t *testing.T) {
	command := newCommand(func(string, io.Writer, io.Writer) (publicationOperations, error) {
		t.Fatal("construction and help must not load publication inputs")
		return nil, nil
	})
	names := make([]string, 0, len(command.Commands()))
	for _, child := range command.Commands() {
		names = append(names, child.Name())
	}
	sort.Strings(names)
	require.Equal(t, []string{"draft", "image-promote", "image-stage", "publish", "record-sign", "upload"}, names)
	for _, forbidden := range []string{"github-token-env", "repository", "asset", "clobber", "output-dir"} {
		require.Nil(t, command.PersistentFlags().Lookup(forbidden))
	}
	command.SetOut(io.Discard)
	command.SetArgs([]string{"--help"})
	require.NoError(t, command.Execute())
	require.True(t, command.CompletionOptions.DisableDefaultCmd)
}

func TestCommandsValidateEveryRequiredFlagBeforeConnecting(t *testing.T) {
	for _, test := range publicationCases() {
		t.Run(test.action, func(t *testing.T) {
			for index := 0; index < len(test.flags); index += 2 {
				command := newCommand(func(string, io.Writer, io.Writer) (publicationOperations, error) {
					t.Fatal("missing required flag must not connect")
					return nil, nil
				})
				args := append([]string{test.action}, test.flags[:index]...)
				command.SetArgs(append(args, test.flags[index+2:]...))
				require.ErrorContains(t, command.Execute(), "required flag")
			}
		})
	}
}

func TestPublicationDispatchUsesSuppliedInputsAndStreams(t *testing.T) {
	for _, test := range publicationCases() {
		t.Run(test.action, func(t *testing.T) {
			fake := &recordingPublication{}
			var output, diagnostic bytes.Buffer
			command := newCommand(func(spec string, stdout, stderr io.Writer) (publicationOperations, error) {
				require.Equal(t, "chosen.toml", spec)
				require.Same(t, &output, stdout)
				require.Same(t, &diagnostic, stderr)
				return fake, nil
			})
			command.SetOut(&output)
			command.SetErr(&diagnostic)
			command.SetArgs(append([]string{"--spec", "chosen.toml", test.action}, test.flags...))
			require.NoError(t, command.ExecuteContext(t.Context()))
			require.Equal(t, []string{test.action}, fake.calls)
			require.Equal(t, test.options, fake.options)
			require.Equal(t, t.Context(), fake.ctx)
			require.Contains(t, output.String(), test.message)
			require.Empty(t, diagnostic.String())
		})
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestPublicationFailuresAndWriterErrors(t *testing.T) {
	failure := errors.New("publication failed")
	for _, test := range publicationCases() {
		fake := &recordingPublication{err: failure}
		command := newCommand(func(string, io.Writer, io.Writer) (publicationOperations, error) { return fake, nil })
		var output bytes.Buffer
		command.SetOut(&output)
		command.SetArgs(append([]string{test.action}, test.flags...))
		require.ErrorIs(t, command.Execute(), failure)
		require.Empty(t, output.String())
		fake.err, fake.calls = nil, nil
		command.SetOut(failingWriter{})
		require.ErrorIs(t, command.Execute(), io.ErrClosedPipe)
		require.Equal(t, []string{test.action}, fake.calls, "output failure must not retry publication")
	}
}

func TestPublicationFactoryFailureDoesNotExecute(t *testing.T) {
	failure := errors.New("invalid specification")
	for _, test := range publicationCases() {
		command := newCommand(func(string, io.Writer, io.Writer) (publicationOperations, error) { return nil, failure })
		command.SetArgs(append([]string{test.action}, test.flags...))
		require.ErrorIs(t, command.Execute(), failure)
	}
}
