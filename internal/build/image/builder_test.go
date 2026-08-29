package image

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/LevitateOS/soda-os/internal/config"
	"github.com/LevitateOS/soda-os/internal/process"
	"github.com/stretchr/testify/require"
)

const (
	testArmBaseReference        = "quay.io/fedora/fedora-bootc@sha256:85677d47c03b2e1f8f9a3a19d838023ea154229817d579d4b4da5b87a21c9c1a"
	testArmBootcNEVRA           = "bootc-0:1.16.10-1.fc44.aarch64"
	testArmPlatform             = "linux/arm64"
	testArmBuilderBaseReference = "registry.fedoraproject.org/fedora@sha256:9c8b291e256262b91aac5b3da50ea323760d0a6b449c6d6ad5f01d9550d48d2a"
	testArmCosignSHA            = "90e7ae0b5dfd60f20816b52c012addf7fc055ebcc7bea4ce81c428ca8518c302"
)

type recordingRunner struct {
	Commands []process.Command
	Outputs  map[string]string
	Err      error
}

func (r *recordingRunner) Run(_ context.Context, command process.Command) error {
	r.Commands = append(r.Commands, command)
	return r.Err
}

func (r *recordingRunner) Output(_ context.Context, command process.Command) (string, error) {
	r.Commands = append(r.Commands, command)
	return r.Outputs[command.String()], r.Err
}

func TestBootcContractForEqualSiblingArchitectures(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	require.NoError(t, err)
	for architecture, expectedBootc := range map[string]string{"aarch64": testArmBootcNEVRA, "x86_64": "bootc-0:1.16.10-1.fc44.x86_64"} {
		t.Run(architecture, func(t *testing.T) {
			builder, err := NewBuilder(root, "distro/soda.toml", architecture, &recordingRunner{})
			require.NoError(t, err)
			require.NoError(t, builder.Check(context.Background()))

			lock, err := builder.packageLock()
			require.NoError(t, err)
			require.Equal(t, builder.Spec.Base.Reference, lock.BaseReference)
			require.Greater(t, len(lock.Package), len(targetRPMs))
			foundBootc := false
			for _, item := range lock.Package {
				require.NotEmpty(t, item.NEVRA)
				if item.Name == "bootc" {
					foundBootc = true
					require.Equal(t, expectedBootc, item.NEVRA)
					require.Equal(t, "fedora", item.Source)
				}
			}
			require.True(t, foundBootc)
		})
	}
}

func TestDockerCommandUsesPinnedArm64Builder(t *testing.T) {
	builder := &Builder{Root: "/workspace/soda", Spec: config.DistroSpec{
		Identity: config.IdentitySpec{Version: "0.2.0"},
		Base:     config.BaseSpec{Platform: testArmPlatform},
		Platform: config.PlatformSpec{ArtifactArchitecture: "aarch64"},
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
		Platform: config.PlatformSpec{ArtifactArchitecture: "aarch64"},
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
	require.NoError(t, os.WriteFile(filepath.Join(root, "distro", "locks", "runtime-packages.toml"), []byte(lock), 0o644))
	builder := &Builder{Root: root, Spec: config.DistroSpec{Image: config.ImageSpec{PackageLock: "distro/locks/runtime-packages.toml"}}}
	require.ErrorContains(t, builder.writeLockedInstallInputs(filepath.Join(root, "rpms")), "locked local RPM soda-release.rpm is missing")
}

func TestPrepareLocalBootcBaseUsesExactDigestDerivedLocalTag(t *testing.T) {
	runner := &recordingRunner{}
	platform := config.PlatformSpec{BaseReference: testArmBaseReference, BaseArchive: "unused.oci.tar", BaseArchiveSHA256: strings.Repeat("a", 64)}
	tag, err := PrepareLocalBootcBase(context.Background(), "/workspace", runner, platform)
	require.NoError(t, err)
	require.Equal(t, "soda-fedora-bootc:sha256-85677d47c03b2e1f8f9a3a19d838023ea154229817d579d4b4da5b87a21c9c1a", tag)
	require.Equal(t, "docker image tag sha256:85677d47c03b2e1f8f9a3a19d838023ea154229817d579d4b4da5b87a21c9c1a "+tag, runner.Commands[0].String())

	platform.BaseReference = "quay.io/fedora/fedora-bootc:44"
	_, err = PrepareLocalBootcBase(context.Background(), "/workspace", runner, platform)
	require.EqualError(t, err, "local Fedora bootc base differs from the approved digest contract")
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

func TestRuntimeCosignInputIsPinnedForLinuxAArch64(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("..", "..", "..", "scripts", "fetch-release-tools.sh"))
	require.NoError(t, err)
	require.Contains(t, string(script), "cosign-linux-arm64")
	require.Contains(t, string(script), testArmCosignSHA)
	require.Contains(t, string(script), "cosign-linux-amd64")
	require.Contains(t, string(script), "releases/download/v3.1.2")

	spec, err := os.ReadFile(filepath.Join("..", "..", "..", "packaging", "rpm", "runtime", "soda-runtime.spec"))
	require.NoError(t, err)
	require.Contains(t, string(spec), "install -m 0755 %{_sourcedir}/cosign %{buildroot}%{_libexecdir}/soda/cosign")
	require.Contains(t, string(spec), "%{_libexecdir}/soda/cosign")

	root := t.TempDir()
	tool := filepath.Join(root, ".artifacts", "tools", "cosign-linux-arm64")
	require.NoError(t, os.MkdirAll(filepath.Dir(tool), 0o755))
	require.NoError(t, os.WriteFile(tool, []byte("not the pinned binary"), 0o755))
	err = (&Builder{Root: root, Spec: config.DistroSpec{Platform: config.PlatformSpec{TargetCosignArchitecture: "arm64", TargetCosignSHA256: testArmCosignSHA}}}).verifyRuntimeCosign()
	require.ErrorContains(t, err, "differs from pinned")
}

func TestStageReleaseTrustAcceptsExplicitCAAndPublicKey(t *testing.T) {
	root := t.TempDir()
	caPath, publicKeyPath := writeTestReleaseTrust(t, root)
	builder := &Builder{Root: root, RegistryCA: caPath, SigningPublicKey: publicKeyPath}
	require.NoError(t, builder.stageReleaseTrust())

	stagedCA, err := os.ReadFile(filepath.Join(root, ".artifacts", "bootc", "trust", "registry-ca.crt"))
	require.NoError(t, err)
	wantCA, err := os.ReadFile(caPath)
	require.NoError(t, err)
	require.Equal(t, wantCA, stagedCA)
	stagedKey, err := os.ReadFile(filepath.Join(root, ".artifacts", "bootc", "trust", "cosign.pub"))
	require.NoError(t, err)
	wantKey, err := os.ReadFile(publicKeyPath)
	require.NoError(t, err)
	require.Equal(t, wantKey, stagedKey)

	policy, err := os.ReadFile(filepath.Join("..", "..", "..", "packaging", "bootc", "trust", "policy.json"))
	require.NoError(t, err)
	require.Contains(t, string(policy), `"registry.soda.local/soda/os"`)
	require.Contains(t, string(policy), `"type": "sigstoreSigned"`)
	require.Contains(t, string(policy), `"keyPath": "/usr/share/soda/release/cosign.pub"`)
	registries, err := os.ReadFile(filepath.Join("..", "..", "..", "packaging", "bootc", "trust", "registries.d.yaml"))
	require.NoError(t, err)
	require.Contains(t, string(registries), "use-sigstore-attachments: true")
}

func TestStageReleaseTrustPreservesInputsAlreadyAtDestination(t *testing.T) {
	root := t.TempDir()
	trustDir := filepath.Join(root, ".artifacts", "bootc", "trust")
	require.NoError(t, os.MkdirAll(trustDir, 0o755))
	caPath, publicKeyPath := writeTestReleaseTrust(t, trustDir)
	wantCA, err := os.ReadFile(caPath)
	require.NoError(t, err)
	wantKey, err := os.ReadFile(publicKeyPath)
	require.NoError(t, err)

	builder := &Builder{Root: root, RegistryCA: caPath, SigningPublicKey: publicKeyPath}
	require.NoError(t, builder.stageReleaseTrust())

	stagedCA, err := os.ReadFile(caPath)
	require.NoError(t, err)
	require.Equal(t, wantCA, stagedCA)
	stagedKey, err := os.ReadFile(publicKeyPath)
	require.NoError(t, err)
	require.Equal(t, wantKey, stagedKey)
}

func TestBuildImageStagesTrustAfterLockedBootcInputs(t *testing.T) {
	source, err := os.ReadFile("builder.go")
	require.NoError(t, err)
	buildRPMs := strings.Index(string(source), "if err := b.BuildRPMs(ctx)")
	stageTrust := strings.Index(string(source), "if err := b.stageReleaseTrust()")
	require.Greater(t, buildRPMs, -1)
	require.Greater(t, stageTrust, buildRPMs)

	root := t.TempDir()
	caPath, publicKeyPath := writeTestReleaseTrust(t, root)
	bootcInputs := filepath.Join(root, ".artifacts", "bootc")
	require.NoError(t, recreate(bootcInputs))
	require.NoError(t, os.WriteFile(filepath.Join(bootcInputs, "expected-packages.txt"), []byte("locked inputs\n"), 0o644))
	builder := &Builder{Root: root, RegistryCA: caPath, SigningPublicKey: publicKeyPath}
	require.NoError(t, builder.stageReleaseTrust())
	require.FileExists(t, filepath.Join(bootcInputs, "expected-packages.txt"))
	require.FileExists(t, filepath.Join(bootcInputs, "trust", "registry-ca.crt"))
	require.FileExists(t, filepath.Join(bootcInputs, "trust", "cosign.pub"))
}

func writeTestReleaseTrust(t *testing.T, root string) (string, string) {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	certificate, err := x509.CreateCertificate(rand.Reader, &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Soda test registry CA"},
		NotBefore: time.Unix(1, 0), NotAfter: time.Unix(2, 0), IsCA: true,
		BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}, &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Soda test registry CA"}, IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}, publicKey, privateKey)
	require.NoError(t, err)
	caPath := filepath.Join(root, "registry-ca.crt")
	require.NoError(t, os.WriteFile(caPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate}), 0o644))
	encodedPublicKey, err := x509.MarshalPKIXPublicKey(publicKey)
	require.NoError(t, err)
	publicKeyPath := filepath.Join(root, "cosign.pub")
	require.NoError(t, os.WriteFile(publicKeyPath, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: encodedPublicKey}), 0o644))
	return caPath, publicKeyPath
}
