package setup

import (
	"errors"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

const DefaultLockPath = "/run/lock/soda/setup.lock"

type FileLocker struct {
	Path string
}

func (locker FileLocker) Lock() (io.Closer, error) {
	path := locker.Path
	if path == "" {
		path = DefaultLockPath
	}
	descriptor, err := unix.Open(path, unix.O_RDWR|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(descriptor), path)
	if file == nil {
		_ = unix.Close(descriptor)
		return nil, errors.New("open Soda Setup lock descriptor")
	}
	if err = validateRootFile(file, 0o600, "Soda Setup lock"); err != nil {
		file.Close()
		return nil, err
	}
	if err = unix.Flock(descriptor, unix.LOCK_EX); err != nil {
		file.Close()
		return nil, err
	}
	return file, nil
}

func validateRootFile(file *os.File, mode uint32, description string) error {
	var metadata unix.Stat_t
	if err := unix.Fstat(int(file.Fd()), &metadata); err != nil {
		return err
	}
	if metadata.Mode&unix.S_IFMT != unix.S_IFREG || metadata.Uid != 0 || metadata.Gid != 0 || metadata.Mode&0o777 != mode {
		return errors.New(description + " must be a root-owned regular file with the expected mode")
	}
	return nil
}
