package updates

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLockRefusesContentionAndReleases(t *testing.T) {
	path := filepath.Join(t.TempDir(), "updates.lock")
	lock, err := Lock(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = lock.Close() })
	_, err = Lock(path)
	require.ErrorContains(t, err, "another Soda update operation is running")
	require.NoError(t, lock.Close())
	next, err := Lock(path)
	require.NoError(t, err)
	require.NoError(t, next.Close())
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestLockRefusesSymlinksAndMissingDirectories(t *testing.T) {
	root := t.TempDir()
	_, err := Lock(filepath.Join(root, "absent", "updates.lock"))
	require.Error(t, err)
	target := filepath.Join(root, "target")
	require.NoError(t, os.WriteFile(target, []byte("preserve"), 0o600))
	link := filepath.Join(root, "updates.lock")
	require.NoError(t, os.Symlink(target, link))
	_, err = Lock(link)
	require.Error(t, err)
	contents, err := os.ReadFile(target)
	require.NoError(t, err)
	require.Equal(t, "preserve", string(contents))
}
