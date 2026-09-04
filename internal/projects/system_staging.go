package projects

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"

	"golang.org/x/sys/unix"
)

type nativeSetupLock struct{ file *os.File }

func (lock *nativeSetupLock) Close() error {
	return errors.Join(unix.Flock(int(lock.file.Fd()), unix.LOCK_UN), lock.file.Close())
}

func (platform *NativePlatform) SetupLock(account Account, projectID string) (io.Closer, error) {
	if !projectIDPattern.MatchString(projectID) {
		return nil, errors.New("project id must match [a-z][a-z0-9-]{0,23}")
	}
	lockRoot, err := platform.ensureStagingRoot(account)
	if err != nil {
		return nil, err
	}
	defer lockRoot.Close()
	lock, err := openSetupLockAt(lockRoot, account, projectID)
	if err != nil {
		return nil, err
	}
	if err = unix.Flock(int(lock.Fd()), unix.LOCK_EX); err != nil {
		lock.Close()
		return nil, fmt.Errorf("lock workspace setup: %w", err)
	}
	return &nativeSetupLock{file: lock}, nil
}

func openSetupLockAt(parent *os.File, account Account, projectID string) (*os.File, error) {
	name := ".setup-" + projectID + ".lock"
	descriptor, err := unix.Openat2(int(parent.Fd()), name, &unix.OpenHow{
		Flags: unix.O_CREAT | unix.O_RDWR | unix.O_CLOEXEC | unix.O_NOFOLLOW,
		Mode:  0o600, Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_MAGICLINKS | unix.RESOLVE_NO_SYMLINKS,
	})
	if err != nil {
		return nil, fmt.Errorf("open workspace setup lock: %w", err)
	}
	lock := os.NewFile(uintptr(descriptor), name)
	if lock == nil {
		_ = unix.Close(descriptor)
		return nil, errors.New("open workspace setup lock descriptor")
	}
	if err = validateOwnedRegularFile(lock, account.UID, "workspace setup lock"); err != nil {
		lock.Close()
		return nil, err
	}
	return lock, nil
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

func (platform *NativePlatform) ensureStagingRoot(account Account) (*os.File, error) {
	userRuntime, err := platform.openRuntimeUserDirectory(account)
	if err != nil {
		return nil, err
	}
	defer userRuntime.Close()
	return ensureCallerOwnedDirectoryAt(userRuntime, "soda-projects", account, "workspace setup lock directory")
}

func (platform *NativePlatform) openRuntimeUserDirectory(account Account) (*os.File, error) {
	runtimeRoot, err := openAbsoluteDirectoryNoSymlinks(platform.RuntimeRoot)
	if err != nil {
		return nil, fmt.Errorf("open runtime root: %w", err)
	}
	defer runtimeRoot.Close()
	userRuntime, err := openDirectoryAt(runtimeRoot, strconv.Itoa(account.UID))
	if err != nil {
		return nil, fmt.Errorf("open user runtime directory: %w", err)
	}
	if err = validateOwnedDirectory(userRuntime, account.UID, "user runtime directory"); err != nil {
		userRuntime.Close()
		return nil, err
	}
	return userRuntime, nil
}

func ensureCallerOwnedDirectoryAt(parent *os.File, name string, account Account, description string) (*os.File, error) {
	directory, err := openDirectoryAt(parent, name)
	if isMissing(err) {
		if err = unix.Mkdirat(int(parent.Fd()), name, 0o700); err != nil && !errors.Is(err, unix.EEXIST) {
			return nil, err
		}
		directory, err = openDirectoryAt(parent, name)
	}
	if err != nil {
		return nil, err
	}
	if err = validateOwnedDirectory(directory, account.UID, description); err != nil {
		directory.Close()
		return nil, err
	}
	return directory, nil
}
