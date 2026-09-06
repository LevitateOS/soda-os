package filelock

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func openTestLock(t *testing.T, path string) *os.File {
	t.Helper()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	require.NoError(t, err)
	t.Cleanup(func() { _ = file.Close() })
	return file
}

func TestCancelledWaitDoesNotAcquireOrRetainTheLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	held := openTestLock(t, path)
	waiting := openTestLock(t, path)
	require.NoError(t, Acquire(t.Context(), held, unix.LOCK_EX))
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- Acquire(ctx, waiting, unix.LOCK_EX) }()
	select {
	case err := <-result:
		t.Fatalf("contended lock returned before cancellation: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	cancel()
	select {
	case err := <-result:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("lock did not cancel")
	}
	require.NoError(t, held.Close())
	other := openTestLock(t, path)
	require.NoError(t, unix.Flock(int(other.Fd()), unix.LOCK_EX|unix.LOCK_NB))
}

func TestSharedHoldersExcludeWriterUntilBothRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	first, second, writer := openTestLock(t, path), openTestLock(t, path), openTestLock(t, path)
	require.NoError(t, Acquire(t.Context(), first, unix.LOCK_SH))
	require.NoError(t, Acquire(t.Context(), second, unix.LOCK_SH))
	result := make(chan error, 1)
	go func() { result <- Acquire(t.Context(), writer, unix.LOCK_EX) }()
	for _, holder := range []*os.File{first, second} {
		select {
		case err := <-result:
			t.Fatalf("writer acquired while a reader held the lock: %v", err)
		case <-time.After(50 * time.Millisecond):
		}
		require.NoError(t, holder.Close())
	}
	select {
	case err := <-result:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("writer did not acquire after readers released")
	}
}

func TestCancellationAndDescriptorErrors(t *testing.T) {
	file := openTestLock(t, filepath.Join(t.TempDir(), "lock"))
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.ErrorIs(t, Acquire(ctx, file, unix.LOCK_EX), context.Canceled)
	require.NoError(t, file.Close())
	require.ErrorIs(t, Acquire(t.Context(), file, unix.LOCK_EX), unix.EBADF)
}
