package projects

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

func openAbsoluteDirectoryNoSymlinks(path string) (*os.File, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || path == "/" {
		return nil, errors.New("directory path must be a normalized absolute path below root")
	}
	rootDescriptor, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	defer unix.Close(rootDescriptor)
	return openAbsoluteDirectoryAt(rootDescriptor, path)
}

func openAbsoluteDirectoryAt(rootDescriptor int, path string) (*os.File, error) {
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
		return nil, errors.New("open directory descriptor")
	}
	return file, nil
}

// openManagedHomeRoot follows ordinary symlinks only while resolving the
// configured home root. Fedora bootc exposes the native home root as
// /home -> var/home. Account-controlled components are opened separately by
// openDirectoryAt, which rejects symlinks.
func openManagedHomeRoot(path string) (*os.File, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || path == "/" {
		return nil, errors.New("home root must be a normalized absolute path below root")
	}
	rootDescriptor, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	defer unix.Close(rootDescriptor)
	descriptor, err := unix.Openat2(rootDescriptor, strings.TrimPrefix(path, "/"), &unix.OpenHow{
		Flags:   unix.O_RDONLY | unix.O_DIRECTORY | unix.O_CLOEXEC,
		Resolve: unix.RESOLVE_IN_ROOT | unix.RESOLVE_NO_MAGICLINKS,
	})
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(descriptor), path)
	if file == nil {
		_ = unix.Close(descriptor)
		return nil, errors.New("open home root descriptor")
	}
	return file, nil
}

func openDirectoryAt(parent *os.File, name string) (*os.File, error) {
	if name == "" || name == "." || name == ".." || strings.ContainsRune(name, '/') {
		return nil, errors.New("invalid directory component")
	}
	descriptor, err := unix.Openat2(int(parent.Fd()), name, &unix.OpenHow{
		Flags:   unix.O_RDONLY | unix.O_DIRECTORY | unix.O_CLOEXEC | unix.O_NOFOLLOW,
		Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_MAGICLINKS | unix.RESOLVE_NO_SYMLINKS,
	})
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(descriptor), name)
	if file == nil {
		_ = unix.Close(descriptor)
		return nil, errors.New("open directory descriptor")
	}
	return file, nil
}

func ensureOwnedDirectoryAt(parent *os.File, name string, account Account) (*os.File, error) {
	directory, err := openDirectoryAt(parent, name)
	if isMissing(err) {
		if err = createOwnedDirectoryAt(parent, name, account); err != nil {
			return nil, err
		}
		directory, err = openDirectoryAt(parent, name)
	}
	if err != nil {
		return nil, err
	}
	if err = validateOwnedDirectory(directory, account.UID, "workspace "+name+" directory"); err != nil {
		directory.Close()
		return nil, err
	}
	return directory, nil
}

func createOwnedDirectoryAt(parent *os.File, name string, account Account) error {
	err := unix.Mkdirat(int(parent.Fd()), name, 0o700)
	if err != nil && !errors.Is(err, unix.EEXIST) {
		return err
	}
	if errors.Is(err, unix.EEXIST) {
		return nil
	}
	return unix.Fchownat(
		int(parent.Fd()),
		name,
		account.UID,
		account.GID,
		unix.AT_SYMLINK_NOFOLLOW,
	)
}

func validateOwnedDirectory(directory *os.File, expectedUID int, description string) error {
	var stat unix.Stat_t
	if err := unix.Fstat(int(directory.Fd()), &stat); err != nil {
		return fmt.Errorf("inspect %s: %w", description, err)
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFDIR {
		return fmt.Errorf("%s is not a directory", description)
	}
	if int(stat.Uid) != expectedUID {
		return fmt.Errorf("%s has unexpected ownership", description)
	}
	return validateSafeDirectoryMode(stat.Mode, description)
}

func validateSafeDirectoryMode(mode uint32, description string) error {
	return validateSafeMode(mode, 0o500, description)
}

func validateSafeFileMode(mode uint32, description string) error {
	return validateSafeMode(mode, 0o400, description)
}

func validateSafeMode(mode, required uint32, description string) error {
	permissions := mode & 0o7777
	if permissions&0o022 != 0 || permissions&0o7000 != 0 {
		return fmt.Errorf("%s has unsafe mode %04o", description, permissions)
	}
	if permissions&required != required {
		return fmt.Errorf("%s does not grant the owner required access", description)
	}
	return nil
}

func validateDescriptorEntry(parent *os.File, name string, descriptor *os.File) error {
	var pathStat, descriptorStat unix.Stat_t
	if err := unix.Fstatat(int(parent.Fd()), name, &pathStat, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return err
	}
	if err := unix.Fstat(int(descriptor.Fd()), &descriptorStat); err != nil {
		return err
	}
	if pathStat.Mode&unix.S_IFMT != unix.S_IFDIR || descriptorStat.Mode&unix.S_IFMT != unix.S_IFDIR {
		return errors.New("directory entry no longer refers to its validated descriptor")
	}
	if pathStat.Dev != descriptorStat.Dev || pathStat.Ino != descriptorStat.Ino {
		return errors.New("directory entry no longer refers to its validated descriptor")
	}
	return nil
}

func validateRegularDescriptorEntry(parent *os.File, name string, descriptor *os.File) error {
	var pathStat, descriptorStat unix.Stat_t
	if err := unix.Fstatat(int(parent.Fd()), name, &pathStat, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return err
	}
	if err := unix.Fstat(int(descriptor.Fd()), &descriptorStat); err != nil {
		return err
	}
	if pathStat.Mode&unix.S_IFMT != unix.S_IFREG || descriptorStat.Mode&unix.S_IFMT != unix.S_IFREG {
		return errors.New("file entry no longer refers to its validated descriptor")
	}
	if pathStat.Dev != descriptorStat.Dev || pathStat.Ino != descriptorStat.Ino {
		return errors.New("file entry no longer refers to its validated descriptor")
	}
	return nil
}

func isMissing(err error) bool {
	return errors.Is(err, os.ErrNotExist) || errors.Is(err, unix.ENOENT)
}
