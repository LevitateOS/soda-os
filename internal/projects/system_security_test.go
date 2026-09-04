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

func secureTestAccount(t *testing.T) (Account, string) {
	t.Helper()
	home := filepath.Join(t.TempDir(), "alice")
	require.NoError(t, os.Mkdir(home, 0o700))
	require.NoError(t, os.Mkdir(filepath.Join(home, ".ssh"), 0o700))
	path := filepath.Join(home, ".ssh", "authorized_keys")
	require.NoError(t, os.WriteFile(path, testAuthorizedKey(t), 0o600))
	return Account{Username: "alice", UID: os.Getuid(), GID: os.Getgid(), Home: home}, path
}

func nativePlatformForAccount(account Account) *NativePlatform {
	return &NativePlatform{HomeRoot: filepath.Dir(account.Home)}
}

func TestManagedHomeRootAcceptsLogicalAndPhysicalNativeHomes(t *testing.T) {
	root := t.TempDir()
	realHomeRoot := filepath.Join(root, "var", "home")
	require.NoError(t, os.MkdirAll(realHomeRoot, 0o755))
	homeRoot := filepath.Join(root, "home")
	require.NoError(t, os.Symlink(filepath.Join("var", "home"), homeRoot))
	realHome := filepath.Join(realHomeRoot, "alice")
	require.NoError(t, os.Mkdir(realHome, 0o700))
	require.NoError(t, os.Mkdir(filepath.Join(realHome, ".ssh"), 0o700))
	keys := testAuthorizedKey(t)
	require.NoError(t, os.WriteFile(filepath.Join(realHome, ".ssh", "authorized_keys"), keys, 0o600))
	account := Account{
		Username: "alice",
		UID:      os.Getuid(),
		GID:      os.Getgid(),
		Home:     filepath.Join(homeRoot, "alice"),
	}
	platform := &NativePlatform{HomeRoot: homeRoot}

	contents, err := platform.ReadAuthorizedKeys(account)
	require.NoError(t, err)
	require.Equal(t, keys, contents)

	account.Home = realHome
	contents, err = platform.ReadAuthorizedKeys(account)
	require.NoError(t, err)
	require.Equal(t, keys, contents)

	account.Home = filepath.Join(root, "other", "alice")
	_, err = platform.ReadAuthorizedKeys(account)
	require.ErrorContains(t, err, "unexpected home")
	account.Home = realHome

	require.NoError(t, os.Rename(realHome, filepath.Join(realHomeRoot, "real-alice")))
	require.NoError(t, os.Symlink("real-alice", realHome))
	_, err = platform.ReadAuthorizedKeys(account)
	require.Error(t, err, "the username component must never be followed as a symlink")
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
	account := Account{
		Username: username,
		UID:      os.Getuid(),
		GID:      os.Getgid(),
		Home:     filepath.Join(homeRoot, username),
	}
	platform := &NativePlatform{
		HomeRoot: homeRoot,
		Runner: commandRunnerFunc(func(request Command) error {
			require.Equal(t, "/usr/sbin/restorecon", request.Name)
			return nil
		}),
	}

	require.NoError(t, platform.InstallAuthorizedKeys(account, testAuthorizedKey(t)))
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
	account := Account{Username: "alice", UID: os.Getuid(), GID: os.Getgid()}
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
	account := Account{Username: "soda-w-example", UID: os.Getuid(), GID: os.Getgid(), Home: filepath.Join(root, "soda-w-example")}
	projects := filepath.Join(account.Home, "Projects")
	checkout := filepath.Join(projects, "site")
	require.NoError(t, os.MkdirAll(filepath.Join(checkout, ".git"), 0o700))
	platform := &NativePlatform{HomeRoot: root}
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
	account := Account{Username: "soda-w-example", UID: os.Getuid(), GID: os.Getgid(), Home: filepath.Join(root, "soda-w-example")}
	require.NoError(t, os.MkdirAll(filepath.Join(account.Home, "Projects", "site", ".git"), 0o700))

	platform := &NativePlatform{HomeRoot: root}
	ready, err := platform.WorkspaceReady(account, "site")
	require.NoError(t, err)
	require.True(t, ready)

	require.NoError(t, os.MkdirAll(filepath.Join(account.Home, ".ssh"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(account.Home, ".ssh", "authorized_keys"), []byte("user-managed malformed contents\n"), 0o600))
	ready, err = platform.WorkspaceReady(account, "site")
	require.NoError(t, err)
	require.True(t, ready)
}

func TestInstallAuthorizedKeysIsDescriptorSafeAndOneTime(t *testing.T) {
	root := t.TempDir()
	account := Account{Username: "soda-w-example", UID: os.Getuid(), GID: os.Getgid(), Home: filepath.Join(root, "soda-w-example")}
	require.NoError(t, os.Mkdir(account.Home, 0o700))
	path := filepath.Join(account.Home, ".ssh", "authorized_keys")
	stagedPath := filepath.Join(account.Home, ".ssh", stagedAuthorizedKeysName)
	relabelObserved := false
	platform := &NativePlatform{HomeRoot: root, Runner: commandRunnerFunc(func(request Command) error {
		require.Equal(t, "/usr/sbin/restorecon", request.Name)
		require.NoFileExists(t, path)
		require.FileExists(t, stagedPath)
		relabelObserved = true
		return nil
	})}
	keys := append([]byte("# copied once from the primary account\nignored-before\n"), testAuthorizedKey(t)...)
	keys = append(keys, []byte("ignored-after\n")...)

	require.NoError(t, platform.InstallAuthorizedKeys(account, keys))
	require.True(t, relabelObserved)
	require.Equal(t, keys, testFileContents(t, path))
	require.NoFileExists(t, stagedPath)
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	platform.Runner = passwordStatusRunner{}
	err = platform.InstallAuthorizedKeys(account, testAuthorizedKey(t))
	require.ErrorIs(t, err, ErrAuthorizedKeysPublished)
	require.Equal(t, keys, testFileContents(t, path))
	require.NoFileExists(t, stagedPath)

	require.NoError(t, os.RemoveAll(filepath.Join(account.Home, ".ssh")))
	target := filepath.Join(root, "target")
	require.NoError(t, os.Mkdir(target, 0o700))
	require.NoError(t, os.Symlink(target, filepath.Join(account.Home, ".ssh")))
	require.Error(t, platform.InstallAuthorizedKeys(account, keys))
	require.NoFileExists(t, filepath.Join(target, "authorized_keys"))
}

func TestAccountHomeValidationRejectsSymlinkAndWrongOwner(t *testing.T) {
	root := t.TempDir()
	homeRoot := filepath.Join(root, "home")
	home := filepath.Join(homeRoot, "soda-w-example")
	require.NoError(t, os.MkdirAll(home, 0o700))
	platform := &NativePlatform{HomeRoot: homeRoot}
	account := Account{Username: "soda-w-example", UID: os.Getuid(), GID: os.Getgid(), Home: home}
	require.NoError(t, platform.validateAccountHome(account))

	realHome := filepath.Join(homeRoot, "real")
	require.NoError(t, os.Rename(home, realHome))
	require.NoError(t, os.Symlink(realHome, home))
	require.Error(t, platform.validateAccountHome(account))

	require.NoError(t, os.Remove(home))
	require.NoError(t, os.Mkdir(home, 0o700))
	account.UID++
	require.ErrorContains(t, platform.validateAccountHome(account), "unexpected ownership")
}

func TestIncompleteCleanupUsesValidatedHomeDescriptors(t *testing.T) {
	root := t.TempDir()
	homeRoot := filepath.Join(root, "home")
	home := filepath.Join(homeRoot, "soda-w-example")
	account := Account{Username: "soda-w-example", UID: os.Getuid(), GID: os.Getgid(), Home: home}
	platform := &NativePlatform{HomeRoot: homeRoot}
	require.NoError(t, os.MkdirAll(filepath.Join(home, "Projects", "site", ".git"), 0o700))
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".ssh"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".ssh", "authorized_keys"), testAuthorizedKey(t), 0o600))
	require.NoError(t, platform.SafeToRemoveIncomplete(account, "site"))

	require.NoError(t, os.RemoveAll(filepath.Join(home, "Projects")))
	target := filepath.Join(root, "target")
	require.NoError(t, os.MkdirAll(filepath.Join(target, "site", ".git"), 0o700))
	sentinel := filepath.Join(target, "keep")
	require.NoError(t, os.WriteFile(sentinel, []byte("keep"), 0o600))
	require.NoError(t, os.Symlink(target, filepath.Join(home, "Projects")))
	require.Error(t, platform.SafeToRemoveIncomplete(account, "site"))
	require.FileExists(t, sentinel)
}

type passwordStatusRunner struct {
	result CommandResult
}

func (runner passwordStatusRunner) Run(_ context.Context, _ Command) (CommandResult, error) {
	return runner.result, nil
}

type commandRunnerFunc func(Command) error

func (run commandRunnerFunc) Run(_ context.Context, request Command) (CommandResult, error) {
	return CommandResult{}, run(request)
}

func TestLookupAccountRejectsNumericOrAliasResolution(t *testing.T) {
	platform := &NativePlatform{Runner: commandRunnerFunc(func(request Command) error {
		if request.Name != "/usr/bin/getent" {
			return fmt.Errorf("unexpected command %s", request.Name)
		}
		return nil
	})}
	platform.Runner = lookupAliasRunner{CommandRunner: platform.Runner}
	_, err := platform.LookupAccount(context.Background(), "1000")
	require.ErrorContains(t, err, "different username")
}

type lookupAliasRunner struct {
	CommandRunner
}

func (runner lookupAliasRunner) Run(ctx context.Context, request Command) (CommandResult, error) {
	if _, err := runner.CommandRunner.Run(ctx, request); err != nil {
		return CommandResult{}, err
	}
	return CommandResult{Stdout: "alice:x:1000:1000:Alice:/home/alice:/bin/bash\n"}, nil
}

func TestNativePasswordLockValidationUsesShadowUtilsStatus(t *testing.T) {
	account := Account{Username: "soda-w-example"}
	for _, status := range []string{"L", "LK"} {
		platform := &NativePlatform{Runner: passwordStatusRunner{result: CommandResult{Stdout: account.Username + " " + status + " 2026-09-01 0 99999 7 -1\n"}}}
		require.NoError(t, platform.ValidatePasswordLocked(context.Background(), account))
	}
	platform := &NativePlatform{Runner: passwordStatusRunner{result: CommandResult{Stdout: account.Username + " P 2026-09-01 0 99999 7 -1\n"}}}
	require.ErrorContains(t, platform.ValidatePasswordLocked(context.Background(), account), "does not have a locked password")
}

func testFileContents(t *testing.T, path string) []byte {
	t.Helper()
	contents, err := os.ReadFile(path)
	require.NoError(t, err)
	return contents
}
