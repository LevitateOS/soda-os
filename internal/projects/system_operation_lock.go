package projects

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

const DefaultWorkspaceOperationLockPath = "/run/lock/soda/workspace-operations.lock"

type workspaceOperationLock struct {
	file *os.File
}

func (lock *workspaceOperationLock) Close() error {
	unlockErr := unix.Flock(int(lock.file.Fd()), unix.LOCK_UN)
	closeErr := lock.file.Close()
	return errors.Join(unlockErr, closeErr)
}

func (platform *NativePlatform) WorkspaceOperationSharedLock() (io.Closer, error) {
	return platform.workspaceOperationLock(unix.LOCK_SH)
}

func (platform *NativePlatform) WorkspaceOperationExclusiveLock() (io.Closer, error) {
	return platform.workspaceOperationLock(unix.LOCK_EX)
}

func (platform *NativePlatform) workspaceOperationLock(kind int) (io.Closer, error) {
	path := platform.OperationLockPath
	if path == "" {
		path = DefaultWorkspaceOperationLockPath
	}
	return openWorkspaceOperationLock(path, platform.OperationLockOwnerUID, kind)
}

func openWorkspaceOperationLock(path string, ownerUID, kind int) (io.Closer, error) {
	parent, err := openAbsoluteDirectoryNoSymlinks(filepath.Dir(path))
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
	stat, err := descriptorStat(file)
	if err != nil {
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
