package projects

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func validateOwnedTreeDescriptor(root *os.File, expectedUID int) error {
	return validateTreeDescriptor(root, expectedUID)
}

func validateTreeDescriptor(root *os.File, expectedUID int) error {
	entries, err := root.ReadDir(-1)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err = validateTreeEntry(root, entry.Name(), expectedUID); err != nil {
			return err
		}
	}
	return nil
}

func validateTreeEntry(root *os.File, name string, expectedUID int) error {
	stat, err := entryStat(root, name)
	if err != nil {
		return err
	}
	if int(stat.Uid) != expectedUID {
		return fmt.Errorf("entry %s has unexpected ownership", name)
	}
	switch stat.Mode & unix.S_IFMT {
	case unix.S_IFDIR:
		return validateTreeDirectory(root, name, expectedUID)
	case unix.S_IFREG, unix.S_IFLNK:
		return nil
	default:
		return fmt.Errorf("entry %s has unsupported file type", name)
	}
}

func validateTreeDirectory(root *os.File, name string, expectedUID int) error {
	child, err := openDirectoryAt(root, name)
	if err != nil {
		return err
	}
	defer child.Close()
	return validateTreeDescriptor(child, expectedUID)
}

func validateGitDirectoryAt(checkout *os.File, expectedUID int, description string) error {
	gitDirectory, err := openDirectoryAt(checkout, ".git")
	if err != nil {
		return fmt.Errorf("open %s: %w", description, err)
	}
	defer gitDirectory.Close()
	return validateOwnedDirectory(gitDirectory, expectedUID, description)
}

func descriptorStat(file *os.File) (unix.Stat_t, error) {
	var stat unix.Stat_t
	err := unix.Fstat(int(file.Fd()), &stat)
	return stat, err
}

func entryStat(parent *os.File, name string) (unix.Stat_t, error) {
	var stat unix.Stat_t
	err := unix.Fstatat(int(parent.Fd()), name, &stat, unix.AT_SYMLINK_NOFOLLOW)
	return stat, err
}
