package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExecuteRejectsArgumentCountBeforeRequest(t *testing.T) {
	for _, args := range [][]string{nil, {"list", "extra"}} {
		err := execute(t.Context(), args, strings.NewReader("{}"), io.Discard, func(context.Context, string, io.Reader) (any, error) {
			t.Fatal("invalid arguments must not resolve identity or execute")
			return nil, nil
		})
		require.ErrorIs(t, err, errUsage)
	}
}

func TestExecuteUsesSuppliedRequestAndWritesOneJSONValue(t *testing.T) {
	var output bytes.Buffer
	input := strings.NewReader("{}\n")
	calls := 0
	err := execute(t.Context(), []string{"list"}, input, &output, func(ctx context.Context, action string, reader io.Reader) (any, error) {
		calls++
		require.Equal(t, t.Context(), ctx)
		require.Equal(t, "list", action)
		require.Same(t, input, reader)
		return map[string]string{"value": "<project>&repository"}, nil
	})
	require.NoError(t, err)
	require.Equal(t, 1, calls)
	require.True(t, strings.HasSuffix(output.String(), "\n"))
	decoder := json.NewDecoder(&output)
	var response map[string]string
	require.NoError(t, decoder.Decode(&response))
	require.Equal(t, "<project>&repository", response["value"])
	require.ErrorIs(t, decoder.Decode(&response), io.EOF)
}

func TestExecutePreservesRequestFailureWithoutOutput(t *testing.T) {
	failure := errors.New("identity or domain failure")
	var output bytes.Buffer
	err := execute(t.Context(), []string{"unknown"}, nil, &output, func(context.Context, string, io.Reader) (any, error) {
		return nil, failure
	})
	require.ErrorIs(t, err, failure)
	require.Empty(t, output.String())
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestExecuteReportsWriterAndEncodingFailures(t *testing.T) {
	for _, test := range []struct {
		response any
		output   io.Writer
	}{
		{map[string]bool{"ok": true}, failingWriter{}},
		{make(chan int), io.Discard},
	} {
		err := execute(t.Context(), []string{"list"}, nil, test.output, func(context.Context, string, io.Reader) (any, error) {
			return test.response, nil
		})
		require.ErrorContains(t, err, "encode result")
	}
}

func TestExecutePassesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err := execute(ctx, []string{"list"}, nil, io.Discard, func(received context.Context, _ string, _ io.Reader) (any, error) {
		return nil, received.Err()
	})
	require.ErrorIs(t, err, context.Canceled)
}

func TestMainUsageExit(t *testing.T) {
	if os.Getenv("SODA_TEST_MAIN_USAGE") == "1" {
		os.Args = []string{"command"}
		main()
		return
	}
	binary, err := os.Executable()
	require.NoError(t, err)
	child := exec.CommandContext(t.Context(), binary, "-test.run=^TestMainUsageExit$")
	child.Env = append(os.Environ(), "SODA_TEST_MAIN_USAGE=1")
	var output, diagnostic bytes.Buffer
	child.Stdout, child.Stderr = &output, &diagnostic
	var exit *exec.ExitError
	require.ErrorAs(t, child.Run(), &exit)
	require.Equal(t, 2, exit.ExitCode())
	require.Empty(t, output.String())
	require.Contains(t, diagnostic.String(), "usage:")
}
