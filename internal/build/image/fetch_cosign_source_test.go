package image

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFetchCosignSourceVerifiesBeforeReplacingCachedInput(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"scripts", "distro/locks", ".artifacts/tools"} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, dir), 0o755))
	}
	script, err := os.ReadFile(filepath.Join("..", "..", "..", "scripts/fetch-cosign-source.sh"))
	require.NoError(t, err)
	scriptPath := filepath.Join(root, "scripts/fetch-cosign-source.sh")
	require.NoError(t, os.WriteFile(scriptPath, script, 0o755))
	archive := gzipTarball(t)
	origin := filepath.Join(root, "origin.tar.gz")
	require.NoError(t, os.WriteFile(origin, archive, 0o644))
	lock := fmt.Sprintf("source_archive = \"cosign.tar.gz\"\nsource_url = \"file://%s\"\nsource_sha256 = \"%x\"\n", origin, sha256.Sum256(archive))
	require.NoError(t, os.WriteFile(filepath.Join(root, "distro/locks/cosign-source.toml"), []byte(lock), 0o644))
	output, err := exec.Command("sh", scriptPath).CombinedOutput()
	require.NoErrorf(t, err, "%s", output)
	cached := filepath.Join(root, ".artifacts/tools/cosign.tar.gz")
	contents, err := os.ReadFile(cached)
	require.NoError(t, err)
	require.Equal(t, archive, contents)

	// A valid cached input needs neither network nor the original download.
	require.NoError(t, os.Remove(origin))
	output, err = exec.Command("sh", scriptPath).CombinedOutput()
	require.NoErrorf(t, err, "%s", output)

	// A bad download must not replace the existing input or leave a temporary.
	require.NoError(t, os.WriteFile(cached, []byte("old invalid cache"), 0o644))
	require.NoError(t, os.WriteFile(origin, []byte("bad upstream bytes"), 0o644))
	output, err = exec.Command("sh", scriptPath).CombinedOutput()
	require.Error(t, err)
	require.Contains(t, string(output), "Cosign source checksum mismatch")
	contents, err = os.ReadFile(cached)
	require.NoError(t, err)
	require.Equal(t, "old invalid cache", string(contents))
	entries, err := os.ReadDir(filepath.Dir(cached))
	require.NoError(t, err)
	require.Len(t, entries, 1)
}
