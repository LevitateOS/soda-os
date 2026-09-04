package workspace

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/LevitateOS/soda-os/internal/projects/catalog"
	"golang.org/x/sys/unix"
)

type setupLock struct{ file *os.File }

func (lock *setupLock) Close() error {
	return errors.Join(unix.Flock(int(lock.file.Fd()), unix.LOCK_UN), lock.file.Close())
}

// SetupLocker owns the per-primary/project serialization primitive. The root
// Projects package composes it with the shared workspace-operation lock.
type SetupLocker struct {
	runtimeRoot string
}

func NewSetupLocker(runtimeRoot string) SetupLocker {
	return SetupLocker{runtimeRoot: runtimeRoot}
}

func (locker SetupLocker) Lock(account linuxhost.Account, entry catalog.Entry) (io.Closer, error) {
	if err := entry.Validate(); err != nil {
		return nil, err
	}
	lockRoot, err := locker.ensureLockRoot(account)
	if err != nil {
		return nil, err
	}
	defer lockRoot.Close()
	lock, err := openSetupLockAt(lockRoot, account, entry.ID)
	if err != nil {
		return nil, err
	}
	if err = unix.Flock(int(lock.Fd()), unix.LOCK_EX); err != nil {
		lock.Close()
		return nil, fmt.Errorf("lock workspace setup: %w", err)
	}
	return &setupLock{file: lock}, nil
}

func openSetupLockAt(parent *os.File, account linuxhost.Account, projectID string) (*os.File, error) {
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

func (locker SetupLocker) ensureLockRoot(account linuxhost.Account) (*os.File, error) {
	userRuntime, err := locker.openRuntimeUserDirectory(account)
	if err != nil {
		return nil, err
	}
	defer userRuntime.Close()
	return ensureCallerOwnedDirectoryAt(userRuntime, "soda-projects", account, "workspace setup lock directory")
}

func (locker SetupLocker) openRuntimeUserDirectory(account linuxhost.Account) (*os.File, error) {
	runtimeRoot, err := openAbsoluteDirectoryNoSymlinks(locker.runtimeRoot)
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

func ensureCallerOwnedDirectoryAt(parent *os.File, name string, account linuxhost.Account, description string) (*os.File, error) {
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
