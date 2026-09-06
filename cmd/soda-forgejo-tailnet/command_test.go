package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/LevitateOS/soda-os/internal/tailnet"
	"github.com/stretchr/testify/require"
)

func TestExecuteWritesEndpointAndReleasesDeadline(t *testing.T) {
	var output bytes.Buffer
	var received context.Context
	err := execute(t.Context(), &output, func(ctx context.Context) (tailnet.Endpoint, error) {
		received = ctx
		_, bounded := ctx.Deadline()
		require.True(t, bounded)
		return tailnet.Endpoint{Identity: "soda.example.ts.net", IPv4: "100.64.0.1"}, nil
	})
	require.NoError(t, err)
	require.Equal(t, "soda.example.ts.net 100.64.0.1\n", output.String())
	require.ErrorIs(t, received.Err(), context.Canceled)
}

func TestExecutePropagatesLookupFailureWithoutOutput(t *testing.T) {
	failure := errors.New("not enrolled")
	var output bytes.Buffer
	err := execute(t.Context(), &output, func(context.Context) (tailnet.Endpoint, error) {
		return tailnet.Endpoint{}, failure
	})
	require.ErrorIs(t, err, failure)
	require.Empty(t, output.String())
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestExecutePropagatesWriteFailure(t *testing.T) {
	err := execute(t.Context(), failingWriter{}, func(context.Context) (tailnet.Endpoint, error) {
		return tailnet.Endpoint{Identity: "host", IPv4: "100.64.0.1"}, nil
	})
	require.ErrorIs(t, err, io.ErrClosedPipe)
}

func TestExecutePreservesParentCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err := execute(ctx, io.Discard, func(ctx context.Context) (tailnet.Endpoint, error) {
		return tailnet.Endpoint{}, ctx.Err()
	})
	require.ErrorIs(t, err, context.Canceled)
}
