package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestCopiedTreeRejectsSpecialFilesWithoutFollowingSymlinks(t *testing.T) {
	rootPath := t.TempDir()
	require.NoError(t, unix.Mkfifo(filepath.Join(rootPath, "pipe"), 0o600))
	root, err := os.Open(rootPath)
	require.NoError(t, err)
	require.ErrorContains(t, validateOwnedTree(root, os.Getuid()), "unsupported file type")
	require.NoError(t, root.Close())

	require.NoError(t, os.Remove(filepath.Join(rootPath, "pipe")))
	outside := t.TempDir()
	require.NoError(t, unix.Mkfifo(filepath.Join(outside, "pipe"), 0o600))
	require.NoError(t, os.Symlink(outside, filepath.Join(rootPath, "source-link")))
	root, err = os.Open(rootPath)
	require.NoError(t, err)
	require.NoError(t, validateOwnedTree(root, os.Getuid()), "Git symlinks are evidence, not directories to traverse")
	require.NoError(t, root.Close())
}
