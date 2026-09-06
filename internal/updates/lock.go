package updates

import (
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

// Lock serializes Soda check/update requests, not native administrator commands.
// The caller supplies the production path; tests use their own temporary files.
func Lock(path string) (io.Closer, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|unix.O_NOFOLLOW, 0o600)
	if err != nil {
		return nil, err
	}
	if err = unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return nil, errors.New("another Soda update operation is running; refresh when it finishes")
		}
		return nil, fmt.Errorf("lock Soda updates: %w", err)
	}
	return operationLock{file}, nil
}

type operationLock struct{ file *os.File }

func (lock operationLock) Close() error {
	return errors.Join(unix.Flock(int(lock.file.Fd()), unix.LOCK_UN), lock.file.Close())
}
