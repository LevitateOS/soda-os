package projects

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"golang.org/x/sys/unix"
)

type nativeSetupLock struct {
	file *os.File
}

func (lock *nativeSetupLock) Close() error {
	unlockErr := unix.Flock(int(lock.file.Fd()), unix.LOCK_UN)
	closeErr := lock.file.Close()
	return errors.Join(unlockErr, closeErr)
}

func (platform *NativePlatform) SetupLock(account Account, projectID string) (io.Closer, error) {
	if err := validateStagingProjectID(projectID); err != nil {
		return nil, err
	}
	stagingRoot, err := platform.ensureStagingRoot(account)
	if err != nil {
		return nil, err
	}
	defer stagingRoot.Close()
	lock, err := openSetupLockAt(stagingRoot, account, projectID)
	if err != nil {
		return nil, err
	}
	if err = unix.Flock(int(lock.Fd()), unix.LOCK_EX); err != nil {
		lock.Close()
		return nil, fmt.Errorf("lock workspace setup: %w", err)
	}
	return &nativeSetupLock{file: lock}, nil
}

func openSetupLockAt(parent *os.File, account Account, projectID string) (*os.File, error) {
	name := ".setup-" + projectID + ".lock"
	descriptor, err := unix.Openat2(int(parent.Fd()), name, &unix.OpenHow{
		Flags:   unix.O_CREAT | unix.O_RDWR | unix.O_CLOEXEC | unix.O_NOFOLLOW,
		Mode:    0o600,
		Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_MAGICLINKS | unix.RESOLVE_NO_SYMLINKS,
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

func validateOwnedRegularFile(file *os.File, expectedUID int, description string) error {
	stat, err := descriptorStat(file)
	if err != nil {
		return fmt.Errorf("inspect %s: %w", description, err)
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFREG {
		return fmt.Errorf("%s is not a regular file", description)
	}
	if int(stat.Uid) != expectedUID {
		return fmt.Errorf("%s has unexpected ownership", description)
	}
	if stat.Nlink != 1 {
		return fmt.Errorf("%s has unexpected link count", description)
	}
	return validateSafeFileMode(stat.Mode, description)
}

func (platform *NativePlatform) StagingPath(account Account, projectID string) string {
	return filepath.Join(
		platform.RuntimeRoot,
		strconv.Itoa(account.UID),
		"soda-projects",
		projectID,
		"checkout",
	)
}

func (platform *NativePlatform) ResetStaging(account Account, projectID string) error {
	if err := validateStagingProjectID(projectID); err != nil {
		return err
	}
	stagingRoot, err := platform.ensureStagingRoot(account)
	if err != nil {
		return err
	}
	defer stagingRoot.Close()
	return resetStagingAt(stagingRoot, account, projectID)
}

func resetStagingAt(stagingRoot *os.File, account Account, projectID string) error {
	if err := removeOwnedDirectoryAt(stagingRoot, projectID, account.UID); err != nil {
		return err
	}
	if err := unix.Mkdirat(int(stagingRoot.Fd()), projectID, 0o700); err != nil {
		return fmt.Errorf("create clone staging directory: %w", err)
	}
	projectDirectory, err := openDirectoryAt(stagingRoot, projectID)
	if err != nil {
		return fmt.Errorf("open clone staging directory: %w", err)
	}
	defer projectDirectory.Close()
	return validateOwnedDirectory(projectDirectory, account.UID, "clone staging directory")
}

// PrepareStaging runs as the primary user. It validates that every object in
// the completed clone belongs to that user, then adds only the read/traverse
// bits needed by the derived user. The primary runtime directory remains 0700,
// so this does not expose the checkout through the filesystem namespace.
func (platform *NativePlatform) PrepareStaging(account Account, projectID string) error {
	checkout, err := platform.openStagingCheckout(account, projectID)
	if err != nil {
		return err
	}
	defer checkout.Close()
	if err = prepareOwnedTree(checkout, account.UID); err != nil {
		return fmt.Errorf("prepare completed clone: %w", err)
	}
	return nil
}

func (platform *NativePlatform) CleanupStaging(account Account, projectID string) error {
	if err := validateStagingProjectID(projectID); err != nil {
		return err
	}
	stagingRoot, exists, err := platform.openStagingRoot(account)
	if err != nil || !exists {
		return err
	}
	defer stagingRoot.Close()
	return removeOwnedDirectoryAt(stagingRoot, projectID, account.UID)
}

func (platform *NativePlatform) openStagingCheckout(account Account, projectID string) (*os.File, error) {
	if err := validateStagingProjectID(projectID); err != nil {
		return nil, err
	}
	stagingRoot, exists, err := platform.openStagingRoot(account)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.New("clone staging root does not exist")
	}
	defer stagingRoot.Close()
	return openOwnedDirectoryChain(stagingRoot, []string{projectID, "checkout"}, account.UID)
}

func validateStagingProjectID(projectID string) error {
	if !projectIDPattern.MatchString(projectID) {
		return errors.New("project id must match [a-z][a-z0-9-]{0,23}")
	}
	return nil
}

func (platform *NativePlatform) ensureStagingRoot(account Account) (*os.File, error) {
	userRuntime, err := platform.openRuntimeUserDirectory(account)
	if err != nil {
		return nil, err
	}
	defer userRuntime.Close()
	return ensureCallerOwnedDirectoryAt(userRuntime, "soda-projects", account, "workspace staging root")
}

func (platform *NativePlatform) openStagingRoot(account Account) (*os.File, bool, error) {
	userRuntime, err := platform.openRuntimeUserDirectory(account)
	if isMissing(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	defer userRuntime.Close()
	return openOptionalOwnedDirectory(userRuntime, "soda-projects", account.UID, "workspace staging root")
}

func (platform *NativePlatform) openRuntimeUserDirectory(account Account) (*os.File, error) {
	runtimeRoot, err := openAbsoluteDirectoryNoSymlinks(platform.RuntimeRoot)
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

func ensureCallerOwnedDirectoryAt(parent *os.File, name string, account Account, description string) (*os.File, error) {
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

func openOwnedDirectoryChain(root *os.File, components []string, expectedUID int) (*os.File, error) {
	current := root
	var opened *os.File
	for _, component := range components {
		next, err := openDirectoryAt(current, component)
		if opened != nil {
			opened.Close()
		}
		if err != nil {
			return nil, fmt.Errorf("open completed clone component %s: %w", component, err)
		}
		opened = next
		current = next
		if err = validateOwnedDirectory(current, expectedUID, "completed clone component "+component); err != nil {
			current.Close()
			return nil, err
		}
	}
	return opened, nil
}

func removeOwnedDirectoryAt(parent *os.File, name string, expectedUID int) error {
	directory, err := openDirectoryAt(parent, name)
	if isMissing(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open clone staging directory: %w", err)
	}
	defer directory.Close()
	if err = validateOwnedDirectory(directory, expectedUID, "clone staging directory"); err != nil {
		return err
	}
	if err = validateTreeOwnership(directory, expectedUID); err != nil {
		return err
	}
	if err = removeDirectoryContents(directory, expectedUID); err != nil {
		return err
	}
	if err = validateDescriptorEntry(parent, name, directory); err != nil {
		return fmt.Errorf("validate clone staging pathname: %w", err)
	}
	if err = unix.Unlinkat(int(parent.Fd()), name, unix.AT_REMOVEDIR); err != nil {
		return fmt.Errorf("remove clone staging directory: %w", err)
	}
	return nil
}

func validateTreeOwnership(directory *os.File, expectedUID int) error {
	entries, err := directory.ReadDir(-1)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err = validateTreeEntryOwnership(directory, entry.Name(), expectedUID); err != nil {
			return err
		}
	}
	return nil
}

func validateTreeEntryOwnership(parent *os.File, name string, expectedUID int) error {
	stat, err := entryStat(parent, name)
	if err != nil {
		return err
	}
	if int(stat.Uid) != expectedUID {
		return fmt.Errorf("clone staging entry %s has unexpected ownership", name)
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFDIR {
		return nil
	}
	child, err := openDirectoryAt(parent, name)
	if err != nil {
		return err
	}
	defer child.Close()
	if err = validateOwnedDirectory(child, expectedUID, "clone staging directory "+name); err != nil {
		return err
	}
	return validateTreeOwnership(child, expectedUID)
}

func removeDirectoryContents(directory *os.File, expectedUID int) error {
	if _, err := directory.Seek(0, io.SeekStart); err != nil {
		return err
	}
	entries, err := directory.ReadDir(-1)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err = removeDirectoryEntry(directory, entry.Name(), expectedUID); err != nil {
			return err
		}
	}
	return nil
}

func removeDirectoryEntry(parent *os.File, name string, expectedUID int) error {
	stat, err := entryStat(parent, name)
	if err != nil {
		return err
	}
	if int(stat.Uid) != expectedUID {
		return fmt.Errorf("clone staging entry %s has unexpected ownership", name)
	}
	if stat.Mode&unix.S_IFMT == unix.S_IFDIR {
		return removeChildDirectory(parent, name, expectedUID)
	}
	if err = unix.Unlinkat(int(parent.Fd()), name, 0); err != nil {
		return fmt.Errorf("remove clone staging entry %s: %w", name, err)
	}
	return nil
}

func removeChildDirectory(parent *os.File, name string, expectedUID int) error {
	child, err := openDirectoryAt(parent, name)
	if err != nil {
		return err
	}
	defer child.Close()
	if err = validateOwnedDirectory(child, expectedUID, "clone staging directory "+name); err != nil {
		return err
	}
	if err = removeDirectoryContents(child, expectedUID); err != nil {
		return err
	}
	if err = validateDescriptorEntry(parent, name, child); err != nil {
		return err
	}
	return unix.Unlinkat(int(parent.Fd()), name, unix.AT_REMOVEDIR)
}
