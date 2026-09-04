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
	projectsDirectory, exists, err := platform.openWorkspaceProjects(account)
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

func (platform *NativePlatform) GenerateWorkspaceGitKey(ctx context.Context, workspace Account) (string, error) {
	keyPath := workspaceGitKeyPath(workspace)
	publicKey, err := platform.workspaceGitPublicKey(ctx, workspace, keyPath)
	if err == nil {
		return publicKey, nil
	}
	result, generateErr := platform.runner().Run(ctx, Command{Name: "/usr/sbin/runuser", Args: []string{
		"--user", workspace.Username, "--", "/usr/bin/ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", "soda-workspace=" + workspace.Username, "-f", keyPath,
	}})
	if generateErr != nil {
		return "", fmt.Errorf("generate workspace outbound Git key: %w", generateErr)
	}
	if result.ExitCode != 0 {
		return "", fmt.Errorf("generate workspace outbound Git key: %s", strings.TrimSpace(result.Stderr))
	}
	return platform.workspaceGitPublicKey(ctx, workspace, keyPath)
}

func (platform *NativePlatform) workspaceGitPublicKey(ctx context.Context, workspace Account, keyPath string) (string, error) {
	result, err := platform.runner().Run(ctx, Command{Name: "/usr/sbin/runuser", Args: []string{
		"--user", workspace.Username, "--", "/usr/bin/ssh-keygen", "-y", "-f", keyPath,
	}})
	if err != nil {
		return "", err
	}
	if result.ExitCode != 0 {
		return "", fmt.Errorf("read workspace outbound Git key: %s", strings.TrimSpace(result.Stderr))
	}
	key, err := canonicalAuthorizedKey(result.Stdout)
	if err != nil {
		return "", fmt.Errorf("read workspace outbound Git key: %w", err)
	}
	return key, nil
}

func (platform *NativePlatform) CloneWorkspace(ctx context.Context, workspace Account, projectID, remote string) error {
	if err := ValidateCanonicalURL(remote); err != nil {
		return fmt.Errorf("clone URL: %w", err)
	}
	if !projectIDPattern.MatchString(projectID) {
		return errors.New("project id must match [a-z][a-z0-9-]{0,23}")
	}
	projectsDirectory, err := platform.openWorkspaceProjectsForPublication(workspace)
	if err != nil {
		return err
	}
	defer projectsDirectory.Close()
	if err = platform.removePreviousCloneAttempt(ctx, workspace, projectID); err != nil {
		return err
	}
	target, err := reservePublicationTarget(projectsDirectory, workspace, projectID)
	if err != nil {
		return err
	}
	defer target.directory.Close()
	keyPath := workspaceGitKeyPath(workspace)
	result, err := platform.runner().Run(ctx, Command{Name: "/usr/sbin/runuser", Args: []string{
		"--user", workspace.Username, "--", "/usr/bin/env", "-i",
		"HOME=" + workspace.Home,
		"USER=" + workspace.Username,
		"LOGNAME=" + workspace.Username,
		"PATH=/usr/local/bin:/usr/bin:/bin",
		"GIT_TERMINAL_PROMPT=0",
		"GIT_SSH_COMMAND=/usr/bin/ssh -i " + keyPath + " -o IdentitiesOnly=yes -o BatchMode=yes -o StrictHostKeyChecking=accept-new",
		"/usr/bin/git", "clone", "--", remote, target.path,
	}})
	if err != nil {
		return fmt.Errorf("clone workspace repository: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("clone workspace repository: %s", strings.TrimSpace(result.Stderr))
	}
	if err = validateCopiedWorkspace(target.directory, workspace.UID); err != nil {
		return err
	}
	if err = platform.relabelPublication(ctx, projectsDirectory, target); err != nil {
		return err
	}
	return finalizeWorkspacePublication(projectsDirectory, target.name, projectID)
}

func (platform *NativePlatform) removePreviousCloneAttempt(ctx context.Context, workspace Account, projectID string) error {
	path := filepath.Join(workspace.Home, "Projects", ".soda-"+projectID+".tmp")
	result, err := platform.runner().Run(ctx, Command{Name: "/usr/sbin/runuser", Args: []string{
		"--user", workspace.Username, "--", "/usr/bin/rm", "--recursive", "--force", "--", path,
	}})
	if err != nil {
		return fmt.Errorf("remove incomplete workspace clone from previous attempt: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("remove incomplete workspace clone from previous attempt: %s", strings.TrimSpace(result.Stderr))
	}
	return nil
}

func workspaceGitKeyPath(workspace Account) string {
	return filepath.Join(workspace.Home, ".ssh", "id_ed25519_soda")
}

func (platform *NativePlatform) openWorkspaceProjects(account Account) (*os.File, bool, error) {
	home, err := platform.openValidatedAccountHome(account)
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

func (platform *NativePlatform) openWorkspaceProjectsForPublication(workspace Account) (*os.File, error) {
	home, err := platform.openValidatedAccountHome(workspace)
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
