package image

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestForgejoSourceLockAcceptsReviewedFieldsAndRejectsDrift(t *testing.T) {
	path := filepath.Join(t.TempDir(), "forgejo-source.toml")
	valid := `version = "15.0.7"
source_archive = "forgejo-src-15.0.7.tar.gz"
url = "https://codeberg.org/forgejo/forgejo/releases/download/v15.0.7/forgejo-src-15.0.7.tar.gz"
sha256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
patch_sha256 = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
build_tags = "bindata timetzdata sqlite sqlite_unlock_notify pam"
`
	require.NoError(t, os.WriteFile(path, []byte(valid), 0o644))
	lock, err := readForgejoSourceLock(path)
	require.NoError(t, err)
	require.Equal(t, "forgejo-src-15.0.7.tar.gz", lock.SourceArchive)

	mirrored := strings.ReplaceAll(valid, "forgejo-src-15.0.7.tar.gz", "forgejo-reviewed.tar.gz")
	mirrored = strings.ReplaceAll(mirrored, "https://codeberg.org/forgejo/forgejo/releases/download/v15.0.7/forgejo-reviewed.tar.gz", "https://mirror.example/forgejo-reviewed.tar.gz")
	require.NoError(t, os.WriteFile(path, []byte(mirrored), 0o644))
	lock, err = readForgejoSourceLock(path)
	require.NoError(t, err)
	require.Equal(t, "forgejo-reviewed.tar.gz", lock.SourceArchive)

	unsafeArchive := strings.ReplaceAll(valid, "forgejo-src-15.0.7.tar.gz", "../forgejo.tar.gz")
	require.NoError(t, os.WriteFile(path, []byte(unsafeArchive), 0o644))
	_, err = readForgejoSourceLock(path)
	require.ErrorContains(t, err, "invalid")

	require.NoError(t, os.WriteFile(path, []byte(valid+"unexpected = true\n"), 0o644))
	_, err = readForgejoSourceLock(path)
	require.ErrorContains(t, err, "unknown fields")

	require.NoError(t, os.WriteFile(path, []byte("version = \"bad\"\n"), 0o644))
	_, err = readForgejoSourceLock(path)
	require.ErrorContains(t, err, "invalid")
}
