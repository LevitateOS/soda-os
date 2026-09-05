package image

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/config"
	"github.com/stretchr/testify/require"
)

func TestCosignSourceAndNativeRPMSpecAgree(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	lock, err := readCosignSourceLock(filepath.Join(root, "distro/locks/cosign-source.toml"))
	require.NoError(t, err)
	spec, err := os.ReadFile(filepath.Join(root, "packaging/rpm/cosign/soda-cosign.spec"))
	require.NoError(t, err)
	require.Contains(t, string(spec), "Version:        "+lock.Version)
	require.Contains(t, string(spec), "ExclusiveArch:  x86_64 aarch64")
	require.Contains(t, string(spec), "%{_bindir}/cosign")
	require.Contains(t, string(spec), "%license")
	for _, architecture := range []string{"aarch64", "x86_64"} {
		runtime, err := readPackageLock(filepath.Join(root, "distro/locks/runtime-packages-"+architecture+".toml"))
		require.NoError(t, err)
		require.Contains(t, runtime.Package, lockedPackage{
			Name: "soda-cosign", NEVRA: "soda-cosign-0:" + lock.Version + "-1.fc44." + architecture,
			Source: "local-rpm", File: "soda-cosign-" + lock.Version + "-1.fc44." + architecture + ".rpm",
		})
	}
}

func TestCosignSourceLockRejectsInvalidInputs(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join("..", "..", "..", "distro/locks/cosign-source.toml"))
	require.NoError(t, err)
	lock, err := readCosignSourceLock(filepath.Join("..", "..", "..", "distro/locks/cosign-source.toml"))
	require.NoError(t, err)
	for _, changed := range []string{
		string(contents) + "unexpected = true\n",
		strings.ReplaceAll(string(contents), lock.Version, "invalid"),
		strings.ReplaceAll(string(contents), lock.Commit, "bad"),
		strings.ReplaceAll(string(contents), lock.SourceArchive, "../source.tar.gz"),
		strings.ReplaceAll(string(contents), lock.SourceSHA256, "bad"),
		strings.ReplaceAll(string(contents), lock.SourceURL, ""),
	} {
		path := filepath.Join(t.TempDir(), "source.toml")
		require.NoError(t, os.WriteFile(path, []byte(changed), 0o644))
		_, err := readCosignSourceLock(path)
		require.Error(t, err)
	}
}

func TestBuildCosignVerifiesSourceBeforeNativeDocker(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"distro/locks", ".artifacts/tools"} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, dir), 0o755))
	}
	archive := []byte("source fixture")
	contents, err := os.ReadFile(filepath.Join("..", "..", "..", "distro/locks/cosign-source.toml"))
	require.NoError(t, err)
	lock, err := readCosignSourceLock(filepath.Join("..", "..", "..", "distro/locks/cosign-source.toml"))
	require.NoError(t, err)
	contents = []byte(strings.ReplaceAll(string(contents), lock.SourceSHA256, fmt.Sprintf("%x", sha256.Sum256(archive))))
	require.NoError(t, os.WriteFile(filepath.Join(root, "distro/locks/cosign-source.toml"), contents, 0o644))
	path := filepath.Join(root, ".artifacts/tools", lock.SourceArchive)
	require.NoError(t, os.WriteFile(path, archive, 0o644))
	runner := &recordingRunner{}
	builder := &Builder{Root: root, runner: runner, Spec: config.DistroSpec{Base: config.BaseSpec{Platform: "linux/arm64"}}}
	require.NoError(t, builder.buildCosign(context.Background()))
	require.Len(t, runner.Commands, 1)
	args := strings.Join(runner.Commands[0].Args, " ")
	for _, expected := range []string{"--platform linux/arm64", "GOTOOLCHAIN=local", "-mod=readonly", "GIT_VERSION=v" + lock.Version, "GIT_HASH=" + lock.Commit, "cosign-LICENSE", "version --json"} {
		require.Contains(t, args, expected)
	}
	require.NoError(t, os.WriteFile(path, []byte("changed"), 0o644))
	require.ErrorContains(t, builder.buildCosign(context.Background()), "SHA-256 checksum mismatch")
	require.Len(t, runner.Commands, 1)
}

func TestAArch64OpenSSLLockMatchesLibraries(t *testing.T) {
	lock, err := readPackageLock(filepath.Join("..", "..", "..", "distro/locks/runtime-packages-aarch64.toml"))
	require.NoError(t, err)
	packages := map[string]string{}
	for _, item := range lock.Package {
		packages[item.Name] = strings.TrimPrefix(item.NEVRA, item.Name+"-")
	}
	require.NotEmpty(t, packages["openssl"])
	require.Equal(t, packages["openssl"], packages["openssl-libs"])
}
