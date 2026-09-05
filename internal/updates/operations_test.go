package updates

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/LevitateOS/soda-os/internal/process"
	"github.com/stretchr/testify/require"
)

type nativeRunner struct {
	commands []process.Command
	hosts    []Host
	failure  string
}

func (r *nativeRunner) Run(_ context.Context, c process.Command) error {
	r.commands = append(r.commands, c)
	if c.Name == r.failure || c.Args[len(c.Args)-1] == r.failure {
		return errors.New("native failure")
	}
	return nil
}
func (r *nativeRunner) Output(ctx context.Context, c process.Command) (string, error) {
	if err := r.Run(ctx, c); err != nil {
		return "", err
	}
	if len(r.hosts) == 0 {
		return "", errors.New("unexpected status read")
	}
	host := r.hosts[0]
	r.hosts = r.hosts[1:]
	data, err := json.Marshal(host)
	return string(data), err
}

type publishedFixture struct {
	selected Release
	err      error
}

func (p publishedFixture) Published(context.Context, string, string) (Release, error) {
	return p.selected, p.err
}

func deployment(version, reference string, downloadOnly bool) *Deployment {
	return &Deployment{DownloadOnly: downloadOnly, Image: &ImageStatus{Version: version, ImageDigest: reference[len(repository)+1:], Architecture: "amd64", Image: ImageReference{Image: reference, Transport: "registry"}}}
}
func hostFixture(staged *Deployment) Host {
	return Host{APIVersion: "org.containers.bootc/v1", Kind: "BootcHost", Status: HostStatus{Booted: deployment("0.6.3", repository+"@"+testDigest, false), Staged: staged}}
}
func operationsFixture(hosts ...Host) (Operations, *nativeRunner, Selection) {
	selected := Release{Version: testVersion, Reference: repository + "@" + testDigest, Architecture: "x86_64"}
	runner := &nativeRunner{hosts: hosts}
	return Operations{Runner: runner, Releases: publishedFixture{selected: selected}, Architecture: "x86_64"}, runner, Selection{Version: selected.Version, Reference: selected.Reference}
}

func TestDownloadReverifiesAndStagesExactImageWithoutRestart(t *testing.T) {
	pending := deployment(testVersion, repository+"@"+testDigest, true)
	ops, runner, selection := operationsFixture(hostFixture(nil), hostFixture(pending))
	require.NoError(t, ops.Download(context.Background(), selection))
	require.Len(t, runner.commands, 3)
	require.Equal(t, []string{"switch", "--download-only", selection.Reference}, runner.commands[1].Args)
	require.Equal(t, "/usr/bin/bootc", runner.commands[1].Name)
}
func TestApplyChecksTargetBeforeAndAfterUnlockThenRestarts(t *testing.T) {
	pending := deployment(testVersion, repository+"@"+testDigest, true)
	enabled := deployment(testVersion, repository+"@"+testDigest, false)
	ops, runner, selection := operationsFixture(hostFixture(pending), hostFixture(enabled))
	require.NoError(t, ops.Apply(context.Background(), selection))
	require.Len(t, runner.commands, 4)
	require.Equal(t, []string{"switch", "--from-downloaded"}, runner.commands[1].Args)
	require.Equal(t, "/usr/bin/systemctl", runner.commands[3].Name)
	require.Equal(t, []string{"reboot"}, runner.commands[3].Args)
}
func TestApplyNeverRestartsChangedOrStillLockedTarget(t *testing.T) {
	pending := deployment(testVersion, repository+"@"+testDigest, true)
	changed := deployment("0.9.0", repository+"@"+testDigest, false)
	for _, after := range []*Deployment{changed, pending, nil} {
		ops, runner, selection := operationsFixture(hostFixture(pending), hostFixture(after))
		err := ops.Apply(context.Background(), selection)
		require.ErrorContains(t, err, "restart was NOT requested")
		require.Len(t, runner.commands, 3)
	}
}
func TestApplyRefusesStaleSelectionBeforeMutation(t *testing.T) {
	ops, runner, selection := operationsFixture(hostFixture(deployment("0.9.0", repository+"@"+testDigest, true)))
	require.Error(t, ops.Apply(context.Background(), selection))
	require.Len(t, runner.commands, 1)
}
func TestDownloadRefusesExistingDeploymentAndDowngrade(t *testing.T) {
	for _, host := range []Host{hostFixture(deployment(testVersion, repository+"@"+testDigest, true)), hostFixture(nil)} {
		host.Status.Booted.Image.Version = "0.9.0"
		ops, runner, selection := operationsFixture(host)
		require.Error(t, ops.Download(context.Background(), selection))
		require.Len(t, runner.commands, 1)
	}
}
func TestOperationsRefuseUnverifiedSelectionWithoutBootc(t *testing.T) {
	ops, runner, selection := operationsFixture()
	ops.Releases = publishedFixture{err: errors.New("signature invalid")}
	require.ErrorContains(t, ops.Download(context.Background(), selection), "signature invalid")
	require.ErrorContains(t, ops.Apply(context.Background(), selection), "signature invalid")
	require.Empty(t, runner.commands)
}
func TestApplyReportsRestartFailureWithoutRollback(t *testing.T) {
	pending := deployment(testVersion, repository+"@"+testDigest, false)
	ops, runner, selection := operationsFixture(hostFixture(pending), hostFixture(pending))
	runner.failure = "/usr/bin/systemctl"
	require.ErrorContains(t, ops.Apply(context.Background(), selection), "enabled for next restart, but restart request failed")
	require.Len(t, runner.commands, 4)
}
func TestApplyStopsOnUnlockFailure(t *testing.T) {
	ops, runner, selection := operationsFixture(hostFixture(deployment(testVersion, repository+"@"+testDigest, true)))
	runner.failure = "--from-downloaded"
	require.ErrorContains(t, ops.Apply(context.Background(), selection), "native failure")
	require.Len(t, runner.commands, 2)
}
func TestMutationRefusesOverlayAndQueuedRollback(t *testing.T) {
	for _, status := range []HostStatus{
		{Booted: deployment("0.6.3", repository+"@"+testDigest, false), RollbackQueued: true},
		{Booted: deployment("0.6.3", repository+"@"+testDigest, false), UsrOverlay: json.RawMessage(`true`)},
	} {
		host := hostFixture(nil)
		host.Status = status
		ops, runner, selection := operationsFixture(host)
		require.Error(t, ops.Download(context.Background(), selection))
		require.Len(t, runner.commands, 1)
	}
}
func TestStatusRefusesNonBootcHost(t *testing.T) {
	runner := &nativeRunner{hosts: []Host{{}}}
	_, err := ReadHost(context.Background(), runner)
	require.ErrorContains(t, err, "installed bootc system")
}
