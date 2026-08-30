package tailnet

import (
	"context"
	"errors"
	"testing"

	"github.com/LevitateOS/soda-os/internal/process"
	"github.com/stretchr/testify/require"
)

type recordingRunner struct {
	output string
	err    error
	seen   []process.Command
}

func (r *recordingRunner) Run(context.Context, process.Command) error { return nil }

func (r *recordingRunner) Output(_ context.Context, command process.Command) (string, error) {
	r.seen = append(r.seen, command)
	return r.output, r.err
}

func TestClientReadsCanonicalMagicDNSIdentity(t *testing.T) {
	runner := &recordingRunner{output: `{"BackendState":"Running","Self":{"DNSName":"Atlas.Example.ts.net."}}`}
	client := New(Options{Runner: runner, CLI: "tailscale"})

	identity, err := client.Identity(context.Background())
	require.NoError(t, err)
	require.Equal(t, "atlas.example.ts.net", identity)
	require.Equal(t, []process.Command{{Name: "tailscale", Args: []string{"status", "--json"}}}, runner.seen)
}

func TestEnrollmentStateDoesNotPretendTailnetAccessIsAvailable(t *testing.T) {
	for name, test := range map[string]struct {
		status Status
		want   EnrollmentState
	}{
		"needs login":        {status: Status{BackendState: "NeedsLogin"}, want: NeedsEnrollment},
		"node is stopped":    {status: Status{BackendState: "Stopped"}, want: NeedsEnrollment},
		"identity is absent": {status: Status{BackendState: "Running"}, want: IdentityUnavailable},
		"identity is ready":  {status: Status{BackendState: "Running", Identity: "atlas.example.ts.net"}, want: Enrolled},
	} {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, test.want, test.status.EnrollmentState())
		})
	}
}

func TestIdentityRejectsUnenrolledAndIdentitylessNodes(t *testing.T) {
	for name, test := range map[string]struct {
		output string
		want   error
	}{
		"needs login":          {output: `{"BackendState":"NeedsLogin","Self":{}}`, want: ErrNotEnrolled},
		"identity unavailable": {output: `{"BackendState":"Running","Self":{}}`, want: ErrIdentityUnavailable},
	} {
		t.Run(name, func(t *testing.T) {
			client := New(Options{Runner: &recordingRunner{output: test.output}, CLI: "tailscale"})
			_, err := client.Identity(context.Background())
			require.ErrorIs(t, err, test.want)
		})
	}
}

func TestStatusRejectsInvalidMagicDNSIdentity(t *testing.T) {
	client := New(Options{Runner: &recordingRunner{output: `{"BackendState":"Running","Self":{"DNSName":"atlas.local"}}`}, CLI: "tailscale"})
	_, err := client.Status(context.Background())
	require.ErrorIs(t, err, ErrUnavailable)
	require.ErrorIs(t, err, ErrInvalidMagicDNSName)
}

func TestStatusReportsUnavailableCLI(t *testing.T) {
	client := New(Options{Runner: &recordingRunner{err: errors.New("daemon unavailable")}, CLI: "tailscale"})
	_, err := client.Status(context.Background())
	require.ErrorIs(t, err, ErrUnavailable)
}
