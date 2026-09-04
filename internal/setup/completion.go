package setup

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

const completionContents = "dismissed\n"

type FileCompletion struct {
	Path string
}

func (completion FileCompletion) path() string {
	if completion.Path == "" {
		return DefaultCompletionPath
	}
	return completion.Path
}

func (completion FileCompletion) Dismissed() (bool, error) {
	file, err := os.Open(completion.path())
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer file.Close()
	if err = validateCompletionFile(file); err != nil {
		return false, err
	}
	contents, err := io.ReadAll(io.LimitReader(file, int64(len(completionContents)+1)))
	if err != nil {
		return false, err
	}
	if string(contents) != completionContents {
		return false, errors.New("Soda Setup dismissal file has invalid contents")
	}
	return true, nil
}

func (completion FileCompletion) Mark() error {
	path := completion.path()
	if dismissed, err := completion.Dismissed(); err != nil || dismissed {
		return err
	}
	parent, err := os.Open(filepath.Dir(path))
	if err != nil {
		return fmt.Errorf("open completion directory: %w", err)
	}
	defer parent.Close()
	descriptor, err := unix.Openat(int(parent.Fd()), filepath.Base(path), unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0o600)
	if err != nil {
		if errors.Is(err, unix.EEXIST) {
			_, validationErr := completion.Dismissed()
			return validationErr
		}
		return err
	}
	file := os.NewFile(uintptr(descriptor), path)
	if file == nil {
		_ = unix.Close(descriptor)
		return errors.New("open Soda Setup dismissal descriptor")
	}
	defer file.Close()
	if _, err = io.WriteString(file, completionContents); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	if err = validateCompletionFile(file); err != nil {
		return err
	}
	return parent.Sync()
}

func validateCompletionFile(file *os.File) error {
	return validateRootFile(file, 0o600, "Soda Setup dismissal file")
}
