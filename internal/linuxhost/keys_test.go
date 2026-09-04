package linuxhost

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"testing"

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

func nativeForAccount(account Account) *Native {
	native := NewNative()
	native.HomeRoot = filepath.Dir(account.Home)
	return native
}

func TestNativeReadsOnlyOwnedSafeAuthorizedKeys(t *testing.T) {
	account, path := secureTestAccount(t)
	native := nativeForAccount(account)

	contents, err := native.ReadAuthorizedKeys(account)
	require.NoError(t, err)
	require.Equal(t, readTestFile(t, path), contents)

	require.NoError(t, os.Chmod(path, 0o622))
	_, err = native.ReadAuthorizedKeys(account)
	require.ErrorContains(t, err, "unsafe mode")

	require.NoError(t, os.Chmod(path, 0o600))
	realPath := filepath.Join(filepath.Dir(path), "keys")
	require.NoError(t, os.Rename(path, realPath))
	require.NoError(t, os.Symlink(realPath, path))
	_, err = native.ReadAuthorizedKeys(account)
	require.Error(t, err)
}

func TestNativeReadAuthorizedKeysPreservesOpenSSHIgnoredLines(t *testing.T) {
	account, path := secureTestAccount(t)
	contents := append([]byte("# primary key follows\n\nignored-before\n"), testAuthorizedKey(t)...)
	contents = append(contents, []byte("ssh-ed25519 not-base64 ignored-after\n# retained comment\n")...)
	require.NoError(t, os.WriteFile(path, contents, 0o600))

	read, err := nativeForAccount(account).ReadAuthorizedKeys(account)
	require.NoError(t, err)
	require.Equal(t, contents, read)
}

func TestNativeReadAuthorizedKeysRejectsUnexpectedFileOwner(t *testing.T) {
	account, path := secureTestAccount(t)
	keyFile, err := os.Open(path)
	require.NoError(t, err)
	defer keyFile.Close()

	err = validateAuthorizedKeyFile(keyFile, account.UID+1, path)
	require.ErrorContains(t, err, "unexpected ownership")
}

func TestNativeAuthorizedKeysReadIsBoundedAndRequiresAKey(t *testing.T) {
	account, path := secureTestAccount(t)
	native := nativeForAccount(account)

	require.NoError(t, os.WriteFile(path, bytes.Repeat([]byte{'x'}, maximumAuthorizedKeysSize+1), 0o600))
	_, err := native.ReadAuthorizedKeys(account)
	require.ErrorContains(t, err, "exceeds the 1048576-byte limit")

	for name, contents := range map[string][]byte{
		"empty":          nil,
		"comments":       []byte("# comment\n\n  # another comment\n"),
		"malformed keys": []byte("ignored-line\nssh-ed25519 not-base64\n"),
	} {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, os.WriteFile(path, contents, 0o600))
			_, err = native.ReadAuthorizedKeys(account)
			require.ErrorContains(t, err, "does not contain a valid public key")
		})
	}
}

func TestNativeInstallsAuthorizedKeysOnceThroughValidatedDescriptors(t *testing.T) {
	root := t.TempDir()
	account := Account{
		Username: "soda-w-example", UID: os.Getuid(), GID: os.Getgid(),
		Home: filepath.Join(root, "soda-w-example"),
	}
	require.NoError(t, os.Mkdir(account.Home, 0o700))
	path := filepath.Join(account.Home, ".ssh", "authorized_keys")
	stagedPath := filepath.Join(account.Home, ".ssh", stagedAuthorizedKeysName)
	relabelObserved := false
	native := NewNative()
	native.HomeRoot = root
	native.Runner = commandRunnerFunc(func(request Command) error {
		require.Equal(t, "/usr/sbin/restorecon", request.Name)
		require.NoFileExists(t, path)
		require.FileExists(t, stagedPath)
		relabelObserved = true
		return nil
	})
	keys := append([]byte("# copied once\nignored-before\n"), testAuthorizedKey(t)...)
	keys = append(keys, []byte("ignored-after\n")...)

	require.NoError(t, native.InstallAuthorizedKeys(account, keys))
	require.True(t, relabelObserved)
	require.Equal(t, keys, readTestFile(t, path))
	require.NoFileExists(t, stagedPath)
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	native.Runner = commandRunnerFunc(func(Command) error { return nil })
	err = native.InstallAuthorizedKeys(account, testAuthorizedKey(t))
	require.ErrorIs(t, err, ErrAuthorizedKeysPublished)
	require.Equal(t, keys, readTestFile(t, path))

	require.NoError(t, os.RemoveAll(filepath.Join(account.Home, ".ssh")))
	target := filepath.Join(root, "outside")
	require.NoError(t, os.Mkdir(target, 0o700))
	sentinel := filepath.Join(target, "keep")
	require.NoError(t, os.WriteFile(sentinel, []byte("keep"), 0o600))
	require.NoError(t, os.Symlink(target, filepath.Join(account.Home, ".ssh")))
	require.Error(t, native.InstallAuthorizedKeys(account, keys))
	require.NoFileExists(t, filepath.Join(target, "authorized_keys"))
	require.Equal(t, []byte("keep"), readTestFile(t, sentinel))
}

func TestNativeAccountHomeAcceptsLogicalOrPhysicalManagedRootOnly(t *testing.T) {
	root := t.TempDir()
	realRoot := filepath.Join(root, "var", "home")
	require.NoError(t, os.MkdirAll(realRoot, 0o755))
	logicalRoot := filepath.Join(root, "home")
	require.NoError(t, os.Symlink(filepath.Join("var", "home"), logicalRoot))
	realHome := filepath.Join(realRoot, "alice")
	require.NoError(t, os.Mkdir(realHome, 0o700))
	account := Account{Username: "alice", UID: os.Getuid(), GID: os.Getgid(), Home: filepath.Join(logicalRoot, "alice")}
	native := NewNative()
	native.HomeRoot = logicalRoot

	home, err := native.OpenAccountHome(account)
	require.NoError(t, err)
	require.NoError(t, home.Close())

	account.Home = realHome
	home, err = native.OpenAccountHome(account)
	require.NoError(t, err)
	require.NoError(t, home.Close())
	account.UID++
	_, err = native.OpenAccountHome(account)
	require.ErrorContains(t, err, "unexpected ownership")
	account.UID--

	account.Home = filepath.Join(root, "other", "alice")
	_, err = native.OpenAccountHome(account)
	require.ErrorContains(t, err, "unexpected home")

	account.Home = realHome
	require.NoError(t, os.Rename(realHome, filepath.Join(realRoot, "real-alice")))
	require.NoError(t, os.Symlink("real-alice", realHome))
	_, err = native.OpenAccountHome(account)
	require.Error(t, err, "the username component must not be followed as a symlink")
}

type commandRunnerFunc func(Command) error

func (run commandRunnerFunc) Run(_ context.Context, request Command) (CommandResult, error) {
	return CommandResult{}, run(request)
}

func readTestFile(t *testing.T, path string) []byte {
	t.Helper()
	contents, err := os.ReadFile(path)
	require.NoError(t, err)
	return contents
}

func TestCanonicalAuthorizedKeyRejectsOptionsAndMultipleRecords(t *testing.T) {
	key := string(testAuthorizedKey(t))
	canonical, err := CanonicalAuthorizedKey(key)
	require.NoError(t, err)
	require.Equal(t, bytes.TrimSpace([]byte(key)), []byte(canonical))

	for _, value := range []string{
		"command=\"false\" " + key,
		key + key,
		fmt.Sprintf("%s\nnot-a-key", key),
	} {
		_, err = CanonicalAuthorizedKey(value)
		require.Error(t, err)
	}
}
