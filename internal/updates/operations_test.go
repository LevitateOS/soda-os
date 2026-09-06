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
	t        *testing.T
	commands []process.Command
	host     Host
	failure  error
	ctx      context.Context
}

func (r *nativeRunner) Run(ctx context.Context, command process.Command) error {
	r.commands = append(r.commands, command)
	r.ctx = ctx
	require.Equal(r.t, "/usr/bin/bootc", command.Name)
	require.Contains(r.t, [][]string{{"upgrade", "--check"}, {"upgrade", "--apply"}}, command.Args)
	return r.failure
}

func (r *nativeRunner) Output(ctx context.Context, command process.Command) (string, error) {
	r.commands = append(r.commands, command)
	r.ctx = ctx
	require.Equal(r.t, process.Command{Name: "/usr/bin/bootc", Args: []string{"status", "--json"}}, command)
	contents, err := json.Marshal(r.host)
	require.NoError(r.t, err)
	return string(contents), r.failure
}

func hostFixture() Host {
	version := "0.6.3"
	source := ImageReference{Image: "ghcr.io/levitateos/soda-os:dev-x86_64", Transport: "registry"}
	return Host{
		APIVersion: "org.containers.bootc/v1", Kind: "BootcHost", Spec: HostSpec{Image: &source},
		Status: HostStatus{UsrOverlay: json.RawMessage(`null`), Booted: &Deployment{Image: &ImageStatus{
			Version: &version, ImageDigest: "sha256:booted", Architecture: "amd64", Image: source,
		}}},
	}
}

func TestCheckUsesNativeMetadataThenJSONStatus(t *testing.T) {
	host := hostFixture()
	cached := *host.Status.Booted.Image
	cached.ImageDigest = "sha256:cached"
	host.Status.Booted.CachedUpdate = &cached
	queries, progress := &nativeRunner{t: t, host: host}, &nativeRunner{t: t}
	operations := Operations{Queries: queries, Runner: progress}
	actual, err := operations.Check(t.Context())
	require.NoError(t, err)
	require.Equal(t, host, actual)
	require.Equal(t, []process.Command{{Name: "/usr/bin/bootc", Args: []string{"upgrade", "--check"}}}, progress.commands)
	require.Len(t, queries.commands, 1)
	require.Same(t, t.Context(), progress.ctx)
}

func TestCheckCommandOrderingAndStatusReadbackFailure(t *testing.T) {
	runner := &nativeRunner{t: t, host: hostFixture()}
	operations := Operations{Queries: runner, Runner: runner}
	_, err := operations.Check(t.Context())
	require.NoError(t, err)
	require.Equal(t, []process.Command{
		{Name: "/usr/bin/bootc", Args: []string{"upgrade", "--check"}},
		{Name: "/usr/bin/bootc", Args: []string{"status", "--json"}},
	}, runner.commands)
	failure := errors.New("status unavailable after successful metadata check")
	queries := &nativeRunner{t: t, failure: failure}
	_, err = (Operations{Queries: queries, Runner: runner}).Check(t.Context())
	require.ErrorIs(t, err, failure)
}

func TestCheckFailureDoesNotReadStatus(t *testing.T) {
	failure := errors.New("native registry failure")
	queries, progress := &nativeRunner{t: t}, &nativeRunner{t: t, failure: failure}
	_, err := (Operations{Queries: queries, Runner: progress}).Check(t.Context())
	require.ErrorIs(t, err, failure)
	require.Empty(t, queries.commands)
}

func TestUpdateDelegatesResolutionAndActivationToNativeBootc(t *testing.T) {
	for _, name := range []string{"unchanged", "same-version cached", "staged", "download-only", "administrator source", "digest-pinned", "unknown version"} {
		t.Run(name, func(t *testing.T) {
			host := hostFixture()
			switch name {
			case "same-version cached":
				cached := *host.Status.Booted.Image
				cached.ImageDigest = "sha256:different"
				host.Status.Booted.CachedUpdate = &cached
			case "staged", "download-only":
				host.Status.Staged = &Deployment{Image: host.Status.Booted.Image, DownloadOnly: name == "download-only"}
			case "administrator source":
				host.Spec.Image = &ImageReference{Image: "example.test/custom:branch", Transport: "registry"}
			case "digest-pinned":
				host.Spec.Image.Image = "example.test/os@sha256:fixed"
			case "unknown version":
				host.Status.Booted.Image.Version = nil
			}
			queries, progress := &nativeRunner{t: t, host: host}, &nativeRunner{t: t}
			require.NoError(t, (Operations{Queries: queries, Runner: progress}).Update(t.Context()))
			require.Len(t, queries.commands, 1)
			require.Equal(t, []process.Command{{Name: "/usr/bin/bootc", Args: []string{"upgrade", "--apply"}}}, progress.commands)
			require.Same(t, t.Context(), progress.ctx)
		})
	}
}

func TestUpdateStopsOnNativeHostDiagnostics(t *testing.T) {
	for _, test := range []struct {
		name    string
		change  func(*Host)
		message string
	}{
		{"rollback", func(h *Host) { h.Status.RollbackQueued = true }, "rollback is queued"},
		{"overlay", func(h *Host) {
			h.Status.UsrOverlay = json.RawMessage(`{"accessMode":"readWrite","persistence":"transient"}`)
		}, "/usr overlay"},
		{"read-only", func(h *Host) { h.Status.ReadOnly = true }, "read-only"},
		{"incompatible booted", func(h *Host) { h.Status.Booted.Incompatible = true }, "current deployment"},
		{"no booted image", func(h *Host) { h.Status.Booted.Image = nil }, "current deployment"},
		{"incompatible staged", func(h *Host) { h.Status.Staged = &Deployment{Incompatible: true} }, "staged deployment"},
		{"no source", func(h *Host) { h.Spec.Image = nil }, "no native image source"},
	} {
		t.Run(test.name, func(t *testing.T) {
			host := hostFixture()
			test.change(&host)
			queries, progress := &nativeRunner{t: t, host: host}, &nativeRunner{t: t}
			require.ErrorContains(t, (Operations{Queries: queries, Runner: progress}).Update(t.Context()), test.message)
			require.Empty(t, progress.commands)
		})
	}
}

func TestNativeErrorsAndCancellationRemainErrorsWithoutExtraActivation(t *testing.T) {
	for _, failure := range []error{errors.New("native apply failed"), context.Canceled} {
		queries, progress := &nativeRunner{t: t, host: hostFixture()}, &nativeRunner{t: t, failure: failure}
		require.ErrorIs(t, (Operations{Queries: queries, Runner: progress}).Update(t.Context()), failure)
		require.Len(t, progress.commands, 1)
		queries.failure = failure
		progress.commands = nil
		require.ErrorIs(t, (Operations{Queries: queries, Runner: progress}).Update(t.Context()), failure)
		require.Empty(t, progress.commands)
	}
}

func TestStatusRequiresBootcButRetainsIncompatibleDeploymentFacts(t *testing.T) {
	runner := &nativeRunner{t: t}
	_, err := ReadHost(t.Context(), runner)
	require.ErrorContains(t, err, "installed bootc system")
	runner.host = hostFixture()
	runner.host.Status.Booted = &Deployment{Incompatible: true}
	host, err := ReadHost(t.Context(), runner)
	require.NoError(t, err)
	require.True(t, host.Status.Booted.Incompatible)
}
