package image

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFetchTeaSourceAcceptsSafeGzipArchive(t *testing.T) {
	root := t.TempDir()
	for _, directory := range []string{
		filepath.Join(root, "scripts"),
		filepath.Join(root, "distro", "locks"),
		filepath.Join(root, "packaging", "rpm", "tea", "sources"),
		filepath.Join(root, ".artifacts", "tools"),
	} {
		require.NoError(t, os.MkdirAll(directory, 0o755))
	}

	script, err := os.ReadFile(filepath.Join("..", "..", "..", "scripts", "fetch-tea-source.sh"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(root, "scripts", "fetch-tea-source.sh"), script, 0o755))
	license := []byte("Tea license fixture\n")
	require.NoError(t, os.WriteFile(filepath.Join(root, "packaging", "rpm", "tea", "sources", "LICENSE"), license, 0o644))
	archive := gzipTarball(t)
	archiveDigest := sha256.Sum256(archive)
	licenseDigest := sha256.Sum256(license)
	lock := "version = \"0.15.1\"\n" +
		"commit = \"f34697c5ed65928e265d6f48e16928819ce0f332\"\n" +
		"source_archive = \"tea.gz\"\n" +
		"source_url = \"https://mirror.example/tea.gz\"\n" +
		"source_sha256 = \"" + fmt.Sprintf("%x", archiveDigest) + "\"\n" +
		"license_url = \"https://mirror.example/LICENSE\"\n" +
		"license_sha256 = \"" + fmt.Sprintf("%x", licenseDigest) + "\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "distro", "locks", "tea-source.toml"), []byte(lock), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".artifacts", "tools", "tea.gz"), archive, 0o644))

	command := exec.Command("sh", filepath.Join(root, "scripts", "fetch-tea-source.sh"))
	output, err := command.CombinedOutput()
	require.NoErrorf(t, err, "fetch Tea source: %s", output)
}

func gzipTarball(t *testing.T) []byte {
	t.Helper()
	var output bytes.Buffer
	gzipWriter := gzip.NewWriter(&output)
	tarWriter := tar.NewWriter(gzipWriter)
	require.NoError(t, tarWriter.WriteHeader(&tar.Header{Name: "tea/README", Mode: 0o644, Size: 3}))
	_, err := tarWriter.Write([]byte("tea"))
	require.NoError(t, err)
	require.NoError(t, tarWriter.Close())
	require.NoError(t, gzipWriter.Close())
	return output.Bytes()
}
