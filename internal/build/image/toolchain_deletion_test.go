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
}
