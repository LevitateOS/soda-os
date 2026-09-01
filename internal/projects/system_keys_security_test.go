package projects

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadAuthorizedKeysUsesOwnedNoSymlinkFile(t *testing.T) {
	account, path := secureTestAccount(t)
	platform := nativePlatformForAccount(account)

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

	read, err := nativePlatformForAccount(account).ReadAuthorizedKeys(account)
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
			_, err := nativePlatformForAccount(account).ReadAuthorizedKeys(account)
			require.ErrorContains(t, err, "does not contain a valid public key")
		})
	}
}

func TestReadAuthorizedKeysRejectsSymlinkAndUnexpectedOwner(t *testing.T) {
	account, path := secureTestAccount(t)
	platform := nativePlatformForAccount(account)
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

	_, err := nativePlatformForAccount(account).ReadAuthorizedKeys(account)
	require.ErrorContains(t, err, "exceeds the 1048576-byte limit")
}

func TestReadAuthorizedKeysRejectsSymlinkedHome(t *testing.T) {
	account, _ := secureTestAccount(t)
	link := filepath.Join(filepath.Dir(account.Home), "linked-home")
	require.NoError(t, os.Symlink(account.Home, link))
	account.Home = link

	_, err := nativePlatformForAccount(account).ReadAuthorizedKeys(account)
	require.Error(t, err)
}
