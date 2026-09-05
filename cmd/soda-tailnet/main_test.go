package main

import (
	"errors"
	"github.com/LevitateOS/soda-os/internal/tailnet"
	"github.com/stretchr/testify/require"
	"testing"
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
