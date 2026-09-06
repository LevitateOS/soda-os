package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVerifyPassesIdentityContextAndStreams(t *testing.T) {
	failure := errors.New("signature rejected")
	var output, diagnostic bytes.Buffer
	calls := 0
	command := newCommand(nil, nil, func(ctx context.Context, path, revision string, stdout, stderr io.Writer) error {
		calls++
		require.Equal(t, t.Context(), ctx)
		require.Equal(t, "record.json", path)
		require.Equal(t, "revision", revision)
		require.Same(t, &output, stdout)
		require.Same(t, &diagnostic, stderr)
		return failure
	})
	command.SetArgs([]string{"verify", "--record", "record.json", "--expected-revision", "revision"})
	command.SetOut(&output)
	command.SetErr(&diagnostic)
	require.ErrorIs(t, command.ExecuteContext(t.Context()), failure)
	require.Equal(t, 1, calls)
	require.Empty(t, output.String())
	require.Empty(t, diagnostic.String())
}
