package projects

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

type incompleteHomeEntryKind uint8

const (
	incompleteRegularFile incompleteHomeEntryKind = iota
	incompleteDirectory
)

type terminationStep struct {
	name string
	args []string
}

func (platform *NativePlatform) SafeToRemoveIncomplete(account Account, projectID string) error {
	if !projectIDPattern.MatchString(projectID) {
		return errors.New("project id must match [a-z][a-z0-9-]{0,23}")
	}
	home, err := platform.openValidatedAccountHome(account)
	if err != nil {
		return err
	}
	defer home.Close()
	if err = validateIncompleteHome(home, account); err != nil {
		return err
	}
	if err = validateIncompleteSSHDirectory(home, account); err != nil {
		return err
	}
	return validateIncompleteProjectsDirectory(home, account, projectID)
}

func validateIncompleteHome(home *os.File, account Account) error {
	entries, err := home.ReadDir(-1)
	if err != nil {
		return fmt.Errorf("inspect incomplete workspace home: %w", err)
	}
	for _, entry := range entries {
		if err = validateIncompleteHomeEntry(home, account, entry.Name()); err != nil {
			return err
		}
	}
	return nil
}

func validateIncompleteHomeEntry(home *os.File, account Account, name string) error {
	kind, known := incompleteHomeEntryType(name)
	if !known {
		return fmt.Errorf("workspace home contains unexpected path %s", name)
	}
	stat, err := entryStat(home, name)
	if err != nil {
		return err
	}
	if int(stat.Uid) != account.UID {
		return fmt.Errorf("workspace path %s has unexpected ownership", filepath.Join(account.Home, name))
	}
	return validateIncompleteEntryType(name, stat.Mode, kind)
}

func incompleteHomeEntryType(name string) (incompleteHomeEntryKind, bool) {
	switch name {
	case ".bash_logout", ".bash_profile", ".bashrc":
		return incompleteRegularFile, true
	case ".ssh", "Projects":
		return incompleteDirectory, true
	default:
		return incompleteRegularFile, false
	}
}

func validateIncompleteEntryType(name string, mode uint32, kind incompleteHomeEntryKind) error {
	if kind == incompleteDirectory {
		if mode&unix.S_IFMT != unix.S_IFDIR {
			return fmt.Errorf("workspace path %s is not a directory", name)
		}
		return nil
	}
	if mode&unix.S_IFMT != unix.S_IFREG {
		return fmt.Errorf("workspace path %s is not a regular file", name)
	}
	return nil
}

func validateIncompleteSSHDirectory(home *os.File, account Account) error {
	sshDirectory, exists, err := openOptionalOwnedDirectory(home, ".ssh", account.UID, "workspace SSH directory")
	if err != nil || !exists {
		return err
	}
	defer sshDirectory.Close()
	entries, err := sshDirectory.ReadDir(-1)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err = validateIncompleteSSHEntry(sshDirectory, account, entry.Name()); err != nil {
			return err
		}
	}
	return nil
}

func validateIncompleteSSHEntry(sshDirectory *os.File, account Account, name string) error {
	if name != "authorized_keys" && name != stagedAuthorizedKeysName {
		return fmt.Errorf("workspace SSH directory contains unexpected path %s", name)
	}
	stat, err := entryStat(sshDirectory, name)
	if err != nil {
		return err
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFREG {
		return errors.New("workspace authorized_keys is not a regular file")
	}
	if int(stat.Uid) != account.UID {
		return fmt.Errorf("workspace path %s has unexpected ownership", filepath.Join(account.Home, ".ssh", name))
	}
	return validateSafeFileMode(stat.Mode, "workspace authorized_keys")
}

func validateIncompleteProjectsDirectory(home *os.File, account Account, projectID string) error {
	projectsDirectory, exists, err := openOptionalOwnedDirectory(home, "Projects", account.UID, "workspace Projects directory")
	if err != nil || !exists {
		return err
	}
	defer projectsDirectory.Close()
	entries, err := projectsDirectory.ReadDir(-1)
	if err != nil {
		return err
	}
	if len(entries) > 1 {
		return errors.New("workspace Projects directory contains multiple publication paths")
	}
	for _, entry := range entries {
		if err = validateIncompleteProjectEntry(projectsDirectory, account, projectID, entry.Name()); err != nil {
			return err
		}
	}
	return nil
}

func validateIncompleteProjectEntry(projectsDirectory *os.File, account Account, projectID, name string) error {
	temporary := ".soda-" + projectID + ".tmp"
	if name != temporary && name != projectID {
		return fmt.Errorf("workspace Projects directory contains unexpected path %s", name)
	}
	projectDirectory, err := openDirectoryAt(projectsDirectory, name)
	if err != nil {
		return errors.New("workspace publication path is not a directory")
	}
	defer projectDirectory.Close()
	if err = validateOwnedDirectory(projectDirectory, account.UID, "workspace publication path"); err != nil {
		return err
	}
	if err = validateOwnedTreeDescriptor(projectDirectory, account.UID); err != nil {
		return err
	}
	if name == projectID {
		return validateGitDirectoryAt(projectDirectory, account.UID, "workspace publication .git directory")
	}
	return nil
}

func (platform *NativePlatform) PreflightDeleteAccount(ctx context.Context, account Account) error {
	_, err := platform.revalidateAccountForDeletion(ctx, account)
	return err
}

func (platform *NativePlatform) DeleteAccount(ctx context.Context, account Account) error {
	current, err := platform.revalidateAccountForDeletion(ctx, account)
	if err != nil {
		return err
	}
	terminatedLogindUser, err := platform.terminateLogindUser(ctx, current)
	if err != nil {
		return err
	}
	for _, step := range accountTerminationSteps(current) {
		if err = platform.runTerminationStep(ctx, current, step); err != nil {
			return err
		}
	}
	if err = platform.verifyNoOwnedProcesses(ctx, current); err != nil {
		return err
	}
	if terminatedLogindUser {
		if err = platform.resetFailedUserManager(ctx, current); err != nil {
			return err
		}
	}
	current, err = platform.revalidateAccountForDeletion(ctx, current)
	if err != nil {
		return err
	}
	return platform.removeLinuxAccount(ctx, current)
}

func (platform *NativePlatform) revalidateAccountForDeletion(ctx context.Context, expected Account) (Account, error) {
	current, err := platform.LookupAccount(ctx, expected.Username)
	if err != nil {
		return Account{}, err
	}
	if !sameAccount(current, expected) {
		return Account{}, fmt.Errorf("Linux account %s changed before deletion", expected.Username)
	}
	if err = platform.validateAccountHome(current); err != nil {
		return Account{}, err
	}
	return current, nil
}

func sameAccount(current, expected Account) bool {
	if current.Username != expected.Username || current.UID != expected.UID || current.GID != expected.GID {
		return false
	}
	if current.GECOS != expected.GECOS || current.Home != expected.Home || current.Shell != expected.Shell {
		return false
	}
	if current.PrimaryGroup != expected.PrimaryGroup {
		return false
	}
	return sameGroups(current.Groups, expected.Groups)
}

func sameGroups(current, expected map[string]bool) bool {
	if len(current) != len(expected) {
		return false
	}
	for group, member := range current {
		if expected[group] != member {
			return false
		}
	}
	return true
}

func (platform *NativePlatform) validateAccountHome(account Account) error {
	home, err := platform.openValidatedAccountHome(account)
	if err != nil {
		return err
	}
	return home.Close()
}

func (platform *NativePlatform) openValidatedAccountHome(account Account) (*os.File, error) {
	expectedHome := filepath.Join(platform.homeRoot(), account.Username)
	homeMatches := account.Home == expectedHome
	if !homeMatches {
		resolvedHomeRoot, err := filepath.EvalSymlinks(platform.homeRoot())
		if err != nil {
			return nil, fmt.Errorf("resolve Linux home root: %w", err)
		}
		homeMatches = account.Home == filepath.Join(resolvedHomeRoot, account.Username)
	}
	if !homeMatches {
		return nil, fmt.Errorf("Linux account %s has unexpected home %s", account.Username, account.Home)
	}
	homeRoot, err := openManagedHomeRoot(platform.homeRoot())
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

func (platform *NativePlatform) homeRoot() string {
	if platform.HomeRoot == "" {
		return "/home"
	}
	return platform.HomeRoot
}

func accountTerminationSteps(account Account) []terminationStep {
	uid := strconv.Itoa(account.UID)
	return []terminationStep{
		{name: "/usr/bin/pkill", args: []string{"--signal", "TERM", "--uid", uid}},
		{name: "/usr/bin/pkill", args: []string{"--signal", "KILL", "--uid", uid}},
	}
}

func (platform *NativePlatform) terminateLogindUser(ctx context.Context, account Account) (bool, error) {
	result, err := platform.run(ctx, "/usr/bin/loginctl", "list-users", "--no-legend", "--no-pager")
	if err != nil {
		return false, err
	}
	if result.ExitCode != 0 {
		return false, fmt.Errorf("inspect logind users: %s", strings.TrimSpace(result.Stderr))
	}
	active, err := logindUserIsActive(result.Stdout, account)
	if err != nil || !active {
		return false, err
	}
	result, err = platform.run(ctx, "/usr/bin/loginctl", "terminate-user", account.Username)
	if err != nil {
		return false, err
	}
	if result.ExitCode != 0 {
		return false, fmt.Errorf("terminate %s logind sessions: %s", account.Username, strings.TrimSpace(result.Stderr))
	}
	return true, nil
}

func logindUserIsActive(output string, account Account) (bool, error) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		uid, username, err := parseLogindUserRecord(scanner.Text())
		if err != nil {
			return false, err
		}
		matched, err := matchLogindUserRecord(account, uid, username)
		if err != nil || matched {
			return matched, err
		}
	}
	if err := scanner.Err(); err != nil {
		return false, fmt.Errorf("read logind users: %w", err)
	}
	return false, nil
}

func parseLogindUserRecord(line string) (int, string, error) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0, "", errors.New("logind user record is invalid")
	}
	uid, err := strconv.Atoi(fields[0])
	if err != nil || uid < 0 {
		return 0, "", errors.New("logind user record has an invalid UID")
	}
	return uid, fields[1], nil
}

func matchLogindUserRecord(account Account, uid int, username string) (bool, error) {
	uidMatches := uid == account.UID
	usernameMatches := username == account.Username
	if uidMatches != usernameMatches {
		return false, errors.New("logind user record does not match the Linux account")
	}
	return uidMatches, nil
}

func (platform *NativePlatform) runTerminationStep(ctx context.Context, account Account, step terminationStep) error {
	result, err := platform.run(ctx, step.name, step.args...)
	if err != nil {
		return err
	}
	if result.ExitCode != 0 && result.ExitCode != 1 {
		return fmt.Errorf("terminate %s processes: %s", account.Username, strings.TrimSpace(result.Stderr))
	}
	return nil
}

func (platform *NativePlatform) verifyNoOwnedProcesses(ctx context.Context, account Account) error {
	deadline := time.Now().Add(5 * time.Second)
	for {
		result, err := platform.run(ctx, "/usr/bin/pgrep", "--uid", strconv.Itoa(account.UID))
		if err != nil {
			return err
		}
		switch result.ExitCode {
		case 1:
			return nil
		case 0:
			if !time.Now().Before(deadline) {
				return fmt.Errorf("account %s still owns processes", account.Username)
			}
		case 2:
			return fmt.Errorf("verify %s processes: %s", account.Username, strings.TrimSpace(result.Stderr))
		default:
			return fmt.Errorf("verify %s processes: unexpected pgrep status %d: %s", account.Username, result.ExitCode, strings.TrimSpace(result.Stderr))
		}
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (platform *NativePlatform) removeLinuxAccount(ctx context.Context, account Account) error {
	result, err := platform.run(ctx, "/usr/sbin/userdel", "--remove", account.Username)
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("delete Linux account %s: %s", account.Username, strings.TrimSpace(result.Stderr))
	}
	return nil
}
