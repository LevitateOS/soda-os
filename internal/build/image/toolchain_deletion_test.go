package image

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRuntimeToolchainOwnershipIsAbsent(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	for _, directory := range []string{
		filepath.Join(root, "internal", "toolchain"),
		filepath.Join(root, "distro", "profiles"),
	} {
		entries, err := filepath.Glob(filepath.Join(directory, "*"))
		require.NoError(t, err)
		require.Empty(t, entries, directory)
	}
	for _, path := range []string{
		filepath.Join(root, "distro", "toolset-commands.txt"),
		filepath.Join(root, "distro", "locks", "bun-source.toml"),
		filepath.Join(root, "packaging", "rpm", "bun", "soda-bun.spec"),
		filepath.Join(root, "scripts", "fetch-bun-source.sh"),
		filepath.Join(root, "internal", "build", "image", "bun.go"),
		filepath.Join(root, "packaging", "rpm", "runtime", "sources", "systemd", "opt-soda-toolchains.mount"),
		filepath.Join(root, "packaging", "rpm", "runtime", "sources", "systemd", "soda-state-directories.service"),
	} {
		_, err := os.Stat(path)
		require.ErrorIs(t, err, os.ErrNotExist, path)
	}

	runtimeSpec, err := os.ReadFile(filepath.Join(root, "packaging", "rpm", "runtime", "soda-runtime.spec"))
	require.NoError(t, err)
	dependencies := specRequires(string(runtimeSpec))
	for _, tool := range []string{"gcc", "gcc-c++", "git-core", "make", "openssh-clients", "pkgconf-pkg-config", "tar", "unzip", "xz"} {
		require.NotContains(t, dependencies, tool)
	}
	for _, path := range []string{
		filepath.Join(root, "distro", "locks", "runtime-packages-aarch64.toml"),
		filepath.Join(root, "distro", "locks", "runtime-packages-x86_64.toml"),
	} {
		contents, readErr := os.ReadFile(path)
		require.NoError(t, readErr)
		require.NotContains(t, string(contents), "soda-bun")
	}
}
