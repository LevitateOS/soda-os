package linuxhost

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

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
		Flags: unix.O_RDONLY | unix.O_DIRECTORY | unix.O_CLOEXEC, Resolve: unix.RESOLVE_IN_ROOT | unix.RESOLVE_NO_MAGICLINKS,
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

func (native *Native) validateAccountHome(account Account) error {
	home, err := native.OpenAccountHome(account)
	if err != nil {
		return err
	}
	return home.Close()
}

// OpenAccountHome returns a descriptor for the validated native account home.
// Callers can inspect their own product-owned contents without re-resolving an
// account-controlled pathname.
func (native *Native) OpenAccountHome(account Account) (*os.File, error) {
	homeRootPath, err := native.homeRoot()
	if err != nil {
		return nil, err
	}
	expectedHome := filepath.Join(homeRootPath, account.Username)
	homeMatches := account.Home == expectedHome
	if !homeMatches {
		resolvedHomeRoot, err := filepath.EvalSymlinks(homeRootPath)
		if err != nil {
			return nil, fmt.Errorf("resolve Linux home root: %w", err)
		}
		homeMatches = account.Home == filepath.Join(resolvedHomeRoot, account.Username)
	}
	if !homeMatches {
		return nil, fmt.Errorf("Linux account %s has unexpected home %s", account.Username, account.Home)
	}
	homeRoot, err := openManagedHomeRoot(homeRootPath)
	if err != nil {
		return nil, fmt.Errorf("open Linux home root: %w", err)
	}
	defer homeRoot.Close()
	home, err := openDirectoryAt(homeRoot, account.Username)
	if err != nil {
		return nil, fmt.Errorf("open Linux account home: %w", err)
	}
	if err = validateOwnedDirectory(home, account.UID, "Linux account home"); err != nil {
		home.Close()
		return nil, err
	}
	return home, nil
}

func openDirectoryAt(parent *os.File, name string) (*os.File, error) {
	if name == "" || name == "." || name == ".." || strings.ContainsRune(name, '/') {
		return nil, errors.New("invalid directory component")
	}
	descriptor, err := unix.Openat2(int(parent.Fd()), name, &unix.OpenHow{
		Flags: unix.O_RDONLY | unix.O_DIRECTORY | unix.O_CLOEXEC | unix.O_NOFOLLOW, Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_MAGICLINKS | unix.RESOLVE_NO_SYMLINKS,
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
	if err = validateOwnedDirectory(directory, account.UID, "account "+name+" directory"); err != nil {
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
	return unix.Fchownat(int(parent.Fd()), name, account.UID, account.GID, unix.AT_SYMLINK_NOFOLLOW)
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
	return validateSafeMode(stat.Mode, 0o500, description)
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

func validateDescriptorEntry(parent *os.File, name string, descriptor *os.File, expectedType uint32) error {
	var pathStat, descriptorStat unix.Stat_t
	if err := unix.Fstatat(int(parent.Fd()), name, &pathStat, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return err
	}
	if err := unix.Fstat(int(descriptor.Fd()), &descriptorStat); err != nil {
		return err
	}
	if pathStat.Mode&unix.S_IFMT != expectedType || descriptorStat.Mode&unix.S_IFMT != expectedType || pathStat.Dev != descriptorStat.Dev || pathStat.Ino != descriptorStat.Ino {
		return errors.New("entry no longer refers to its validated descriptor")
	}
	return nil
}

func descriptorStat(file *os.File) (unix.Stat_t, error) {
	var stat unix.Stat_t
	err := unix.Fstat(int(file.Fd()), &stat)
	return stat, err
}

func isMissing(err error) bool {
	return errors.Is(err, os.ErrNotExist) || errors.Is(err, unix.ENOENT)
}
