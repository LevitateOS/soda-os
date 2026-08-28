package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadDistro(t *testing.T) {
	spec, err := LoadDistro(filepath.Join("testdata", "soda.toml"))
	require.NoError(t, err)
	require.Equal(t, "0.2.0", spec.Identity.Version)
	require.Equal(t, DefaultDaemonSocket, spec.Paths.DaemonSocket)
}

func TestLoadDistroRejectsUnknownSchema(t *testing.T) {
	_, err := LoadDistro(filepath.Join("testdata", "unsupported-soda.toml"))
	require.EqualError(t, err, "unsupported distro schema version 3; expected 2")
}
