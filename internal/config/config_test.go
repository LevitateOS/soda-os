package config

import (
	"path/filepath"
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
