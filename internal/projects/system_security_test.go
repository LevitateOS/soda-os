package projects

import (
	"bytes"
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
	"golang.org/x/sys/unix"
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

func TestReadAuthorizedKeysUsesOwnedNoSymlinkFile(t *testing.T) {
	account, path := secureTestAccount(t)
	platform := &NativePlatform{}

	contents, err := platform.ReadAuthorizedKeys(account)
	require.NoError(t, err)
	require.Equal(t, testFileContents(t, path), contents)

	require.NoError(t, os.Chmod(path, 0o622))
	_, err = platform.ReadAuthorizedKeys(account)
	require.ErrorContains(t, err, "unsafe mode")
}

func TestReadAuthorizedKeysAcceptsAndPreservesOpenSSHIgnoredLines(t *testing.T) {
	account, path := secureTestAccount(t)
	contents := append([]byte("# primary key follows\n\nignored-before\n"), testAuthorizedKey(t)...)
	contents = append(contents, []byte("ssh-ed25519 not-base64 ignored-after\n# retained comment\n")...)
	require.NoError(t, os.WriteFile(path, contents, 0o600))

	read, err := (&NativePlatform{}).ReadAuthorizedKeys(account)
	require.NoError(t, err)
	require.Equal(t, contents, read)
}

func TestReadAuthorizedKeysRejectsFileWithoutValidKey(t *testing.T) {
	account, path := secureTestAccount(t)
	for name, contents := range map[string][]byte{
		"empty":          nil,
		"comments":       []byte("# comment\n\n  # another comment\n"),
		"malformed keys": []byte("ignored-line\nssh-ed25519 not-base64\n"),
	} {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, os.WriteFile(path, contents, 0o600))
			_, err := (&NativePlatform{}).ReadAuthorizedKeys(account)
			require.ErrorContains(t, err, "does not contain a valid public key")
		})
	}
}

func TestReadAuthorizedKeysRejectsSymlinkAndUnexpectedOwner(t *testing.T) {
	account, path := secureTestAccount(t)
	platform := &NativePlatform{}
	realPath := filepath.Join(filepath.Dir(path), "keys")
	require.NoError(t, os.Rename(path, realPath))
	require.NoError(t, os.Symlink(realPath, path))

	_, err := platform.ReadAuthorizedKeys(account)
	require.Error(t, err)

	require.NoError(t, os.Remove(path))
	require.NoError(t, os.Rename(realPath, path))
	account.UID++
	_, err = platform.ReadAuthorizedKeys(account)
	require.ErrorContains(t, err, "unexpected ownership")
}

func TestReadAuthorizedKeysIsBounded(t *testing.T) {
	account, path := secureTestAccount(t)
	require.NoError(t, os.WriteFile(path, bytes.Repeat([]byte{'x'}, (1<<20)+1), 0o600))

	_, err := (&NativePlatform{}).ReadAuthorizedKeys(account)
	require.ErrorContains(t, err, "exceeds the 1048576-byte limit")
}

func TestReadAuthorizedKeysRejectsSymlinkedHome(t *testing.T) {
	account, _ := secureTestAccount(t)
	link := filepath.Join(filepath.Dir(account.Home), "linked-home")
	require.NoError(t, os.Symlink(account.Home, link))
	account.Home = link

	_, err := (&NativePlatform{}).ReadAuthorizedKeys(account)
	require.Error(t, err)
}

func TestPrepareStagingValidatesOwnershipAndAddsOnlyReadTraversal(t *testing.T) {
	root := t.TempDir()
	account := Account{Username: "alice", UID: os.Getuid(), GID: os.Getgid()}
	platform := &NativePlatform{RuntimeRoot: filepath.Join(root, "run")}
	checkout := platform.StagingPath(account, "site")
	require.NoError(t, os.MkdirAll(filepath.Join(checkout, ".git", "objects"), 0o700))
	file := filepath.Join(checkout, ".git", "config")
	require.NoError(t, os.WriteFile(file, []byte("config"), 0o600))
	require.NoError(t, os.Symlink("config", filepath.Join(checkout, ".git", "config-link")))

	require.NoError(t, platform.PrepareStaging(account, "site"))
	checkoutInfo, err := os.Stat(checkout)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o005), checkoutInfo.Mode().Perm()&0o005)
	fileInfo, err := os.Stat(file)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o004), fileInfo.Mode().Perm()&0o004)
	linkInfo, err := os.Lstat(filepath.Join(checkout, ".git", "config-link"))
	require.NoError(t, err)
	require.True(t, linkInfo.Mode()&os.ModeSymlink != 0)
}

func TestPrepareStagingRejectsSymlinkedCheckoutAndSpecialFiles(t *testing.T) {
	root := t.TempDir()
	account := Account{Username: "alice", UID: os.Getuid(), GID: os.Getgid()}
	platform := &NativePlatform{RuntimeRoot: filepath.Join(root, "run")}
	checkout := platform.StagingPath(account, "site")
	require.NoError(t, os.MkdirAll(filepath.Dir(checkout), 0o700))
	target := filepath.Join(root, "target")
	require.NoError(t, os.Mkdir(target, 0o700))
	require.NoError(t, os.Symlink(target, checkout))
	require.Error(t, platform.PrepareStaging(account, "site"))

	require.NoError(t, os.Remove(checkout))
	require.NoError(t, os.Mkdir(checkout, 0o700))
	require.NoError(t, unix.Mkfifo(filepath.Join(checkout, "pipe"), 0o600))
	require.ErrorContains(t, platform.PrepareStaging(account, "site"), "unsupported file type")
}

func TestStagingMutationRejectsSymlinkedAncestor(t *testing.T) {
	root := t.TempDir()
	account := Account{Username: "alice", UID: os.Getuid(), GID: os.Getgid()}
	platform := &NativePlatform{RuntimeRoot: filepath.Join(root, "run")}
	userRuntime := filepath.Join(platform.RuntimeRoot, fmt.Sprint(account.UID))
	require.NoError(t, os.MkdirAll(userRuntime, 0o700))
	target := filepath.Join(root, "target")
	require.NoError(t, os.MkdirAll(filepath.Join(target, "site"), 0o700))
	sentinel := filepath.Join(target, "site", "keep")
	require.NoError(t, os.WriteFile(sentinel, []byte("keep"), 0o600))
	require.NoError(t, os.Symlink(target, filepath.Join(userRuntime, "soda-projects")))

	require.Error(t, platform.ResetStaging(account, "site"))
	require.Error(t, platform.CleanupStaging(account, "site"))
	require.FileExists(t, sentinel)
}

func TestStagingDescriptorRemainsConfinedAfterAncestorReplacement(t *testing.T) {
	root := t.TempDir()
	account := Account{Username: "alice", UID: os.Getuid(), GID: os.Getgid()}
	platform := &NativePlatform{RuntimeRoot: filepath.Join(root, "run")}
	userRuntime := filepath.Join(platform.RuntimeRoot, fmt.Sprint(account.UID))
	require.NoError(t, os.MkdirAll(userRuntime, 0o700))
	stagingRoot, err := platform.ensureStagingRoot(account)
	require.NoError(t, err)
	defer stagingRoot.Close()

	detached := filepath.Join(userRuntime, "detached")
	require.NoError(t, os.Rename(filepath.Join(userRuntime, "soda-projects"), detached))
	target := filepath.Join(root, "target")
	require.NoError(t, os.MkdirAll(target, 0o700))
	sentinel := filepath.Join(target, "keep")
	require.NoError(t, os.WriteFile(sentinel, []byte("keep"), 0o600))
	require.NoError(t, os.Symlink(target, filepath.Join(userRuntime, "soda-projects")))

	require.NoError(t, resetStagingAt(stagingRoot, account, "site"))
	require.DirExists(t, filepath.Join(detached, "site"))
	require.FileExists(t, sentinel)
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
	account := Account{Username: "soda-w-example", UID: os.Getuid(), GID: os.Getgid(), Home: filepath.Join(root, "workspace")}
	projects := filepath.Join(account.Home, "Projects")
	checkout := filepath.Join(projects, "site")
	require.NoError(t, os.MkdirAll(filepath.Join(checkout, ".git"), 0o700))
	platform := &NativePlatform{}
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
	account := Account{Username: "soda-w-example", UID: os.Getuid(), GID: os.Getgid(), Home: filepath.Join(root, "workspace")}
	require.NoError(t, os.MkdirAll(filepath.Join(account.Home, "Projects", "site", ".git"), 0o700))

	ready, err := (&NativePlatform{}).WorkspaceReady(account, "site")
	require.NoError(t, err)
	require.True(t, ready)

	require.NoError(t, os.MkdirAll(filepath.Join(account.Home, ".ssh"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(account.Home, ".ssh", "authorized_keys"), []byte("user-managed malformed contents\n"), 0o600))
	ready, err = (&NativePlatform{}).WorkspaceReady(account, "site")
	require.NoError(t, err)
	require.True(t, ready)
}

func TestInstallAuthorizedKeysIsDescriptorSafeAndOneTime(t *testing.T) {
	root := t.TempDir()
	account := Account{Username: "soda-w-example", UID: os.Getuid(), GID: os.Getgid(), Home: filepath.Join(root, "workspace")}
	require.NoError(t, os.Mkdir(account.Home, 0o700))
	path := filepath.Join(account.Home, ".ssh", "authorized_keys")
	stagedPath := filepath.Join(account.Home, ".ssh", stagedAuthorizedKeysName)
	relabelObserved := false
	platform := &NativePlatform{Runner: commandRunnerFunc(func(request Command) error {
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
