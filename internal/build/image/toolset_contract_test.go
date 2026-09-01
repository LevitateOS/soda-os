package image

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImmutableToolsetManifestIsExact(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	contents, err := os.ReadFile(filepath.Join(root, "distro", "toolset-commands.txt"))
	require.NoError(t, err)
	commands := strings.Split(strings.TrimSuffix(string(contents), "\n"), "\n")
	require.Equal(t, []string{
		"go", "gofmt", "python3", "uv", "uvx", "rustc", "cargo", "rustfmt", "cargo-clippy",
		"node", "npm", "npx", "bun", "gcc", "g++", "cpp", "as", "ld", "ar", "make", "cmake", "ninja", "pkg-config",
		"git", "git-lfs", "gh", "ssh", "scp", "sftp", "rsync", "podman", "buildah", "skopeo", "sqlite3", "jq", "yq",
		"curl", "wget", "openssl", "patch", "rg", "fd", "fzf", "shellcheck", "just", "tar", "gzip", "bzip2", "xz",
		"zstd", "zip", "unzip", "vim", "nano",
	}, commands)
	for _, command := range commands {
		require.Equal(t, command, strings.TrimSpace(command))
		require.NotContains(t, command, " ")
	}
}

func TestBunIsOneLockedImmutableRPMPerArchitecture(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	require.NoError(t, err)
	for _, architecture := range []string{"aarch64", "x86_64"} {
		t.Run(architecture, func(t *testing.T) {
			builder, buildErr := NewBuilder(root, "distro/soda.toml", architecture, nil)
			require.NoError(t, buildErr)
			lock, lockErr := builder.packageLock()
			require.NoError(t, lockErr)
			require.Contains(t, lock.Package, lockedPackage{
				Name: "soda-bun", NEVRA: "soda-bun-0:1.4.0-1.fc44." + architecture,
				Source: "local-rpm", File: "soda-bun-1.4.0-1.fc44." + architecture + ".rpm",
			})
		})
	}

	spec, err := os.ReadFile(filepath.Join(root, "packaging", "rpm", "bun", "soda-bun.spec"))
	require.NoError(t, err)
	require.Contains(t, string(spec), "This package contains no downloader, updater, or persistent runtime state.")
	require.Contains(t, string(spec), `%check`)
	require.Contains(t, string(spec), `test "$(%{_sourcedir}/bun --version)" = "%{version}"`)
	require.Contains(t, string(spec), `process.stdout.write("soda-bun-native")`)
	require.NotContains(t, string(spec), "%post")
	require.NotContains(t, string(spec), "curl")
}

func TestBuilderAndBunInputsAreSelectedForTheNativeArchitecture(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	builderFetcher, err := os.ReadFile(filepath.Join(root, "scripts", "fetch-builder-tools.sh"))
	require.NoError(t, err)
	require.Contains(t, string(builderFetcher), `case "$1:$(uname -m)" in`)
	require.Contains(t, string(builderFetcher), `fetch_toolchain "$architecture" "$expected"`)
	require.NotContains(t, string(builderFetcher), "fetch_toolchain amd64 ")
	require.NotContains(t, string(builderFetcher), "fetch_toolchain arm64 ")

	justfile, err := os.ReadFile(filepath.Join(root, "justfile"))
	require.NoError(t, err)
	require.Contains(t, string(justfile), "rpm architecture: (builder-tools architecture) forgejo-source bun-source")
	require.Contains(t, string(justfile), "oci architecture: (builder-tools architecture) forgejo-source bun-source")
}
