package projects

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

const DefaultWorkspaceOperationLockPath = "/run/lock/soda/workspace-operations.lock"

// OperationLocker coordinates setup with destructive workspace, project, and
// human removal. It owns only this cross-domain lock, not any domain state.
type OperationLocker struct {
	path     string
	ownerUID int
}

func NewSystemOperationLocker() OperationLocker {
	return OperationLocker{path: DefaultWorkspaceOperationLockPath, ownerUID: 0}
}

func NewOperationLocker(path string, ownerUID int) (OperationLocker, error) {
	if path == "" {
		return OperationLocker{}, errors.New("workspace operation lock path is required")
	}
	if !filepath.IsAbs(path) {
		return OperationLocker{}, errors.New("workspace operation lock path must be absolute")
	}
	if ownerUID < 0 {
		return OperationLocker{}, errors.New("workspace operation lock owner UID must be non-negative")
	}
	return OperationLocker{path: path, ownerUID: ownerUID}, nil
}

type workspaceOperationLock struct {
	file *os.File
}

func (lock *workspaceOperationLock) Close() error {
	unlockErr := unix.Flock(int(lock.file.Fd()), unix.LOCK_UN)
	closeErr := lock.file.Close()
	return errors.Join(unlockErr, closeErr)
}

func (locker OperationLocker) Shared() (io.Closer, error) {
	return locker.lock(unix.LOCK_SH)
}

func (locker OperationLocker) Exclusive() (io.Closer, error) {
	return locker.lock(unix.LOCK_EX)
}

func (locker OperationLocker) lock(kind int) (io.Closer, error) {
	if locker.path == "" {
		return nil, errors.New("workspace operation locker was not constructed")
	}
	return openWorkspaceOperationLock(locker.path, locker.ownerUID, kind)
}

func openWorkspaceOperationLock(path string, ownerUID, kind int) (io.Closer, error) {
	parent, err := openWorkspaceOperationLockDirectory(filepath.Dir(path))
	if err != nil {
		return nil, fmt.Errorf("open workspace operation lock directory: %w", err)
	}
	defer parent.Close()
	descriptor, err := unix.Openat2(int(parent.Fd()), filepath.Base(path), &unix.OpenHow{
		Flags:   unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NOFOLLOW,
		Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_MAGICLINKS | unix.RESOLVE_NO_SYMLINKS,
	})
	if err != nil {
		return nil, fmt.Errorf("open workspace operation lock: %w", err)
	}
	file := os.NewFile(uintptr(descriptor), path)
	if file == nil {
		_ = unix.Close(descriptor)
		return nil, errors.New("open workspace operation lock descriptor")
	}
	if err = validateWorkspaceOperationLock(file, ownerUID); err != nil {
		file.Close()
		return nil, err
	}
	if err = unix.Flock(descriptor, kind); err != nil {
		file.Close()
		return nil, fmt.Errorf("lock workspace operations: %w", err)
	}
	return &workspaceOperationLock{file: file}, nil
}

func validateWorkspaceOperationLock(file *os.File, ownerUID int) error {
	var stat unix.Stat_t
	if err := unix.Fstat(int(file.Fd()), &stat); err != nil {
		return fmt.Errorf("inspect workspace operation lock: %w", err)
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFREG {
		return errors.New("workspace operation lock is not a regular file")
	}
	if int(stat.Uid) != ownerUID {
		return errors.New("workspace operation lock has unexpected ownership")
	}
	if stat.Mode&0o777 != 0o444 {
		return errors.New("workspace operation lock must have mode 0444")
	}
	return nil
}

func openWorkspaceOperationLockDirectory(path string) (*os.File, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || path == "/" {
		return nil, errors.New("workspace operation lock directory must be a normalized absolute path below root")
	}
	rootDescriptor, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	defer unix.Close(rootDescriptor)
	descriptor, err := unix.Openat2(rootDescriptor, strings.TrimPrefix(path, "/"), &unix.OpenHow{
		Flags:   unix.O_RDONLY | unix.O_DIRECTORY | unix.O_CLOEXEC | unix.O_NOFOLLOW,
		Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_MAGICLINKS | unix.RESOLVE_NO_SYMLINKS,
	})
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(descriptor), path)
	if file == nil {
		_ = unix.Close(descriptor)
		return nil, errors.New("open workspace operation lock directory descriptor")
	}
	return file, nil
}
