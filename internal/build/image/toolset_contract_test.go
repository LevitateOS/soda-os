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
		"git", "git-lfs", "gh", "tea", "ssh", "scp", "sftp", "rsync", "podman", "buildah", "skopeo", "sqlite3", "jq", "yq",
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

func TestTeaIsOneLockedImmutableRPMPerArchitecture(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	require.NoError(t, err)
	for _, architecture := range []string{"aarch64", "x86_64"} {
		t.Run(architecture, func(t *testing.T) {
			builder, buildErr := NewBuilder(root, "distro/soda.toml", architecture, nil)
			require.NoError(t, buildErr)
			lock, lockErr := builder.packageLock()
			require.NoError(t, lockErr)
			require.Contains(t, lock.Package, lockedPackage{
				Name: "soda-tea", NEVRA: "soda-tea-0:0.15.1-1.fc44." + architecture,
				Source: "local-rpm", File: "soda-tea-0.15.1-1.fc44." + architecture + ".rpm",
			})
		})
	}

	spec, err := os.ReadFile(filepath.Join(root, "packaging", "rpm", "tea", "soda-tea.spec"))
	require.NoError(t, err)
	require.Contains(t, string(spec), "This package contains no downloader, updater,")
	require.Contains(t, string(spec), `%check`)
	require.Contains(t, string(spec), `tea --version`)
	require.Contains(t, string(spec), `tea --help`)
	require.NotContains(t, string(spec), "%post")
	require.NotContains(t, string(spec), "curl")

	sourceLock, err := os.ReadFile(filepath.Join(root, "distro", "locks", "tea-source.toml"))
	require.NoError(t, err)
	require.Contains(t, string(sourceLock), `version = "0.15.1"`)
	require.Contains(t, string(sourceLock), `checksum_manifest_sha256 = "295347169dacd180fd920d78079e770a829f338c6bb0ae26493baa0ff4e8ac61"`)
	require.Contains(t, string(sourceLock), `license_sha256 = "a804f8028d201e1e36e44372674025f74c71f67a28c58f09991c1069726f1fd2"`)
	require.Contains(t, string(sourceLock), `sha256 = "cd4dc38e2dd051577e434ee9649793c80f1e5b3266efa901ba64b72f8d8e53a8"`)
	require.Contains(t, string(sourceLock), `sha256 = "cd7db63fd4319045b842af7843ff0bfbc247cd73b8e6b482a067cc4e3ce1d404"`)
}

func TestBuilderBunAndTeaInputsAreSelectedForTheNativeArchitecture(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	builderFetcher, err := os.ReadFile(filepath.Join(root, "scripts", "fetch-builder-tools.sh"))
	require.NoError(t, err)
	require.Contains(t, string(builderFetcher), `case "$1:$(uname -m)" in`)
	require.Contains(t, string(builderFetcher), `fetch_toolchain "$architecture" "$expected"`)
	require.NotContains(t, string(builderFetcher), "fetch_toolchain amd64 ")
	require.NotContains(t, string(builderFetcher), "fetch_toolchain arm64 ")
	teaFetcher, err := os.ReadFile(filepath.Join(root, "scripts", "fetch-tea-source.sh"))
	require.NoError(t, err)
	require.Contains(t, string(teaFetcher), `case "$(uname -m)" in`)
	require.Contains(t, string(teaFetcher), `architecture=x86_64`)
	require.Contains(t, string(teaFetcher), `architecture=aarch64`)
	require.Contains(t, string(teaFetcher), `xz --test "$archive_temporary"`)

	justfile, err := os.ReadFile(filepath.Join(root, "justfile"))
	require.NoError(t, err)
	require.Contains(t, string(justfile), "rpm architecture: (builder-tools architecture) forgejo-source bun-source tea-source")
	require.Contains(t, string(justfile), "oci architecture: (builder-tools architecture) forgejo-source bun-source tea-source")
}
