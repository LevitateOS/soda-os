package image

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResidualControlPlaneIsAbsent(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	for _, path := range []string{
		"buf.yaml",
		"buf.gen.yaml",
		"scripts/protobuf-generate.sh",
		"scripts/protobuf-verify.sh",
		"packaging/rpm/runtime/sources/systemd/sodad.service",
		"packaging/rpm/runtime/sources/sysusers/soda.conf",
		"packaging/rpm/runtime/sources/tmpfiles/soda.conf",
	} {
		_, err := os.Stat(filepath.Join(root, path))
		require.ErrorIs(t, err, os.ErrNotExist, path)
	}
	for _, path := range []string{"api/soda/v2", "cmd/sodad", "cmd/sodactl", "internal/daemon", "internal/grpcclient", "internal/gen/soda/v2"} {
		entries, err := os.ReadDir(filepath.Join(root, path))
		if os.IsNotExist(err) {
			continue
		}
		require.NoError(t, err, path)
		require.Empty(t, entries, path)
	}

	module, err := os.ReadFile(filepath.Join(root, "go.mod"))
	require.NoError(t, err)
	for _, dependency := range []string{"google.golang.org/grpc", "google.golang.org/protobuf", "github.com/bufbuild/buf", "buf.build/"} {
		require.NotContains(t, string(module), dependency)
	}

	for _, path := range []string{"distro/soda.toml", "internal/config/testdata/soda.toml"} {
		contents, readErr := os.ReadFile(filepath.Join(root, path))
		require.NoError(t, readErr)
		require.NotContains(t, string(contents), "daemon_socket")
		require.NotContains(t, string(contents), "[paths]")
	}

	justfile, err := os.ReadFile(filepath.Join(root, "justfile"))
	require.NoError(t, err)
	require.False(t, strings.Contains(string(justfile), "protobuf"))
}
