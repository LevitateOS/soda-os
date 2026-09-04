package image

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStageTeaSourceUsesBuiltUpstreamBinary(t *testing.T) {
	root := t.TempDir()
	for _, directory := range []string{
		filepath.Join(root, "distro", "locks"),
		filepath.Join(root, "packaging", "rpm", "tea", "sources"),
		filepath.Join(root, ".artifacts", "build"),
	} {
		require.NoError(t, os.MkdirAll(directory, 0o755))
	}
	license := []byte("pinned Tea license\n")
	licenseDigest := sha256.Sum256(license)
	require.NoError(t, os.WriteFile(filepath.Join(root, "packaging", "rpm", "tea", "sources", "LICENSE"), license, 0o644))
	binary := []byte("native upstream Tea fixture")
	require.NoError(t, os.WriteFile(filepath.Join(root, ".artifacts", "build", "tea"), binary, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "distro", "locks", "tea-source.toml"), []byte(testTeaSourceLock(licenseDigest, "")), 0o644))

	sources := t.TempDir()
	builder := &Builder{Root: root}
	require.NoError(t, builder.stageTeaSource(sources))
	contents, err := os.ReadFile(filepath.Join(sources, "tea"))
	require.NoError(t, err)
	require.Equal(t, binary, contents)
	contents, err = os.ReadFile(filepath.Join(sources, "tea-LICENSE"))
	require.NoError(t, err)
	require.Equal(t, license, contents)
}

func TestTeaSourceLockRejectsUnknownFieldsAndSourceChanges(t *testing.T) {
	root := t.TempDir()
	lockPath := filepath.Join(root, "tea-source.toml")
	digest := sha256.Sum256([]byte("fixture"))
	require.NoError(t, os.WriteFile(lockPath, []byte(testTeaSourceLock(digest, "unexpected = true\n")), 0o644))
	_, err := readTeaSourceLock(lockPath)
	require.ErrorContains(t, err, "unknown fields")

	require.NoError(t, os.WriteFile(lockPath, []byte(testTeaSourceLock(digest, "")), 0o644))
	lock, err := readTeaSourceLock(lockPath)
	require.NoError(t, err)
	require.Equal(t, "tea-src-0.15.1.tar.gz", lock.SourceArchive)

	contents := []byte(testTeaSourceLock(digest, ""))
	contents = []byte(fmt.Sprintf("%s\nsource_sha256 = \"bad\"\n", contents))
	require.NoError(t, os.WriteFile(lockPath, contents, 0o644))
	_, err = readTeaSourceLock(lockPath)
	require.Error(t, err)
}

func testTeaSourceLock(licenseDigest [sha256.Size]byte, extra string) string {
	return fmt.Sprintf(`version = "0.15.1"
commit = "f34697c5ed65928e265d6f48e16928819ce0f332"
source_archive = "tea-src-0.15.1.tar.gz"
source_url = "https://gitea.com/gitea/tea/archive/v0.15.1.tar.gz"
source_sha256 = "%064x"
license_url = "https://gitea.com/gitea/tea/raw/tag/v0.15.1/LICENSE"
license_sha256 = "%x"
%s`, 1, licenseDigest, extra)
}
