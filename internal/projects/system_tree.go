package projects

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

type treeAccess uint8

const (
	treeOwned treeAccess = iota
	treeDerivedReadable
)

func prepareOwnedTree(root *os.File, expectedUID int) error {
	if err := prepareOwnedTreeRoot(root, expectedUID); err != nil {
		return err
	}
	entries, err := root.ReadDir(-1)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err = prepareOwnedTreeEntry(root, entry.Name(), expectedUID); err != nil {
			return err
		}
	}
	return nil
}

func prepareOwnedTreeRoot(root *os.File, expectedUID int) error {
	stat, err := descriptorStat(root)
	if err != nil {
		return err
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFDIR || int(stat.Uid) != expectedUID {
		return errors.New("completed clone root has unexpected type or ownership")
	}
	return unix.Fchmod(int(root.Fd()), stat.Mode&0o777|0o005)
}

func prepareOwnedTreeEntry(root *os.File, name string, expectedUID int) error {
	stat, err := entryStat(root, name)
	if err != nil {
		return err
	}
	if int(stat.Uid) != expectedUID {
		return fmt.Errorf("clone staging entry %s has unexpected ownership", name)
	}
	switch stat.Mode & unix.S_IFMT {
	case unix.S_IFDIR:
		return prepareOwnedTreeDirectory(root, name, expectedUID)
	case unix.S_IFREG:
		return prepareOwnedTreeFile(root, name, stat.Mode)
	case unix.S_IFLNK:
		// Git worktrees may intentionally contain symbolic links. cp -a copies
		// the link itself and never follows it.
		return nil
	default:
		return fmt.Errorf("clone staging entry %s has unsupported file type", name)
	}
}

func prepareOwnedTreeDirectory(root *os.File, name string, expectedUID int) error {
	child, err := openDirectoryAt(root, name)
	if err != nil {
		return err
	}
	defer child.Close()
	return prepareOwnedTree(child, expectedUID)
}

func prepareOwnedTreeFile(root *os.File, name string, mode uint32) error {
	descriptor, err := unix.Openat2(int(root.Fd()), name, &unix.OpenHow{
		Flags:   unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NOFOLLOW,
		Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_MAGICLINKS | unix.RESOLVE_NO_SYMLINKS,
	})
	if err != nil {
		return err
	}
	defer unix.Close(descriptor)
	return unix.Fchmod(descriptor, mode&0o777|0o004)
}

func validateReadableOwnedTree(root *os.File, expectedUID int) error {
	stat, err := descriptorStat(root)
	if err != nil {
		return err
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFDIR || int(stat.Uid) != expectedUID {
		return errors.New("completed clone root is not an owned, derived-readable directory")
	}
	if stat.Mode&0o005 != 0o005 {
		return errors.New("completed clone root is not an owned, derived-readable directory")
	}
	return validateTreeDescriptor(root, expectedUID, treeDerivedReadable)
}

func validateOwnedTreeDescriptor(root *os.File, expectedUID int) error {
	return validateTreeDescriptor(root, expectedUID, treeOwned)
}

func validateTreeDescriptor(root *os.File, expectedUID int, access treeAccess) error {
	entries, err := root.ReadDir(-1)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err = validateTreeEntry(root, entry.Name(), expectedUID, access); err != nil {
			return err
		}
	}
	return nil
}

func validateTreeEntry(root *os.File, name string, expectedUID int, access treeAccess) error {
	stat, err := entryStat(root, name)
	if err != nil {
		return err
	}
	if int(stat.Uid) != expectedUID {
		return fmt.Errorf("entry %s has unexpected ownership", name)
	}
	switch stat.Mode & unix.S_IFMT {
	case unix.S_IFDIR:
		return validateTreeDirectory(root, name, expectedUID, stat.Mode, access)
	case unix.S_IFREG:
		return validateTreeFile(name, stat.Mode, access)
	case unix.S_IFLNK:
		return nil
	default:
		return fmt.Errorf("entry %s has unsupported file type", name)
	}
}

func validateTreeDirectory(root *os.File, name string, expectedUID int, mode uint32, access treeAccess) error {
	if access == treeDerivedReadable && mode&0o005 != 0o005 {
		return fmt.Errorf("directory %s is not readable by the derived account", name)
	}
	child, err := openDirectoryAt(root, name)
	if err != nil {
		return err
	}
	defer child.Close()
	return validateTreeDescriptor(child, expectedUID, access)
}

func validateTreeFile(name string, mode uint32, access treeAccess) error {
	if access == treeDerivedReadable && mode&0o004 == 0 {
		return fmt.Errorf("file %s is not readable by the derived account", name)
	}
	return nil
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
