package projects

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func testAuthorizedKey(t *testing.T) []byte {
	t.Helper()
	public, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	key, err := ssh.NewPublicKey(public)
	require.NoError(t, err)
	return ssh.MarshalAuthorizedKey(key)
}

func TestWorkspaceHomeOperationsUseManagedSymlinkRoot(t *testing.T) {
	root := t.TempDir()
	realHomeRoot := filepath.Join(root, "var", "home")
	require.NoError(t, os.MkdirAll(realHomeRoot, 0o755))
	homeRoot := filepath.Join(root, "home")
	require.NoError(t, os.Symlink(filepath.Join("var", "home"), homeRoot))
	username := "soda-w-example"
	realHome := filepath.Join(realHomeRoot, username)
	require.NoError(t, os.Mkdir(realHome, 0o700))
	account := linuxhost.Account{Username: username, UID: os.Getuid(), GID: os.Getgid(), Home: filepath.Join(homeRoot, username)}
	host := &linuxhost.Native{HomeRoot: homeRoot, Runner: commandRunnerFunc(func(request linuxhost.Command) error {
		require.Equal(t, "/usr/sbin/restorecon", request.Name)
		return nil
	})}
	platform := &NativePlatform{Host: host}

	require.NoError(t, host.InstallAuthorizedKeys(account, testAuthorizedKey(t)))
	require.FileExists(t, filepath.Join(realHome, ".ssh", "authorized_keys"))
	require.NoError(t, os.MkdirAll(filepath.Join(realHome, "Projects", "site", ".git"), 0o700))
	ready, err := platform.WorkspaceReady(account, "site")
	require.NoError(t, err)
	require.True(t, ready)
	projects, err := platform.openWorkspaceProjectsForPublication(account)
	require.NoError(t, err)
	require.NoError(t, projects.Close())
}

func TestSetupLockSerializesOnePrimaryProject(t *testing.T) {
	root := t.TempDir()
	account := linuxhost.Account{Username: "alice", UID: os.Getuid(), GID: os.Getgid()}
	platform := &NativePlatform{RuntimeRoot: filepath.Join(root, "run")}
	require.NoError(t, os.MkdirAll(filepath.Join(platform.RuntimeRoot, fmt.Sprint(account.UID)), 0o700))
	first, err := platform.SetupLock(account, "site")
	require.NoError(t, err)
	acquired := make(chan io.Closer, 1)
	failed := make(chan error, 1)
	go func() {
		lock, lockErr := platform.SetupLock(account, "site")
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

func TestWorkspaceReadyRejectsSymlinkedCheckoutAndGitDirectory(t *testing.T) {
	root := t.TempDir()
	account := linuxhost.Account{Username: "soda-w-example", UID: os.Getuid(), GID: os.Getgid(), Home: filepath.Join(root, "soda-w-example")}
	projects := filepath.Join(account.Home, "Projects")
	checkout := filepath.Join(projects, "site")
	require.NoError(t, os.MkdirAll(filepath.Join(checkout, ".git"), 0o700))
	platform := &NativePlatform{Host: &linuxhost.Native{HomeRoot: root}}
	ready, err := platform.WorkspaceReady(account, "site")
	require.NoError(t, err)
	require.True(t, ready)

	require.NoError(t, os.RemoveAll(checkout))
	target := filepath.Join(root, "other")
	require.NoError(t, os.MkdirAll(filepath.Join(target, ".git"), 0o700))
	require.NoError(t, os.Symlink(target, checkout))
	_, err = platform.WorkspaceReady(account, "site")
	require.Error(t, err)

	require.NoError(t, os.Remove(checkout))
	require.NoError(t, os.Mkdir(checkout, 0o700))
	require.NoError(t, os.Symlink(filepath.Join(target, ".git"), filepath.Join(checkout, ".git")))
	_, err = platform.WorkspaceReady(account, "site")
	require.Error(t, err)
}

func TestWorkspaceReadyDoesNotDependOnCurrentAuthorizedKeys(t *testing.T) {
	root := t.TempDir()
	account := linuxhost.Account{Username: "soda-w-example", UID: os.Getuid(), GID: os.Getgid(), Home: filepath.Join(root, "soda-w-example")}
	require.NoError(t, os.MkdirAll(filepath.Join(account.Home, "Projects", "site", ".git"), 0o700))
	platform := &NativePlatform{Host: &linuxhost.Native{HomeRoot: root}}
	ready, err := platform.WorkspaceReady(account, "site")
	require.NoError(t, err)
	require.True(t, ready)

	require.NoError(t, os.MkdirAll(filepath.Join(account.Home, ".ssh"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(account.Home, ".ssh", "authorized_keys"), []byte("user-managed malformed contents\n"), 0o600))
	ready, err = platform.WorkspaceReady(account, "site")
	require.NoError(t, err)
	require.True(t, ready)
}

type commandRunnerFunc func(linuxhost.Command) error

func (run commandRunnerFunc) Run(_ context.Context, request linuxhost.Command) (linuxhost.CommandResult, error) {
	return linuxhost.CommandResult{}, run(request)
}
