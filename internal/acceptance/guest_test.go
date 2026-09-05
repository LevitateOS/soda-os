package acceptance

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGuestFallbackKeepsOneOwnerAndEnrollmentAcrossReplacements(t *testing.T) {
	evidence := prepareGuestCommands(t)
	cleanup := &Cleanup{}
	guest := enrolledTestGuest(t, evidence, cleanup)
	enrollment, disk := guest.enrollment, guest.config.Disk
	require.NoError(t, guest.restart(context.Background(), "fallback/boot-fallback"))
	require.NoError(t, guest.restart(context.Background(), "fallback/boot-candidate"))
	require.Same(t, enrollment, guest.enrollment)
	require.Equal(t, disk, guest.vm.Config.Disk)
	require.Equal(t, "installed", guest.vm.Config.Mode)
	require.Empty(t, guest.vm.Config.ISO)
	require.Len(t, cleanup.actions, 1)
	require.NotContains(t, guestEvents(t), "logout")
	require.NoError(t, guest.captureQMP(context.Background(), "final-qmp.json"))
	require.NoError(t, guest.shutdown(context.Background()))
	require.NoError(t, cleanup.Run(context.Background()))
	require.NoError(t, guest.cleanup(context.Background()))
	require.Equal(t, []string{"start:iso", "powerdown:iso", "start:boot-fallback", "powerdown:boot-fallback", "start:boot-candidate", "logout", "powerdown:boot-candidate"}, guestEvents(t))
}

func TestGuestFailedReplacementRetainsLogoutObligation(t *testing.T) {
	evidence := prepareGuestCommands(t)
	cleanup := &Cleanup{}
	guest := enrolledTestGuest(t, evidence, cleanup)
	previous := guest.vm
	t.Setenv("SODA_QEMU", "nonexistent-acceptance-qemu")
	require.Error(t, guest.restart(context.Background(), "fallback/failed"))
	require.Nil(t, guest.vm)
	require.NoError(t, previous.Process.Wait(context.Background()))
	require.False(t, guest.enrollment.attempted)
	// A stopped guest cannot revoke its enrollment. Report that failure, not a pass.
	t.Setenv("FAIL_GUEST_LOGOUT", "23")
	err := cleanup.Run(context.Background())
	require.ErrorContains(t, err, "exit status 23")
	require.ErrorContains(t, guest.cleanup(context.Background()), "exit status 23")
	require.Equal(t, []string{"start:iso", "powerdown:iso", "logout"}, guestEvents(t))
}

func TestGuestFailedPowerdownKeepsExactProcessForEmergencyStop(t *testing.T) {
	evidence := prepareGuestCommands(t)
	cleanup := &Cleanup{}
	guest := enrolledTestGuest(t, evidence, cleanup)
	previous := guest.vm
	guest.vm.QMP.Dial = errorQMPDialer
	require.ErrorContains(t, guest.restart(context.Background(), "fallback/not-launched"), "rejected")
	require.Same(t, previous, guest.vm)
	require.NoError(t, cleanup.Run(context.Background()))
	require.Equal(t, []string{"start:iso", "logout", "stop:iso"}, guestEvents(t))
}

func TestGuestLogoutFailureStillPowersDownAndRemainsAnError(t *testing.T) {
	evidence := prepareGuestCommands(t)
	cleanup := &Cleanup{}
	guest := enrolledTestGuest(t, evidence, cleanup)
	t.Setenv("FAIL_GUEST_LOGOUT", "23")
	require.ErrorContains(t, guest.shutdown(context.Background()), "exit status 23")
	require.Nil(t, guest.vm)
	require.ErrorContains(t, cleanup.Run(context.Background()), "exit status 23")
	require.ErrorContains(t, guest.cleanup(context.Background()), "exit status 23")
	require.Equal(t, []string{"start:iso", "logout", "powerdown:iso"}, guestEvents(t))
}

func TestGuestCleanupIgnoresCancelledItineraryAndStopsAfterLogoutFailure(t *testing.T) {
	evidence := prepareGuestCommands(t)
	cleanup := &Cleanup{}
	guest := enrolledTestGuest(t, evidence, cleanup)
	t.Setenv("FAIL_GUEST_LOGOUT", "23")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorContains(t, cleanup.Run(ctx), "exit status 23")
	require.ErrorContains(t, guest.cleanup(ctx), "exit status 23")
	require.Equal(t, []string{"start:iso", "logout", "stop:iso"}, guestEvents(t))
}

func TestGuestExpiredLogoutDoesNotSkipEmergencyStop(t *testing.T) {
	evidence := prepareGuestCommands(t)
	guest := enrolledTestGuest(t, evidence, &Cleanup{})
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	require.ErrorIs(t, guest.logout(ctx), context.DeadlineExceeded)
	require.ErrorIs(t, guest.cleanup(ctx), context.DeadlineExceeded)
	require.Nil(t, guest.vm)
	require.Equal(t, []string{"start:iso", "stop:iso"}, guestEvents(t))
}

func TestQCOW2ShutdownDoesNotConsumeISOEnrollment(t *testing.T) {
	evidence := prepareGuestCommands(t)
	cleanup := &Cleanup{}
	iso := enrolledTestGuest(t, evidence, cleanup)
	qcow, err := launchGuest(context.Background(), guestTestConfig(t, evidence, "qcow2"), evidence, cleanup)
	require.NoError(t, err)
	t.Cleanup(func() { _ = qcow.cleanup(context.Background()) })
	require.NoError(t, qcow.shutdown(context.Background()))
	require.False(t, iso.enrollment.attempted)
	require.NoError(t, cleanup.Run(context.Background()))
	require.Len(t, cleanup.actions, 2)
	require.Equal(t, []string{"start:iso", "start:qcow2", "powerdown:qcow2", "logout", "stop:iso"}, guestEvents(t))
}

func TestGuestCleanupRegistrationFailureStopsItsLaunchedProcess(t *testing.T) {
	evidence := prepareGuestCommands(t)
	cleanup := &Cleanup{}
	require.NoError(t, cleanup.Run(context.Background()))
	guest, err := launchGuest(context.Background(), guestTestConfig(t, evidence, "unregistered"), evidence, cleanup)
	require.ErrorContains(t, err, "cleanup already ran")
	require.Nil(t, guest)
	require.Equal(t, []string{"start:unregistered", "stop:unregistered"}, guestEvents(t))
}
