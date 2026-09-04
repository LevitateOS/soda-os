package workspace

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/stretchr/testify/require"
)

func TestSetupLockerSerializesOnePrimaryProject(t *testing.T) {
	runtimeRoot := filepath.Join(t.TempDir(), "run")
	account := linuxhost.Account{Username: "alice", UID: os.Getuid(), GID: os.Getgid()}
	require.NoError(t, os.MkdirAll(filepath.Join(runtimeRoot, fmt.Sprint(account.UID)), 0o700))
	locker := NewSetupLocker(runtimeRoot)
	entry := projectEntry("site")
	first, err := locker.Lock(account, entry)
	require.NoError(t, err)
	acquired := make(chan io.Closer, 1)
	failed := make(chan error, 1)
	go func() {
		lock, lockErr := locker.Lock(account, entry)
		if lockErr != nil {
			failed <- lockErr
			return
		}
		acquired <- lock
	}()
	select {
	case lockErr := <-failed:
		t.Fatalf("second setup lock failed: %v", lockErr)
	case lock := <-acquired:
		_ = lock.Close()
		t.Fatal("second setup lock acquired before the first was released")
	case <-time.After(50 * time.Millisecond):
	}
	require.NoError(t, first.Close())
	select {
	case lockErr := <-failed:
		t.Fatalf("second setup lock failed: %v", lockErr)
	case lock := <-acquired:
		require.NoError(t, lock.Close())
	case <-time.After(time.Second):
		t.Fatal("second setup lock did not acquire after release")
	}
}
