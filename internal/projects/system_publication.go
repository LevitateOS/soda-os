package projects

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

type publicationTarget struct {
	name      string
	path      string
	directory *os.File
}

func (platform *NativePlatform) WorkspaceReady(account Account, projectID string) (bool, error) {
	projectsDirectory, exists, err := openWorkspaceProjects(account)
	if err != nil || !exists {
		return false, err
	}
	defer projectsDirectory.Close()
	checkout, exists, err := openOptionalOwnedDirectory(
		projectsDirectory,
		projectID,
		account.UID,
		"workspace clone",
	)
	if err != nil || !exists {
		return false, err
	}
	defer checkout.Close()
	if err = validateReadyGitDirectory(checkout, account.UID); err != nil {
		return false, err
	}
	return true, nil
}

func openWorkspaceProjects(account Account) (*os.File, bool, error) {
	home, err := openAbsoluteDirectoryNoSymlinks(account.Home)
	if err != nil {
		return nil, false, fmt.Errorf("open workspace home: %w", err)
	}
	defer home.Close()
	if err = validateOwnedDirectory(home, account.UID, "workspace home"); err != nil {
		return nil, false, err
	}
	return openOptionalOwnedDirectory(home, "Projects", account.UID, "workspace Projects directory")
}

func openOptionalOwnedDirectory(parent *os.File, name string, expectedUID int, description string) (*os.File, bool, error) {
	directory, err := openDirectoryAt(parent, name)
	if isMissing(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("open %s: %w", description, err)
	}
	if err = validateOwnedDirectory(directory, expectedUID, description); err != nil {
		directory.Close()
		return nil, false, err
	}
	return directory, true, nil
}

func validateReadyGitDirectory(checkout *os.File, expectedUID int) error {
	gitDirectory, err := openDirectoryAt(checkout, ".git")
	if isMissing(err) {
		return errors.New("workspace path is not a complete Git clone")
	}
	if err != nil {
		return fmt.Errorf("open workspace .git directory: %w", err)
	}
	defer gitDirectory.Close()
	return validateOwnedDirectory(gitDirectory, expectedUID, "workspace .git directory")
}

func (platform *NativePlatform) PublishWorkspace(ctx context.Context, primary, workspace Account, projectID string) error {
	checkout, err := platform.validatedStagingCheckout(primary, projectID)
	if err != nil {
		return err
	}
	defer checkout.Close()
	projectsDirectory, err := openWorkspaceProjectsForPublication(workspace)
	if err != nil {
		return err
	}
	defer projectsDirectory.Close()
	target, err := reservePublicationTarget(projectsDirectory, workspace, projectID)
	if err != nil {
		return err
	}
	defer target.directory.Close()
	if err = platform.copyWorkspaceCheckout(ctx, workspace, checkout, target.directory); err != nil {
		return err
	}
	if err = validateCopiedWorkspace(target.directory, workspace.UID); err != nil {
		return err
	}
	if err = platform.relabelPublication(ctx, projectsDirectory, target); err != nil {
		return err
	}
	return finalizeWorkspacePublication(projectsDirectory, target.name, projectID)
}

func (platform *NativePlatform) validatedStagingCheckout(primary Account, projectID string) (*os.File, error) {
	checkout, err := platform.openStagingCheckout(primary, projectID)
	if err != nil {
		return nil, err
	}
	if err = validateReadableOwnedTree(checkout, primary.UID); err != nil {
		checkout.Close()
		return nil, fmt.Errorf("validate completed clone: %w", err)
	}
	if err = validateGitDirectoryAt(checkout, primary.UID, "completed clone .git directory"); err != nil {
		checkout.Close()
		return nil, err
	}
	return checkout, nil
}

func openWorkspaceProjectsForPublication(workspace Account) (*os.File, error) {
	home, err := openAbsoluteDirectoryNoSymlinks(workspace.Home)
	if err != nil {
		return nil, fmt.Errorf("open workspace home: %w", err)
	}
	defer home.Close()
	if err = validateOwnedDirectory(home, workspace.UID, "workspace home"); err != nil {
		return nil, err
	}
	projectsDirectory, err := ensureOwnedDirectoryAt(home, "Projects", workspace)
	if err != nil {
		return nil, fmt.Errorf("prepare workspace Projects directory: %w", err)
	}
	return projectsDirectory, nil
}

func reservePublicationTarget(projectsDirectory *os.File, workspace Account, projectID string) (publicationTarget, error) {
	target := publicationTarget{
		name: ".soda-" + projectID + ".tmp",
		path: filepath.Join(workspace.Home, "Projects", ".soda-"+projectID+".tmp"),
	}
	if err := reserveOwnedDirectory(projectsDirectory, target.name, workspace, target.path); err != nil {
		return publicationTarget{}, err
	}
	directory, err := openDirectoryAt(projectsDirectory, target.name)
	if err != nil {
		return publicationTarget{}, fmt.Errorf("open temporary workspace publication: %w", err)
	}
	if err = validateOwnedDirectory(directory, workspace.UID, "temporary workspace publication"); err != nil {
		directory.Close()
		return publicationTarget{}, err
	}
	target.directory = directory
	return target, nil
}

func reserveOwnedDirectory(parent *os.File, name string, account Account, path string) error {
	if err := unix.Mkdirat(int(parent.Fd()), name, 0o700); err != nil {
		if errors.Is(err, unix.EEXIST) {
			return fmt.Errorf("workspace publication path %s already exists", path)
		}
		return fmt.Errorf("reserve temporary workspace publication: %w", err)
	}
	if err := unix.Fchownat(int(parent.Fd()), name, account.UID, account.GID, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return fmt.Errorf("own temporary workspace publication: %w", err)
	}
	return nil
}

func (platform *NativePlatform) copyWorkspaceCheckout(ctx context.Context, workspace Account, checkout, target *os.File) error {
	request := Command{
		Name: "/usr/sbin/runuser",
		Args: []string{
			"--user", workspace.Username,
			"--", "/usr/bin/cp", "--archive", "--",
			"/proc/self/fd/3/.", "/proc/self/fd/4/",
		},
		ExtraFiles: []*os.File{checkout, target},
	}
	result, err := platform.runner().Run(ctx, request)
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("cp failed: %s", strings.TrimSpace(result.Stderr))
	}
	return nil
}

func validateCopiedWorkspace(target *os.File, expectedUID int) error {
	if err := validateOwnedTreeDescriptor(target, expectedUID); err != nil {
		return fmt.Errorf("validate temporary workspace publication: %w", err)
	}
	return validateGitDirectoryAt(target, expectedUID, "temporary workspace .git directory")
}

func (platform *NativePlatform) relabelPublication(ctx context.Context, parent *os.File, target publicationTarget) error {
	if err := validateDescriptorEntry(parent, target.name, target.directory); err != nil {
		return fmt.Errorf("validate temporary workspace pathname: %w", err)
	}
	result, err := platform.run(ctx, "/usr/sbin/restorecon", "-R", target.path)
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("restorecon failed: %s", strings.TrimSpace(result.Stderr))
	}
	if err = validateDescriptorEntry(parent, target.name, target.directory); err != nil {
		return fmt.Errorf("validate relabeled workspace pathname: %w", err)
	}
	return nil
}

func finalizeWorkspacePublication(parent *os.File, temporaryName, projectID string) error {
	err := unix.Renameat2(
		int(parent.Fd()),
		temporaryName,
		int(parent.Fd()),
		projectID,
		unix.RENAME_NOREPLACE,
	)
	if err != nil {
		return fmt.Errorf("publish workspace clone: %w", err)
	}
	if err = unix.Fsync(int(parent.Fd())); err != nil {
		return fmt.Errorf("sync workspace Projects directory: %w", err)
	}
	return nil
}
