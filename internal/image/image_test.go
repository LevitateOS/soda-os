package image

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/config"
	"github.com/stretchr/testify/require"
)

func TestBootcContract(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)
	builder, err := NewBuilder(root, "distro/soda.toml", &RecordingRunner{})
	require.NoError(t, err)
	require.NoError(t, builder.Check(context.Background()))

	lock, err := builder.packageLock()
	require.NoError(t, err)
	require.Equal(t, bootcBaseReference, lock.BaseReference)
	require.Greater(t, len(lock.Package), len(targetRPMs))
	for _, item := range lock.Package {
		require.NotEmpty(t, item.NEVRA)
	}
}

func TestDockerCommandUsesPinnedArm64Builder(t *testing.T) {
	builder := &Builder{Root: "/workspace/soda", Spec: config.DistroSpec{
		Identity: config.IdentitySpec{Version: "0.2.0"},
		Base:     config.BaseSpec{Platform: bootcPlatform},
	}}
	command := builder.dockerCommand([]string{"SOURCE_DATE_EPOCH=1787825905"}, "rpm", "--version")
	require.Equal(t, "docker", command.Name)
	require.Equal(t, []string{
		"run", "--rm", "--platform", "linux/arm64", "--volume", "/workspace/soda:/src", "--workdir", "/src",
		"--env", "SOURCE_DATE_EPOCH=1787825905", "soda-os-rpm-builder:0.2.0", "rpm", "--version",
	}, command.Args)
}

func TestRPMBuildPinsHeaderTimeAndHost(t *testing.T) {
	runner := &RecordingRunner{}
	builder := &Builder{Root: "/workspace/soda", runner: runner, Spec: config.DistroSpec{
		Identity: config.IdentitySpec{Version: "0.2.0"},
		Base:     config.BaseSpec{Platform: bootcPlatform},
		Build:    config.BuildSpec{SourceDateEpoch: 1787825905},
	}}
	require.NoError(t, builder.rpmbuild(context.Background(), "soda-runtime"))
	require.Len(t, runner.Commands, 1)
	command := runner.Commands[0].String()
	require.Contains(t, command, "--define _source_date_epoch 1787825905")
	require.Contains(t, command, "--define use_source_date_epoch_as_buildtime 1")
	require.Contains(t, command, "--define _buildhost soda-builder")
}

func TestSourceRevisionAcceptsCleanWorktree(t *testing.T) {
	const revision = "79eb8c180a711f1b4230a88d95aa411b3ceb99ca"
	runner := &RecordingRunner{Outputs: map[string]string{
		"git status --porcelain=v1 --untracked-files=all": "",
		"git rev-parse HEAD":                              revision + "\n",
	}}
	builder := &Builder{Root: "/workspace/soda", runner: runner}
	actual, err := builder.sourceRevision(context.Background())
	require.NoError(t, err)
	require.Equal(t, revision, actual)
	require.Equal(t, []string{
		"git status --porcelain=v1 --untracked-files=all",
		"git rev-parse HEAD",
	}, []string{runner.Commands[0].String(), runner.Commands[1].String()})
}

func TestSourceRevisionRejectsDirtyWorktree(t *testing.T) {
	for name, status := range map[string]string{
		"tracked":   " M internal/image/image.go\n",
		"staged":    "M  internal/image/image.go\n",
		"untracked": "?? internal/image/new_source.go\n",
	} {
		t.Run(name, func(t *testing.T) {
			runner := &RecordingRunner{Outputs: map[string]string{
				"git status --porcelain=v1 --untracked-files=all": status,
			}}
			builder := &Builder{Root: "/workspace/soda", runner: runner}
			_, err := builder.sourceRevision(context.Background())
			require.EqualError(t, err, "release artifact builds require a clean Git worktree; commit or remove tracked, staged, and untracked source changes")
			require.Len(t, runner.Commands, 1)
			require.Equal(t, "git status --porcelain=v1 --untracked-files=all", runner.Commands[0].String())
		})
	}
}

func TestArtifactBuildsRejectDirtyWorktreeBeforeDocker(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)
	for name, build := range map[string]func(context.Context, *Builder) error{
		"rpms":  func(ctx context.Context, builder *Builder) error { return builder.BuildRPMs(ctx) },
		"image": func(ctx context.Context, builder *Builder) error { return builder.BuildImage(ctx) },
	} {
		t.Run(name, func(t *testing.T) {
			runner := &RecordingRunner{Outputs: map[string]string{
				"git status --porcelain=v1 --untracked-files=all": "?? relevant-source.go\n",
			}}
			builder, err := NewBuilder(root, "distro/soda.toml", runner)
			require.NoError(t, err)
			require.ErrorContains(t, build(context.Background(), builder), "release artifact builds require a clean Git worktree")
			require.Len(t, runner.Commands, 1)
			require.Equal(t, "git status --porcelain=v1 --untracked-files=all", runner.Commands[0].String())
		})
	}
}

func TestLockedInstallInputsRequireExactLocalRPMFiles(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "packaging", "bootc"), 0o755))
	lock := `schema_version = 1
base_reference = "` + bootcBaseReference + `"

[[package]]
name = "make"
nevra = "make-1:4.4.1-12.fc44.aarch64"
source = "fedora"

[[package]]
name = "soda-release"
nevra = "soda-release-0:0.2.0-1.fc44.noarch"
source = "local-rpm"
file = "soda-release.rpm"
`
	require.NoError(t, os.WriteFile(filepath.Join(root, "packaging", "bootc", "packages.lock"), []byte(lock), 0o644))
	builder := &Builder{Root: root, Spec: config.DistroSpec{Image: config.ImageSpec{PackageLock: "packaging/bootc/packages.lock"}}}
	require.ErrorContains(t, builder.writeLockedInstallInputs(filepath.Join(root, "rpms")), "locked local RPM soda-release.rpm is missing")
}

func TestRuntimeImageEnablesServicesAndMasksAutomaticUpdates(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join("..", "..", "packaging", "bootc", "Containerfile"))
	require.NoError(t, err)
	containerfile := string(contents)
	for _, expected := range []string{
		bootcBaseReference,
		"systemd-sysusers /usr/lib/sysusers.d/soda.conf",
		"systemctl enable sshd.service sodad.service soda-authd.service soda-cockpit.service avahi-daemon.service srv-soda-projects.mount opt-soda-toolchains.mount",
		"systemctl mask bootc-fetch-apply-updates.timer",
		"rpm-inventory.sha256",
		"sha256sum --check rpm-inventory.sha256",
		"/usr/lib/sysimage/libdnf5/transaction_history.sqlite*",
		"/var/cache/ldconfig/aux-cache",
		"/var/lib/dnf/repos",
		"/var/log/dnf5.log",
	} {
		require.Contains(t, containerfile, expected)
	}
	require.NotContains(t, containerfile, "bootc-fetch-apply-updates.service")

	sysusers, err := os.ReadFile(filepath.Join("..", "..", "packaging", "sysusers.d", "soda.conf"))
	require.NoError(t, err)
	require.Contains(t, string(sysusers), "g soda-api 976")
	require.Contains(t, string(sysusers), "u soda-cockpit 976:soda-api")

	preset, err := os.ReadFile(filepath.Join("..", "..", "packaging", "systemd", "90-soda.preset"))
	require.NoError(t, err)
	for _, unit := range []string{"sshd.service", "sodad.service", "soda-authd.service", "soda-cockpit.service", "avahi-daemon.service", "srv-soda-projects.mount", "opt-soda-toolchains.mount"} {
		require.True(t, strings.Contains(string(preset), "enable "+unit))
	}
}
