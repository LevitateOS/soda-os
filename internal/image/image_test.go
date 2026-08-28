package image

import (
	"bytes"
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
	"github.com/stretchr/testify/require"
)

type recordingRunner struct {
	Commands []Command
	Outputs  map[string]string
	Err      error
}

func (r *recordingRunner) Run(_ context.Context, command Command) error {
	r.Commands = append(r.Commands, command)
	return r.Err
}

func (r *recordingRunner) Output(_ context.Context, command Command) (string, error) {
	r.Commands = append(r.Commands, command)
	return r.Outputs[command.String()], r.Err
}

func TestBootcContract(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)
	builder, err := NewBuilder(root, "distro/soda.toml", &recordingRunner{})
	require.NoError(t, err)
	require.NoError(t, builder.Check(context.Background()))

	lock, err := builder.packageLock()
	require.NoError(t, err)
	require.Equal(t, bootcBaseReference, lock.BaseReference)
	require.Greater(t, len(lock.Package), len(targetRPMs))
	foundBootc := false
	for _, item := range lock.Package {
		require.NotEmpty(t, item.NEVRA)
		if item.Name == "bootc" {
			foundBootc = true
			require.Equal(t, bootcRuntimeNEVRA, item.NEVRA)
			require.Equal(t, "fedora", item.Source)
		}
	}
	require.True(t, foundBootc)
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

func TestRPMBuilderContractPinsFedoraBaseAndInstalledInventory(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)

	contents, err := os.ReadFile(filepath.Join(root, "packaging", "builder", "Containerfile"))
	require.NoError(t, err)
	containerfile := string(contents)
	require.Contains(t, containerfile, "FROM "+builderBaseReference)
	require.Contains(t, containerfile, "COPY packaging/builder/packages.lock")
	require.Contains(t, containerfile, "dnf -y install --setopt=install_weak_deps=False $(awk")
	require.Contains(t, containerfile, "%{ARCH}\\n' | LC_ALL=C sort")
	require.Contains(t, containerfile, "test \"$actual\" = \"$expected\"")
	require.NotContains(t, containerfile, "registry.fedoraproject.org/fedora:44")
}

func TestOSRunnerWiresOnlyExplicitStdin(t *testing.T) {
	input := bytes.NewBufferString("administrator input")
	runner := OSRunner{Stdin: input}
	command := runner.command(context.Background(), Command{Name: "ignored"})
	require.Same(t, input, command.Stdin)
	require.Nil(t, (OSRunner{}).command(context.Background(), Command{Name: "ignored"}).Stdin)
}

func TestRPMBuildPinsHeaderTimeAndHost(t *testing.T) {
	runner := &recordingRunner{}
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
		"tracked":   " M internal/image/image.go\n",
		"staged":    "M  internal/image/image.go\n",
		"untracked": "?? internal/image/new_source.go\n",
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
	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)
	for name, build := range map[string]func(context.Context, *Builder) error{
		"rpms":  func(ctx context.Context, builder *Builder) error { return builder.BuildRPMs(ctx) },
		"image": func(ctx context.Context, builder *Builder) error { return builder.BuildImage(ctx) },
	} {
		t.Run(name, func(t *testing.T) {
			runner := &recordingRunner{Outputs: map[string]string{
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

func TestPrepareLocalBootcBaseUsesExactDigestDerivedLocalTag(t *testing.T) {
	runner := &recordingRunner{}
	tag, err := PrepareLocalBootcBase(context.Background(), "/workspace", runner, bootcBaseReference)
	require.NoError(t, err)
	require.Equal(t, "soda-fedora-bootc:sha256-85677d47c03b2e1f8f9a3a19d838023ea154229817d579d4b4da5b87a21c9c1a", tag)
	require.Equal(t, "docker image tag sha256:85677d47c03b2e1f8f9a3a19d838023ea154229817d579d4b4da5b87a21c9c1a "+tag, runner.Commands[0].String())

	_, err = PrepareLocalBootcBase(context.Background(), "/workspace", runner, "quay.io/fedora/fedora-bootc:44")
	require.EqualError(t, err, "local Fedora bootc base differs from the approved digest")
}

func TestSodaRPMsAreScriptletFree(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)
	scriptlet := regexp.MustCompile(`(?m)^%(?:pre(?:un|trans)?|post(?:un|trans)?|trigger\w*|filetrigger\w*|transfiletrigger\w*|verifyscript)(?:\s|$)`)
	for _, name := range targetRPMs {
		spec, err := os.ReadFile(filepath.Join(root, "packaging", "rpm", name+".spec"))
		require.NoError(t, err)
		require.NotRegexp(t, scriptlet, string(spec), "%s must be an image-build input without RPM lifecycle scriptlets", name)
	}
}

func TestRuntimeCosignInputIsPinnedForLinuxAArch64(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("..", "..", "scripts", "fetch-release-tools.sh"))
	require.NoError(t, err)
	require.Contains(t, string(script), "cosign-linux-arm64")
	require.Contains(t, string(script), cosignArm64SHA256)
	require.Contains(t, string(script), "releases/download/v3.1.2")

	spec, err := os.ReadFile(filepath.Join("..", "..", "packaging", "rpm", "soda-runtime.spec"))
	require.NoError(t, err)
	require.Contains(t, string(spec), "install -m 0755 %{_sourcedir}/cosign %{buildroot}%{_libexecdir}/soda/cosign")
	require.Contains(t, string(spec), "%{_libexecdir}/soda/cosign")

	root := t.TempDir()
	tool := filepath.Join(root, ".artifacts", "tools", "cosign-linux-arm64")
	require.NoError(t, os.MkdirAll(filepath.Dir(tool), 0o755))
	require.NoError(t, os.WriteFile(tool, []byte("not the pinned binary"), 0o755))
	err = (&Builder{Root: root}).verifyRuntimeCosign()
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

	policy, err := os.ReadFile(filepath.Join("..", "..", "packaging", "release", "policy.json"))
	require.NoError(t, err)
	require.Contains(t, string(policy), `"registry.soda.local/soda/os"`)
	require.Contains(t, string(policy), `"type": "sigstoreSigned"`)
	require.Contains(t, string(policy), `"keyPath": "/usr/share/soda/release/cosign.pub"`)
	registries, err := os.ReadFile(filepath.Join("..", "..", "packaging", "release", "registries.d.yaml"))
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
	source, err := os.ReadFile("image.go")
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
