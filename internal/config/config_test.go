package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadDistro(t *testing.T) {
	spec, err := LoadDistro(filepath.Join("testdata", "soda.toml"), "aarch64")
	require.NoError(t, err)
	require.Equal(t, "0.2.0", spec.Identity.Version)
	require.Equal(t, "aarch64", spec.Identity.Architecture)
	require.Equal(t, "linux/arm64", spec.Base.Platform)
	require.Equal(t, DefaultDaemonSocket, spec.Paths.DaemonSocket)
	require.Equal(t, "LevitateOS/soda-os", spec.Distribution.GitHubRepository)
}

func TestDistroHasNoRuntimeUpdateDiscoveryMetadata(t *testing.T) {
	for _, path := range []string{filepath.Join("testdata", "soda.toml"), filepath.Join("..", "..", "distro", "soda.toml")} {
		contents, err := os.ReadFile(path)
		require.NoError(t, err)
		require.NotContains(t, string(contents), "index_url")
		require.NotContains(t, string(contents), "state_schema")
		require.Equal(t, 1, strings.Count(string(contents), "github_repository"))
	}
}

func TestLoadDistroSelectsEqualSiblingPlatforms(t *testing.T) {
	for architecture, oci := range map[string]string{"aarch64": "arm64", "x86_64": "amd64"} {
		t.Run(architecture, func(t *testing.T) {
			spec, err := LoadDistro(filepath.Join("testdata", "soda.toml"), architecture)
			require.NoError(t, err)
			require.Equal(t, architecture, spec.Identity.Architecture)
			require.Equal(t, "linux/"+oci, spec.Base.Platform)
			require.Equal(t, architecture, spec.Platform.Release.Channel)
		})
	}
}

func TestLoadDistroRejectsUnknownSchema(t *testing.T) {
	_, err := LoadDistro(filepath.Join("testdata", "unsupported-soda.toml"), "aarch64")
	require.EqualError(t, err, "unsupported distro schema version 3; expected 2")
}

func TestRequireNativeHostArchitecture(t *testing.T) {
	for _, test := range []struct {
		name, target, host, message string
	}{
		{name: "AArch64 host accepts AArch64 artifacts", target: "aarch64", host: "arm64"},
		{name: "x86-64 host accepts x86-64 artifacts", target: "x86_64", host: "amd64"},
		{name: "x86-64 host rejects AArch64 artifacts", target: "aarch64", host: "amd64", message: "Soda aarch64 artifact operations require a native arm64 host; running on amd64"},
		{name: "AArch64 host rejects x86-64 artifacts", target: "x86_64", host: "arm64", message: "Soda x86_64 artifact operations require a native amd64 host; running on arm64"},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := RequireNativeHostArchitecture(test.target, test.host)
			if test.message == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, test.message)
		})
	}
}
