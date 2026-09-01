package projects

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
)

func (platform *NativePlatform) UIDMin() (int, error) {
	file, err := os.Open(platform.LoginDefsPath)
	if err != nil {
		return 0, fmt.Errorf("read Linux UID policy: %w", err)
	}
	defer file.Close()
	return scanUIDMin(file)
}

func scanUIDMin(file *os.File) (int, error) {
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && fields[0] == "UID_MIN" {
			return parseInt("UID_MIN", fields[1])
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("read Linux UID policy: %w", err)
	}
	return 0, errors.New("Linux UID policy does not define UID_MIN")
}

func (platform *NativePlatform) LookupAccount(ctx context.Context, username string) (Account, error) {
	if username == "" || strings.ContainsAny(username, ":\x00\r\n") {
		return Account{}, errors.New("invalid Linux username")
	}
	result, err := platform.run(ctx, "/usr/bin/getent", "passwd", username)
	if err != nil {
		return Account{}, fmt.Errorf("look up Linux account: %w", err)
	}
	if result.ExitCode != 0 {
		return Account{}, fmt.Errorf("%w: %s", ErrAccountNotFound, username)
	}
	account, err := parsePasswdLine(strings.TrimSpace(result.Stdout))
	if err != nil {
		return Account{}, err
	}
	if account.Username != username {
		return Account{}, errors.New("Linux account lookup returned a different username")
	}
	return platform.loadAccountGroups(ctx, account)
}

func (platform *NativePlatform) loadAccountGroups(ctx context.Context, account Account) (Account, error) {
	groups, err := platform.groups(ctx, account.Username)
	if err != nil {
		return Account{}, err
	}
	account.Groups = groups
	primaryGroup, err := platform.primaryGroup(ctx, account.Username)
	if err != nil {
		return Account{}, err
	}
	account.PrimaryGroup = primaryGroup
	return account, nil
}

func (platform *NativePlatform) WorkspaceAccounts(ctx context.Context) ([]Account, error) {
	groupResult, err := platform.run(ctx, "/usr/bin/getent", "group", WorkspaceGroup)
	if err != nil {
		return nil, fmt.Errorf("look up workspace group: %w", err)
	}
	if groupResult.ExitCode != 0 {
		return nil, errors.New("workspace group does not exist")
	}
	group, err := parseWorkspaceGroup(groupResult.Stdout)
	if err != nil {
		return nil, err
	}

	passwdResult, err := platform.run(ctx, "/usr/bin/getent", "passwd")
	if err != nil {
		return nil, fmt.Errorf("enumerate Linux accounts: %w", err)
	}
	if passwdResult.ExitCode != 0 {
		return nil, errors.New("enumerate Linux accounts")
	}
	return platform.workspaceAccountsFromPasswd(ctx, group, passwdResult.Stdout)
}

type workspaceGroupRecord struct {
	GID     int
	Members map[string]bool
}

func parseWorkspaceGroup(record string) (workspaceGroupRecord, error) {
	fields := strings.Split(strings.TrimSpace(record), ":")
	if len(fields) != 4 || fields[0] != WorkspaceGroup {
		return workspaceGroupRecord{}, errors.New("workspace group record is invalid")
	}
	gid, err := parseInt("workspace group GID", fields[2])
	if err != nil {
		return workspaceGroupRecord{}, err
	}
	members := map[string]bool{}
	if fields[3] == "" {
		return workspaceGroupRecord{GID: gid, Members: members}, nil
	}
	for _, username := range strings.Split(fields[3], ",") {
		if username == "" || strings.ContainsAny(username, ":\x00\r\n") || members[username] {
			return workspaceGroupRecord{}, errors.New("workspace group member list is invalid")
		}
		members[username] = true
	}
	return workspaceGroupRecord{GID: gid, Members: members}, nil
}

func (platform *NativePlatform) workspaceAccountsFromPasswd(ctx context.Context, group workspaceGroupRecord, records string) ([]Account, error) {
	passwdAccounts, err := parsePasswdAccounts(records)
	if err != nil {
		return nil, err
	}
	candidates, err := selectWorkspaceCandidates(passwdAccounts, group)
	if err != nil {
		return nil, err
	}
	return platform.loadWorkspaceCandidates(ctx, candidates)
}

func parsePasswdAccounts(records string) ([]Account, error) {
	lines := strings.Split(strings.TrimSpace(records), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil, errors.New("Linux account enumeration is empty")
	}
	accounts := make([]Account, 0, len(lines))
	foundUsernames := map[string]bool{}
	for _, line := range lines {
		account, err := parsePasswdLine(line)
		if err != nil {
			return nil, err
		}
		if foundUsernames[account.Username] {
			return nil, fmt.Errorf("Linux account enumeration contains duplicate username %s", account.Username)
		}
		foundUsernames[account.Username] = true
		accounts = append(accounts, account)
	}
	return accounts, nil
}

func selectWorkspaceCandidates(accounts []Account, group workspaceGroupRecord) ([]Account, error) {
	candidates := []Account{}
	foundMembers := map[string]bool{}
	for _, account := range accounts {
		if group.Members[account.Username] {
			foundMembers[account.Username] = true
		}
		candidate := account.GID == group.GID || group.Members[account.Username] || strings.HasPrefix(account.GECOS, workspaceMarkerKey)
		if candidate {
			candidates = append(candidates, account)
		}
	}
	for username := range group.Members {
		if !foundMembers[username] {
			return nil, fmt.Errorf("workspace group member %s has no Linux account record", username)
		}
	}
	return candidates, nil
}

func (platform *NativePlatform) loadWorkspaceCandidates(ctx context.Context, candidates []Account) ([]Account, error) {
	accounts := make([]Account, 0, len(candidates))
	for _, candidate := range candidates {
		account, err := platform.loadAccountGroups(ctx, candidate)
		if err != nil {
			return nil, fmt.Errorf("load workspace candidate %s: %w", candidate.Username, err)
		}
		accounts = append(accounts, account)
	}
	sort.Slice(accounts, func(i, j int) bool { return accounts[i].Username < accounts[j].Username })
	return accounts, nil
}

func (platform *NativePlatform) groups(ctx context.Context, username string) (map[string]bool, error) {
	result, err := platform.run(ctx, "/usr/bin/id", "--name", "--groups", username)
	if err != nil {
		return nil, fmt.Errorf("read Linux groups: %w", err)
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("read Linux groups: %s", strings.TrimSpace(result.Stderr))
	}
	groups := map[string]bool{}
	for _, group := range strings.Fields(result.Stdout) {
		groups[group] = true
	}
	return groups, nil
}

func (platform *NativePlatform) primaryGroup(ctx context.Context, username string) (string, error) {
	result, err := platform.run(ctx, "/usr/bin/id", "--name", "--group", username)
	if err != nil {
		return "", fmt.Errorf("read Linux primary group: %w", err)
	}
	group := strings.TrimSpace(result.Stdout)
	if result.ExitCode != 0 || group == "" || strings.ContainsAny(group, "\x00\r\n") {
		return "", errors.New("read Linux primary group")
	}
	return group, nil
}

func parsePasswdLine(line string) (Account, error) {
	fields := strings.Split(line, ":")
	if len(fields) != 7 {
		return Account{}, errors.New("Linux account record is invalid")
	}
	uid, err := parseInt("account UID", fields[2])
	if err != nil {
		return Account{}, err
	}
	gid, err := parseInt("account GID", fields[3])
	if err != nil {
		return Account{}, err
	}
	return Account{
		Username: fields[0],
		UID:      uid,
		GID:      gid,
		GECOS:    fields[4],
		Home:     fields[5],
		Shell:    fields[6],
	}, nil
}

func (platform *NativePlatform) CreateWorkspace(ctx context.Context, primary Account, projectID string) (Account, error) {
	username, err := DerivedUsername(primary.Username, projectID)
	if err != nil {
		return Account{}, err
	}
	marker, err := WorkspaceMarker(primary.Username, projectID)
	if err != nil {
		return Account{}, err
	}
	uidMin, err := platform.UIDMin()
	if err != nil {
		return Account{}, err
	}
	home := "/home/" + username
	result, err := platform.run(ctx, "/usr/sbin/useradd",
		"--create-home",
		"--user-group",
		"--groups", WorkspaceGroup,
		"--shell", WorkspaceShell,
		"--home-dir", home,
		"--comment", marker,
		"--", username,
	)
	if err != nil {
		return Account{}, fmt.Errorf("create workspace account %s: %w", username, err)
	}
	if result.ExitCode != 0 {
		return Account{}, fmt.Errorf("create workspace account %s: %s", username, strings.TrimSpace(result.Stderr))
	}
	account, err := platform.LookupAccount(ctx, username)
	if err != nil {
		return Account{}, fmt.Errorf("verify created workspace account %s: %w", username, err)
	}
	if err := account.ValidateWorkspace(primary.Username, projectID, uidMin); err != nil {
		return Account{}, fmt.Errorf("verify created workspace account %s: %w", username, err)
	}
	if err := platform.ValidatePasswordLocked(ctx, account); err != nil {
		return Account{}, err
	}
	return account, nil
}

// ValidatePasswordLocked uses shadow-utils' native account status boundary.
// It is called only by the privileged helper before publishing or deleting a
// derived account; no password or shadow data enters Soda.
func (platform *NativePlatform) ValidatePasswordLocked(ctx context.Context, account Account) error {
	result, err := platform.run(ctx, "/usr/bin/passwd", "--status", account.Username)
	if err != nil {
		return fmt.Errorf("read Linux password status for %s: %w", account.Username, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("read Linux password status for %s: %s", account.Username, strings.TrimSpace(result.Stderr))
	}
	return validatePasswordStatus(account.Username, result.Stdout)
}

func validatePasswordStatus(username, output string) error {
	fields := strings.Fields(output)
	if len(fields) < 2 || fields[0] != username {
		return fmt.Errorf("Linux password status for %s is invalid", username)
	}
	if fields[1] != "L" && fields[1] != "LK" {
		return fmt.Errorf("workspace account %s does not have a locked password", username)
	}
	return nil
}
