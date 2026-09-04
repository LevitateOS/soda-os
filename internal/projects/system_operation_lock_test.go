package projects

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

type operationLockResult struct {
	lock io.Closer
	err  error
}

func TestOperationLockerRequiresExplicitConstruction(t *testing.T) {
	_, err := (OperationLocker{}).Shared()
	require.ErrorContains(t, err, "not constructed")
	_, err = NewOperationLocker("relative.lock", os.Getuid())
	require.ErrorContains(t, err, "absolute")
	_, err = NewOperationLocker("/run/lock/soda/workspace-operations.lock", -1)
	require.ErrorContains(t, err, "non-negative")
}

func TestWorkspaceOperationLockCoordinatesSetupAndRemoval(t *testing.T) {
	path := testOperationLockFile(t)
	firstShared, err := openWorkspaceOperationLock(path, os.Getuid(), unix.LOCK_SH)
	require.NoError(t, err)
	defer func() {
		if firstShared != nil {
			_ = firstShared.Close()
		}
	}()

	exclusiveResult := acquireOperationLock(path, unix.LOCK_EX)
	requireLockBlocked(t, exclusiveResult)
	secondSharedResult := acquireOperationLock(path, unix.LOCK_SH)
	secondShared := requireLockAcquired(t, secondSharedResult)
	require.NotNil(t, secondShared, "the helper must nest a shared lock while removal waits")
	requireLockBlocked(t, exclusiveResult)
	require.NoError(t, secondShared.Close())
	requireLockBlocked(t, exclusiveResult)
	require.NoError(t, firstShared.Close())
	firstShared = nil
	exclusive := requireLockAcquired(t, exclusiveResult)

	sharedResult := acquireOperationLock(path, unix.LOCK_SH)
	requireLockBlocked(t, sharedResult)
	require.NoError(t, exclusive.Close())
	shared := requireLockAcquired(t, sharedResult)
	require.NoError(t, shared.Close())
}

func TestWorkspaceOperationLockRejectsMutableOrSymlinkedFiles(t *testing.T) {
	root := t.TempDir()
	mutable := filepath.Join(root, "mutable.lock")
	require.NoError(t, os.WriteFile(mutable, nil, 0o600))
	require.NoError(t, os.Chmod(mutable, 0o644))
	_, err := openWorkspaceOperationLock(mutable, os.Getuid(), unix.LOCK_SH)
	require.ErrorContains(t, err, "mode 0444")

	locked := testOperationLockFile(t)
	_, err = openWorkspaceOperationLock(locked, os.Getuid()+1, unix.LOCK_SH)
	require.ErrorContains(t, err, "ownership")
	symlink := filepath.Join(root, "symlink.lock")
	require.NoError(t, os.Symlink(locked, symlink))
	_, err = openWorkspaceOperationLock(symlink, os.Getuid(), unix.LOCK_SH)
	require.Error(t, err)
}

func testOperationLockFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "workspace-operations.lock")
	require.NoError(t, os.WriteFile(path, nil, 0o600))
	require.NoError(t, os.Chmod(path, 0o444))
	return path
}

func acquireOperationLock(path string, kind int) <-chan operationLockResult {
	result := make(chan operationLockResult, 1)
	go func() {
		lock, err := openWorkspaceOperationLock(path, os.Getuid(), kind)
		result <- operationLockResult{lock: lock, err: err}
	}()
	return result
}

func requireLockBlocked(t *testing.T, result <-chan operationLockResult) {
	t.Helper()
	select {
	case acquired := <-result:
		if acquired.lock != nil {
			_ = acquired.lock.Close()
		}
		require.Fail(t, "operation lock acquired before the conflicting holder closed", "%v", acquired.err)
	case <-time.After(50 * time.Millisecond):
	}
}

func requireLockAcquired(t *testing.T, result <-chan operationLockResult) io.Closer {
	t.Helper()
	select {
	case acquired := <-result:
		require.NoError(t, acquired.err)
		require.NotNil(t, acquired.lock)
		return acquired.lock
	case <-time.After(time.Second):
		require.FailNow(t, "operation lock did not acquire after the conflicting holder closed")
		return nil
	}
}
