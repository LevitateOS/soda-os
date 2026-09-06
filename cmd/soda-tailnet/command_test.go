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

func TestEnrollmentMessage(t *testing.T) {
	for name, status := range map[string]tailnet.Status{
		"not signed in": {BackendState: "NeedsLogin"},
		"disconnected":  {BackendState: "Stopped"},
		"waiting for Tailnet administrator approval": {BackendState: "NeedsMachineAuth"},
		"waiting for browser authentication":         {BackendState: "NeedsLogin", AuthPending: true},
		"expired":                                    {BackendState: "Running", Expired: true},
	} {
		t.Run(name, func(t *testing.T) {
			message := enrollmentMessage(status, nil)
			require.Contains(t, message, name)
			require.Contains(t, message, "Cockpit → Tailscale")
			require.NotContains(t, message, "Soda Setup")
		})
	}
	require.Contains(t, enrollmentMessage(tailnet.Status{}, errors.New("unavailable")), "status is unavailable")
}

func TestConnectedMessageIncludesBothServiceURLs(t *testing.T) {
	status := tailnet.Status{BackendState: "Running", Identity: "atlas.example.ts.net", IPv4: "100.64.0.1", MagicDNSEnabled: true}
	message := enrollmentMessage(status, nil)
	require.Contains(t, message, "Tailnet identity: atlas.example.ts.net")
	require.Contains(t, message, "Cockpit: https://atlas.example.ts.net:9090")
	require.Contains(t, message, "Forgejo: http://atlas.example.ts.net:30000/")
	status.MagicDNSEnabled = false
	message = enrollmentMessage(status, nil)
	require.Contains(t, message, "Cockpit: https://100.64.0.1:9090")
	require.Contains(t, message, "Forgejo: http://100.64.0.1:30000/")
}

func TestExecuteKeepsUnavailableStatusNonFatal(t *testing.T) {
	var output bytes.Buffer
	var received context.Context
	err := execute(t.Context(), &output, func(ctx context.Context) (tailnet.Status, error) {
		received = ctx
		_, bounded := ctx.Deadline()
		require.True(t, bounded)
		return tailnet.Status{}, errors.New("unavailable")
	})
	require.NoError(t, err)
	require.Contains(t, output.String(), "Tailscale status is unavailable")
	require.ErrorIs(t, received.Err(), context.Canceled)
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestExecuteReportsWriteFailure(t *testing.T) {
	err := execute(t.Context(), failingWriter{}, func(context.Context) (tailnet.Status, error) {
		return tailnet.Status{BackendState: "Running"}, nil
	})
	require.ErrorIs(t, err, io.ErrClosedPipe)
}

func TestExecutePreservesParentCancellationAsGuidance(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	var output bytes.Buffer
	err := execute(ctx, &output, func(ctx context.Context) (tailnet.Status, error) {
		require.ErrorIs(t, ctx.Err(), context.Canceled)
		return tailnet.Status{}, ctx.Err()
	})
	require.NoError(t, err)
	require.Contains(t, output.String(), "status is unavailable")
}
