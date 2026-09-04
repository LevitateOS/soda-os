package main

import (
	"errors"
	"testing"

	"github.com/LevitateOS/soda-os/internal/tailnet"
	"github.com/stretchr/testify/require"
)

func TestEnrollmentMessage(t *testing.T) {
	for name, test := range map[string]struct {
		status tailnet.Status
		err    error
		want   string
		absent string
	}{
		"not enrolled": {
			status: tailnet.Status{BackendState: "NeedsLogin"},
			want:   "Tailscale is not enrolled. Tailnet access is unavailable.\nInfrastructure owner: run `sudo tailscale up`, then open the one-time URL it prints to authorize this appliance. After authorization, run `sudo /usr/libexec/soda/forgejo-init refresh-tailnet`",
			absent: "soda-cockpit",
		},
		"identity ready": {
			status: tailnet.Status{BackendState: "Running", Identity: "atlas.example.ts.net"},
			want:   "Tailscale is connected.\nMagicDNS identity: atlas.example.ts.net\nOpen the Soda OS dashboard:\n  https://atlas.example.ts.net:9090",
		},
		"status unavailable": {
			err:  errors.New("daemon unavailable"),
			want: "Tailscale status is unavailable. Check: sudo systemctl status tailscaled",
		},
	} {
		t.Run(name, func(t *testing.T) {
			message := enrollmentMessage(test.status, test.err)
			require.Contains(t, message, test.want)
			if test.absent != "" {
				require.NotContains(t, message, test.absent)
			}
		})
	}
}
