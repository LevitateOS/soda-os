package linuxhost

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/sys/unix"
)

const (
	maximumAuthorizedKeysSize = 1 << 20
	stagedAuthorizedKeysName  = ".soda-authorized-keys.tmp"
)

var ErrAuthorizedKeysPublished = errors.New("authorized_keys is published or has ambiguous provenance")

func CanonicalAuthorizedKey(input string) (string, error) {
	publicKey, _, options, rest, err := ssh.ParseAuthorizedKey([]byte(input))
	if err != nil || len(options) != 0 || len(bytes.TrimSpace(rest)) != 0 {
		return "", errors.New("authorized key must contain exactly one valid OpenSSH public key")
	}
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(publicKey))), nil
}

func (native *Native) ReadAuthorizedKeys(account Account) ([]byte, error) {
	path := filepath.Join(account.Home, ".ssh", "authorized_keys")
	keyFile, err := native.openAuthorizedKeys(account, path)
	if err != nil {
		return nil, err
	}
	defer keyFile.Close()
	contents, err := readBoundedAuthorizedKeys(keyFile, path)
	if err != nil {
		return nil, err
	}
	if err = validateAuthorizedKeyContents(contents, path); err != nil {
		return nil, err
	}
	return contents, nil
}

func (native *Native) openAuthorizedKeys(account Account, path string) (*os.File, error) {
	home, err := native.OpenAccountHome(account)
	if err != nil {
		return nil, fmt.Errorf("open account home: %w", err)
	}
	defer home.Close()
	if err = validateOwnedDirectory(home, account.UID, "account home"); err != nil {
		return nil, err
	}
	sshDirectory, err := openDirectoryAt(home, ".ssh")
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", filepath.Dir(path), err)
	}
	defer sshDirectory.Close()
	if err = validateOwnedDirectory(sshDirectory, account.UID, "account SSH directory"); err != nil {
		return nil, err
	}
	return openOwnedAuthorizedKeys(sshDirectory, account.UID, path)
}

func openOwnedAuthorizedKeys(sshDirectory *os.File, expectedUID int, path string) (*os.File, error) {
	descriptor, err := unix.Openat2(int(sshDirectory.Fd()), "authorized_keys", &unix.OpenHow{
		Flags: unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NOFOLLOW, Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_MAGICLINKS | unix.RESOLVE_NO_SYMLINKS,
	})
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	keyFile := os.NewFile(uintptr(descriptor), path)
	if keyFile == nil {
		_ = unix.Close(descriptor)
		return nil, fmt.Errorf("open %s", path)
	}
	if err = validateAuthorizedKeyFile(keyFile, expectedUID, path); err != nil {
		keyFile.Close()
		return nil, err
	}
	return keyFile, nil
}

func validateAuthorizedKeyFile(keyFile *os.File, expectedUID int, path string) error {
	stat, err := descriptorStat(keyFile)
	if err != nil {
		return fmt.Errorf("inspect %s: %w", path, err)
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFREG {
		return fmt.Errorf("%s is not a regular file", path)
	}
	if int(stat.Uid) != expectedUID {
		return fmt.Errorf("%s has unexpected ownership", path)
	}
	if err = validateSafeFileMode(stat.Mode, path); err != nil {
		return err
	}
	if stat.Size < 0 || stat.Size > maximumAuthorizedKeysSize {
		return authorizedKeysSizeError(path)
	}
	return nil
}

func readBoundedAuthorizedKeys(keyFile *os.File, path string) ([]byte, error) {
	contents, err := io.ReadAll(io.LimitReader(keyFile, maximumAuthorizedKeysSize+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(contents) > maximumAuthorizedKeysSize {
		return nil, authorizedKeysSizeError(path)
	}
	return contents, nil
}

func authorizedKeysSizeError(path string) error {
	return fmt.Errorf("%s exceeds the %d-byte limit", path, maximumAuthorizedKeysSize)
}

func validateAuthorizedKeyContents(contents []byte, path string) error {
	if _, _, _, _, err := ssh.ParseAuthorizedKey(contents); err != nil {
		return fmt.Errorf("%s does not contain a valid public key: %w", path, err)
	}
	return nil
}

func (native *Native) InstallAuthorizedKeys(account Account, contents []byte) error {
	path := filepath.Join(account.Home, ".ssh", "authorized_keys")
	if err := validateAuthorizedKeyContents(contents, path); err != nil {
		return err
	}
	home, err := native.OpenAccountHome(account)
	if err != nil {
		return fmt.Errorf("open account home: %w", err)
	}
	defer home.Close()
	if err = validateOwnedDirectory(home, account.UID, "account home"); err != nil {
		return err
	}
	sshDirectory, err := ensureOwnedDirectoryAt(home, ".ssh", account)
	if err != nil {
		return fmt.Errorf("prepare account SSH directory: %w", err)
	}
	defer sshDirectory.Close()
	return native.installAuthorizedKeysAt(home, sshDirectory, account, contents, path)
}

func (native *Native) installAuthorizedKeysAt(home, sshDirectory *os.File, account Account, contents []byte, path string) (returnErr error) {
	keyFile, err := createStagedAuthorizedKeysAt(sshDirectory, path)
	if err != nil {
		return err
	}
	defer keyFile.Close()
	published := false
	defer func() {
		if !published {
			returnErr = errors.Join(returnErr, cleanupStagedAuthorizedKeys(sshDirectory, keyFile))
		}
	}()
	if err = writeAuthorizedKeys(keyFile, contents, path); err != nil {
		return err
	}
	if err = ownStagedAuthorizedKeys(keyFile, account, path); err != nil {
		return err
	}
	if err = native.relabelAuthorizedKeys(home, sshDirectory, keyFile, account); err != nil {
		return err
	}
	published, err = publishAuthorizedKeys(sshDirectory, keyFile)
	return err
}

func createStagedAuthorizedKeysAt(parent *os.File, path string) (*os.File, error) {
	descriptor, err := unix.Openat2(int(parent.Fd()), stagedAuthorizedKeysName, &unix.OpenHow{
		Flags: unix.O_CREAT | unix.O_EXCL | unix.O_WRONLY | unix.O_CLOEXEC | unix.O_NOFOLLOW, Mode: 0o600,
		Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_MAGICLINKS | unix.RESOLVE_NO_SYMLINKS,
	})
	if err != nil {
		return nil, fmt.Errorf("stage %s: %w", path, err)
	}
	keyFile := os.NewFile(uintptr(descriptor), stagedAuthorizedKeysName)
	if keyFile == nil {
		_ = unix.Close(descriptor)
		return nil, fmt.Errorf("stage %s", path)
	}
	if err = validateRegularFileDescriptor(keyFile, path); err != nil {
		keyFile.Close()
		return nil, err
	}
	return keyFile, nil
}

func validateRegularFileDescriptor(file *os.File, description string) error {
	stat, err := descriptorStat(file)
	if err != nil {
		return fmt.Errorf("inspect %s: %w", description, err)
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFREG {
		return fmt.Errorf("%s is not a regular file", description)
	}
	return validateSafeFileMode(stat.Mode, description)
}

func writeAuthorizedKeys(keyFile *os.File, contents []byte, path string) error {
	if _, err := io.Copy(keyFile, bytes.NewReader(contents)); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := keyFile.Sync(); err != nil {
		return fmt.Errorf("sync %s: %w", path, err)
	}
	return nil
}

func ownStagedAuthorizedKeys(keyFile *os.File, account Account, path string) error {
	if err := unix.Fchown(int(keyFile.Fd()), account.UID, account.GID); err != nil {
		return fmt.Errorf("own %s: %w", path, err)
	}
	return validateOwnedRegularFile(keyFile, account.UID, path)
}

func validateOwnedRegularFile(file *os.File, expectedUID int, description string) error {
	stat, err := descriptorStat(file)
	if err != nil {
		return fmt.Errorf("inspect %s: %w", description, err)
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFREG {
		return fmt.Errorf("%s is not a regular file", description)
	}
	if int(stat.Uid) != expectedUID {
		return fmt.Errorf("%s has unexpected ownership", description)
	}
	if stat.Nlink != 1 {
		return fmt.Errorf("%s has unexpected link count", description)
	}
	return validateSafeFileMode(stat.Mode, description)
}

func (native *Native) relabelAuthorizedKeys(home, sshDirectory, keyFile *os.File, account Account) error {
	if err := validateDescriptorEntry(home, ".ssh", sshDirectory, unix.S_IFDIR); err != nil {
		return fmt.Errorf("validate account SSH pathname: %w", err)
	}
	if err := validateDescriptorEntry(sshDirectory, stagedAuthorizedKeysName, keyFile, unix.S_IFREG); err != nil {
		return fmt.Errorf("validate account authorized_keys pathname: %w", err)
	}
	result, err := native.run(context.Background(), "/usr/sbin/restorecon", "-R", filepath.Join(account.Home, ".ssh"))
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("restorecon failed: %s", strings.TrimSpace(result.Stderr))
	}
	if err = validateDescriptorEntry(home, ".ssh", sshDirectory, unix.S_IFDIR); err != nil {
		return fmt.Errorf("validate relabeled account SSH pathname: %w", err)
	}
	return validateDescriptorEntry(sshDirectory, stagedAuthorizedKeysName, keyFile, unix.S_IFREG)
}

func publishAuthorizedKeys(sshDirectory, keyFile *os.File) (bool, error) {
	if err := validateDescriptorEntry(sshDirectory, stagedAuthorizedKeysName, keyFile, unix.S_IFREG); err != nil {
		return false, fmt.Errorf("validate staged authorized_keys pathname: %w", err)
	}
	err := unix.Renameat2(int(sshDirectory.Fd()), stagedAuthorizedKeysName, int(sshDirectory.Fd()), "authorized_keys", unix.RENAME_NOREPLACE)
	if err != nil {
		if errors.Is(err, unix.EEXIST) {
			return false, errors.Join(ErrAuthorizedKeysPublished, fmt.Errorf("publish authorized_keys: %w", err))
		}
		return false, fmt.Errorf("publish authorized_keys: %w", err)
	}
	if err = unix.Fsync(int(sshDirectory.Fd())); err != nil {
		return true, errors.Join(ErrAuthorizedKeysPublished, fmt.Errorf("sync account SSH directory: %w", err))
	}
	if err = validateDescriptorEntry(sshDirectory, "authorized_keys", keyFile, unix.S_IFREG); err != nil {
		return true, errors.Join(ErrAuthorizedKeysPublished, fmt.Errorf("validate published authorized_keys pathname: %w", err))
	}
	return true, nil
}

func cleanupStagedAuthorizedKeys(sshDirectory, keyFile *os.File) error {
	if err := validateDescriptorEntry(sshDirectory, stagedAuthorizedKeysName, keyFile, unix.S_IFREG); err != nil {
		return fmt.Errorf("retain ambiguous staged authorized_keys: %w", err)
	}
	if err := unix.Unlinkat(int(sshDirectory.Fd()), stagedAuthorizedKeysName, 0); err != nil {
		return fmt.Errorf("remove staged authorized_keys: %w", err)
	}
	return nil
}
