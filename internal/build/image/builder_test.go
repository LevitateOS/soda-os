package image

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/config"
	"github.com/LevitateOS/soda-os/internal/process"
	"github.com/stretchr/testify/require"
)

const (
	testArmBaseReference        = "quay.io/fedora/fedora-bootc@sha256:950a52fa1244db4d7fe2673af57fd6784a605a83bec3cd2d716ed8c00ebd366d"
	testArmBootcNEVRA           = "bootc-0:1.16.10-1.fc44.aarch64"
	testArmPlatform             = "linux/arm64"
	testArmBuilderBaseReference = "registry.fedoraproject.org/fedora@sha256:9c8b291e256262b91aac5b3da50ea323760d0a6b449c6d6ad5f01d9550d48d2a"
)

type recordingRunner struct {
	Commands      []process.Command
	Outputs       map[string]string
	OutputResults []string
	Err           error
}

func (r *recordingRunner) Run(_ context.Context, command process.Command) error {
	r.Commands = append(r.Commands, command)
	return r.Err
}

func (r *recordingRunner) Output(_ context.Context, command process.Command) (string, error) {
	r.Commands = append(r.Commands, command)
	if len(r.OutputResults) > 0 {
		output := r.OutputResults[0]
		r.OutputResults = r.OutputResults[1:]
		return output, r.Err
	}
	return r.Outputs[command.String()], r.Err
}

func TestBootcContractForEqualSiblingArchitectures(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	require.NoError(t, err)
	for architecture, expected := range map[string][6]string{
		"aarch64": {testArmBootcNEVRA, "soda-forgejo-0:15.0.7-1.fc44.aarch64", "distro/locks/runtime-packages-aarch64.toml", "distro/locks/builder-packages-aarch64.toml", "distro/locks/installer-image-builder-aarch64.toml", "packaging/installer/iso-aarch64.yaml"},
		"x86_64":  {"bootc-0:1.16.10-1.fc44.x86_64", "soda-forgejo-0:15.0.7-1.fc44.x86_64", "distro/locks/runtime-packages-x86_64.toml", "distro/locks/builder-packages-x86_64.toml", "distro/locks/installer-image-builder-x86_64.toml", "packaging/installer/iso-x86_64.yaml"},
	} {
		t.Run(architecture, func(t *testing.T) {
			builder, err := NewBuilder(root, "distro/soda.toml", architecture, &recordingRunner{})
			require.NoError(t, err)
			require.NoError(t, builder.Check(context.Background()))

			lock, err := builder.packageLock()
			require.NoError(t, err)
			require.Equal(t, builder.Spec.Base.Reference, lock.BaseReference)
			require.Greater(t, len(lock.Package), len(targetRPMs))
			require.Contains(t, lock.Package, lockedPackage{Name: "bootc", NEVRA: expected[0], Source: "fedora"})
			require.Contains(t, lock.Package, lockedPackage{Name: "soda-forgejo", NEVRA: expected[1], Source: "local-rpm", File: strings.ReplaceAll(expected[1], "-0:", "-") + ".rpm"})
			require.Equal(t, expected[2], builder.Spec.Image.PackageLock)
			require.Equal(t, expected[3], builder.Spec.Platform.Builder.PackageLock)
			require.Equal(t, expected[4], builder.Spec.Platform.Installer.ToolLock)
			require.Equal(t, expected[5], builder.Spec.Platform.Installer.ISOConfig)
		})
	}
}

func TestDockerCommandUsesPinnedArm64Builder(t *testing.T) {
	builder := &Builder{Root: "/workspace/soda", Spec: config.DistroSpec{
		Identity: config.IdentitySpec{Version: "0.2.0"},
		Base:     config.BaseSpec{Platform: testArmPlatform},
		Platform: config.PlatformSpec{Architecture: config.PlatformArchitecture{Artifact: "aarch64"}},
	}}
	command := builder.dockerCommand([]string{"SOURCE_DATE_EPOCH=1787825905"}, "rpm", "--version")
	require.Equal(t, "docker", command.Name)
	require.Equal(t, []string{
		"run", "--rm", "--platform", "linux/arm64", "--volume", "/workspace/soda:/src", "--workdir", "/src",
		"--env", "SOURCE_DATE_EPOCH=1787825905", "soda-os-rpm-builder:0.2.0-aarch64", "rpm", "--version",
	}, command.Args)
}

func TestRPMBuilderContractPinsFedoraBaseAndInstalledInventory(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	require.NoError(t, err)

	contents, err := os.ReadFile(filepath.Join(root, "packaging", "builder", "Containerfile"))
	require.NoError(t, err)
	containerfile := string(contents)
	require.Contains(t, containerfile, "FROM ${BUILDER_BASE_REFERENCE}")
	require.Contains(t, containerfile, "COPY .artifacts/builder/packages.lock")
	require.Contains(t, containerfile, "COPY .artifacts/builder/go.tar.gz")
	require.Contains(t, containerfile, "go version go1.27.0")
	require.Contains(t, containerfile, "GOTOOLCHAIN=local")
	require.Contains(t, containerfile, "dnf -y install --setopt=install_weak_deps=False $(awk")
	require.Contains(t, containerfile, "%{ARCH}\\n' | LC_ALL=C sort")
	require.Contains(t, containerfile, "test \"$actual\" = \"$expected\"")
	require.NotContains(t, containerfile, "registry.fedoraproject.org/fedora:44")
}

func TestRPMBuildPinsHeaderTimeAndHost(t *testing.T) {
	runner := &recordingRunner{}
	builder := &Builder{Root: "/workspace/soda", runner: runner, Spec: config.DistroSpec{
		Identity: config.IdentitySpec{Version: "0.2.0"},
		Base:     config.BaseSpec{Platform: testArmPlatform},
		Platform: config.PlatformSpec{Architecture: config.PlatformArchitecture{Artifact: "aarch64"}},
		Build:    config.BuildSpec{SourceDateEpoch: 1787825905},
	}}
	require.NoError(t, builder.rpmbuild(context.Background(), "soda-runtime"))
	require.Len(t, runner.Commands, 1)
	command := runner.Commands[0].String()
	require.Contains(t, command, "--define _source_date_epoch 1787825905")
	require.Contains(t, command, "--define use_source_date_epoch_as_buildtime 1")
	require.Contains(t, command, "--define _buildhost soda-builder")
}

func TestSourceRevisionAcceptsCleanAndRejectsDirtyWorktrees(t *testing.T) {
	const revision = "79eb8c180a711f1b4230a88d95aa411b3ceb99ca"
	runner := &recordingRunner{Outputs: map[string]string{
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

	for name, status := range map[string]string{
		"tracked":   " M internal/build/image/image.go\n",
		"staged":    "M  internal/build/image/image.go\n",
		"untracked": "?? internal/build/image/new_source.go\n",
	} {
		t.Run(name, func(t *testing.T) {
			runner := &recordingRunner{Outputs: map[string]string{
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
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	require.NoError(t, err)
	for name, build := range map[string]func(context.Context, *Builder) error{
		"rpms":  func(ctx context.Context, builder *Builder) error { return builder.BuildRPMs(ctx) },
		"image": func(ctx context.Context, builder *Builder) error { return builder.BuildImage(ctx) },
	} {
		t.Run(name, func(t *testing.T) {
			runner := &recordingRunner{Outputs: map[string]string{
				"git status --porcelain=v1 --untracked-files=all": "?? relevant-source.go\n",
			}}
			builder, err := NewBuilder(root, "distro/soda.toml", "aarch64", runner)
			require.NoError(t, err)
			require.ErrorContains(t, build(context.Background(), builder), "release artifact builds require a clean Git worktree")
			require.Len(t, runner.Commands, 1)
			require.Equal(t, "git status --porcelain=v1 --untracked-files=all", runner.Commands[0].String())
		})
	}
}

func TestLockedInstallInputsRequireExactLocalRPMFiles(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "distro", "locks"), 0o755))
	lock := `schema_version = 1
base_reference = "` + testArmBaseReference + `"

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
	require.NoError(t, os.WriteFile(filepath.Join(root, "distro", "locks", "runtime-packages-aarch64.toml"), []byte(lock), 0o644))
	builder := &Builder{Root: root, Spec: config.DistroSpec{Image: config.ImageSpec{PackageLock: "distro/locks/runtime-packages-aarch64.toml"}}}
	require.ErrorContains(t, builder.writeLockedInstallInputs(filepath.Join(root, "rpms")), "locked local RPM soda-release.rpm is missing")
}

func TestPrepareLocalBootcBaseUsesExactDigestDerivedLocalTag(t *testing.T) {
	const imageID = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	list := "docker image ls --no-trunc --quiet --filter reference=" + testArmBaseReference
	runner := &recordingRunner{Outputs: map[string]string{list: imageID + "\n"}}
	platform := config.PlatformSpec{Base: config.PlatformBase{Reference: testArmBaseReference, Archive: "unused.oci.tar", ArchiveSHA256: strings.Repeat("a", 64)}}
	tag, err := PrepareLocalBootcBase(context.Background(), "/workspace", runner, platform)
	require.NoError(t, err)
	require.Equal(t, "soda-fedora-bootc:sha256-950a52fa1244db4d7fe2673af57fd6784a605a83bec3cd2d716ed8c00ebd366d", tag)
	require.Equal(t, []string{list, "docker image tag " + imageID + " " + tag}, []string{runner.Commands[0].String(), runner.Commands[1].String()})

	platform.Base.Reference = "quay.io/fedora/fedora-bootc:44"
	_, err = PrepareLocalBootcBase(context.Background(), "/workspace", runner, platform)
	require.EqualError(t, err, "local Fedora bootc base differs from the approved digest contract")
}

func TestPrepareLocalBootcBaseTagsReferenceCreatedByDockerLoad(t *testing.T) {
	root := t.TempDir()
	archive := []byte("valid local OCI archive")
	require.NoError(t, os.WriteFile(filepath.Join(root, "base.oci.tar"), archive, 0o644))
	checksum := sha256.Sum256(archive)
	platform := config.PlatformSpec{Base: config.PlatformBase{
		Reference:     testArmBaseReference,
		Archive:       "base.oci.tar",
		ArchiveSHA256: fmt.Sprintf("%x", checksum),
	}}
	const imageID = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	runner := &recordingRunner{OutputResults: []string{"", imageID + "\n"}}

	tag, err := PrepareLocalBootcBase(context.Background(), root, runner, platform)
	require.NoError(t, err)
	require.Equal(t, "soda-fedora-bootc:sha256-950a52fa1244db4d7fe2673af57fd6784a605a83bec3cd2d716ed8c00ebd366d", tag)
	require.Equal(t, []string{
		"docker image ls --no-trunc --quiet --filter reference=" + testArmBaseReference,
		"docker load --input " + filepath.Join(root, "base.oci.tar"),
		"docker image ls --no-trunc --quiet --filter reference=" + testArmBaseReference,
		"docker image tag " + imageID + " " + tag,
	}, []string{runner.Commands[0].String(), runner.Commands[1].String(), runner.Commands[2].String(), runner.Commands[3].String()})
}

func TestSodaRPMsAreScriptletFree(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	require.NoError(t, err)
	scriptlet := regexp.MustCompile(`(?m)^%(?:pre(?:un|trans)?|post(?:un|trans)?|trigger\w*|filetrigger\w*|transfiletrigger\w*|verifyscript)(?:\s|$)`)
	for _, name := range targetRPMs {
		owner := strings.TrimPrefix(name, "soda-")
		spec, err := os.ReadFile(filepath.Join(root, "packaging", "rpm", owner, name+".spec"))
		require.NoError(t, err)
		require.NotRegexp(t, scriptlet, string(spec), "%s must be an image-build input without RPM lifecycle scriptlets", name)
	}
}

func TestStageDistributionWritesUpdateLocation(t *testing.T) {
	root := t.TempDir()
	builder := &Builder{Root: root, Spec: testDistributionSpec()}
	require.NoError(t, builder.stageDistribution())
	distribution, err := os.ReadFile(filepath.Join(root, ".artifacts", "bootc", "distribution", "distribution.json"))
	require.NoError(t, err)
	require.JSONEq(t, `{"github_repository":"LevitateOS/soda-os","index_url":"https://github.com/LevitateOS/soda-os/releases/latest/download/soda-os-release-index.json"}`, string(distribution))
}

func testDistributionSpec() config.DistroSpec {
	return config.DistroSpec{Distribution: config.DistributionSpec{
		GitHubRepository: "LevitateOS/soda-os",
		IndexURL:         "https://github.com/LevitateOS/soda-os/releases/latest/download/soda-os-release-index.json",
	}}
}
