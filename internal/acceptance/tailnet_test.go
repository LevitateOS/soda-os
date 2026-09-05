package acceptance

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEnrollmentUsesKnownGuestAndOwnsLogoutBeforeEvidence(t *testing.T) {
	installAcceptanceCommand(t, "ssh", `printf '%s' '{"BackendState":"Running","Self":{"ID":"this-guest","HostName":"renamed-machine","TailscaleIPs":["100.64.0.9"]},"Peer":{"unrelated":{"HostName":"soda","TailscaleIPs":["100.64.0.8"]}}}'`)
	person := testPerson(t, "owner")
	installed := &guest{}
	host, evidence, err := awaitGuestEnrollment(context.Background(), installed, person)
	require.NoError(t, err)
	require.Equal(t, "100.64.0.9", host)
	require.NotContains(t, string(evidence), "unrelated")
	require.NotContains(t, string(evidence), "100.64.0.8")
	require.NotNil(t, installed.enrollment)
	require.Equal(t, person.Remote, installed.enrollment.remote)
	require.Equal(t, person.LinuxPassword, installed.enrollment.password)
}

func TestEnrollmentRetainsLogoutWhenAddressIsMissing(t *testing.T) {
	installAcceptanceCommand(t, "ssh", `printf '%s' '{"BackendState":"Running","Self":{"ID":"guest","TailscaleIPs":[]}}'`)
	installed := &guest{}
	_, _, err := awaitGuestEnrollment(context.Background(), installed, testPerson(t, "owner"))
	require.ErrorContains(t, err, "no usable Tailnet address")
	require.NotNil(t, installed.enrollment)
}

func TestUnenrolledGuestWaitIsCancellable(t *testing.T) {
	installAcceptanceCommand(t, "ssh", `printf '%s' '{"BackendState":"NeedsLogin"}'`)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	installed := &guest{}
	_, _, err := awaitGuestEnrollment(ctx, installed, testPerson(t, "owner"))
	require.Error(t, err)
	require.NotNil(t, installed.enrollment, "enrollment may finish between the last status read and cancellation")
}
