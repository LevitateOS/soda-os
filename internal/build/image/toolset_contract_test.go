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
				Name: "soda-tea", NEVRA: "soda-tea-0:0.15.1-2.fc44." + architecture,
				Source: "local-rpm", File: "soda-tea-0.15.1-2.fc44." + architecture + ".rpm",
			})
		})
	}

	spec, err := os.ReadFile(filepath.Join(root, "packaging", "rpm", "tea", "soda-tea.spec"))
	require.NoError(t, err)
	require.Contains(t, string(spec), "This package contains no downloader, updater,")
	require.Contains(t, string(spec), `%check`)
	require.Contains(t, string(spec), `tea --version`)
	require.Contains(t, string(spec), `tea --help`)
	require.Contains(t, string(spec), `--password-stdin`)
	require.Contains(t, string(spec), `--token-name`)
	require.NotContains(t, string(spec), "%post")
	require.NotContains(t, string(spec), "curl")

	sourceLock, err := os.ReadFile(filepath.Join(root, "distro", "locks", "tea-source.toml"))
	require.NoError(t, err)
	require.Contains(t, string(sourceLock), `version = "0.15.1"`)
	require.Contains(t, string(sourceLock), `commit = "f34697c5ed65928e265d6f48e16928819ce0f332"`)
	require.Contains(t, string(sourceLock), `source_sha256 = "e242dd3589c31a36320d75e0de9eefa3fa429bd9b0af89d35af8585c7f514b9c"`)
	require.Contains(t, string(sourceLock), `patch_sha256 = "cb0b10ead27cefaa47531097aec338774bf0bf9fb9e001f45127ea3c15a1f167"`)
	require.Contains(t, string(sourceLock), `license_sha256 = "a804f8028d201e1e36e44372674025f74c71f67a28c58f09991c1069726f1fd2"`)
}

func TestBuilderBunAndTeaInputsRemainPinned(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	builderFetcher, err := os.ReadFile(filepath.Join(root, "scripts", "fetch-builder-tools.sh"))
	require.NoError(t, err)
	require.Contains(t, string(builderFetcher), `case "$1:$(uname -m)" in`)
	require.Contains(t, string(builderFetcher), `lock="$repo_root/distro/platforms/$platform.toml"`)
	require.Contains(t, string(builderFetcher), `version=$(builder_value go_version)`)
	require.Contains(t, string(builderFetcher), `fetch_toolchain`)
	require.NotContains(t, string(builderFetcher), "go1.27.0")
	teaFetcher, err := os.ReadFile(filepath.Join(root, "scripts", "fetch-tea-source.sh"))
	require.NoError(t, err)
	require.Contains(t, string(teaFetcher), `source_sha256`)
	require.Contains(t, string(teaFetcher), `patch_sha256`)
	require.Contains(t, string(teaFetcher), `tar -tzf "$temporary"`)
	require.NotContains(t, string(teaFetcher), `latest`)
	buildSource, err := os.ReadFile(filepath.Join(root, "internal", "build", "image", "rpm.go"))
	require.NoError(t, err)
	require.Contains(t, string(buildSource), `make BUILDMODE=-buildvcs=false build`)

	justfile, err := os.ReadFile(filepath.Join(root, "justfile"))
	require.NoError(t, err)
	require.Contains(t, string(justfile), "rpm architecture: (builder-tools architecture) forgejo-source bun-source tea-source")
	require.Contains(t, string(justfile), "oci architecture: (builder-tools architecture) forgejo-source bun-source tea-source")
}
