package linuxhost

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/user"
	"sort"
	"strconv"
	"strings"
)

func (native *Native) UIDMin() (int, error) {
	file, err := os.Open(native.LoginDefsPath)
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

func (native *Native) LookupAccount(ctx context.Context, username string) (Account, error) {
	if err := validateAccountName(username); err != nil {
		return Account{}, err
	}
	result, err := native.run(ctx, "/usr/bin/getent", "passwd", username)
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
	return native.loadAccountGroups(ctx, account)
}

// CandidateAccounts returns complete accounts whose passwd or group records
// match the supplied native evidence. The caller owns the meaning of that
// evidence and must perform its own product-specific validation.
func (native *Native) CandidateAccounts(ctx context.Context, groupName, gecosPrefix string) ([]Account, error) {
	group, err := native.LookupGroup(ctx, groupName)
	if err != nil {
		return nil, err
	}
	passwdResult, err := native.run(ctx, "/usr/bin/getent", "passwd")
	if err != nil {
		return nil, fmt.Errorf("enumerate Linux accounts: %w", err)
	}
	if passwdResult.ExitCode != 0 {
		return nil, errors.New("enumerate Linux accounts")
	}
	accounts, err := parsePasswdAccounts(passwdResult.Stdout)
	if err != nil {
		return nil, err
	}
	candidates, err := selectCandidates(accounts, group, gecosPrefix)
	if err != nil {
		return nil, err
	}
	return native.loadCandidates(ctx, candidates)
}

func (native *Native) LookupGroup(ctx context.Context, groupName string) (Group, error) {
	if err := validateAccountName(groupName); err != nil {
		return Group{}, errors.New("invalid Linux group name")
	}
	result, err := native.run(ctx, "/usr/bin/getent", "group", groupName)
	if err != nil {
		return Group{}, fmt.Errorf("look up Linux group: %w", err)
	}
	if result.ExitCode != 0 {
		return Group{}, fmt.Errorf("Linux group %s does not exist", groupName)
	}
	return parseGroupLine(result.Stdout, groupName)
}

func (native *Native) loadAccountGroups(ctx context.Context, account Account) (Account, error) {
	groups, err := native.groups(ctx, account.Username)
	if err != nil {
		return Account{}, err
	}
	account.Groups = groups
	primaryGroup, err := native.primaryGroup(ctx, account.Username)
	if err != nil {
		return Account{}, err
	}
	account.PrimaryGroup = primaryGroup
	return account, nil
}

func parseGroupLine(record, expectedName string) (Group, error) {
	fields := strings.Split(strings.TrimSpace(record), ":")
	if len(fields) != 4 || fields[0] != expectedName {
		return Group{}, errors.New("Linux group record is invalid")
	}
	gid, err := parseInt("Linux group GID", fields[2])
	if err != nil {
		return Group{}, err
	}
	members := map[string]bool{}
	if fields[3] == "" {
		return Group{Name: expectedName, GID: gid, Members: members}, nil
	}
	for _, username := range strings.Split(fields[3], ",") {
		if err = validateAccountName(username); err != nil || members[username] {
			return Group{}, errors.New("Linux group member list is invalid")
		}
		members[username] = true
	}
	return Group{Name: expectedName, GID: gid, Members: members}, nil
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

func selectCandidates(accounts []Account, group Group, gecosPrefix string) ([]Account, error) {
	candidates := []Account{}
	foundMembers := map[string]bool{}
	for _, account := range accounts {
		if group.Members[account.Username] {
			foundMembers[account.Username] = true
		}
		if account.GID == group.GID || group.Members[account.Username] || strings.HasPrefix(account.GECOS, gecosPrefix) {
			candidates = append(candidates, account)
		}
	}
	for username := range group.Members {
		if !foundMembers[username] {
			return nil, fmt.Errorf("Linux group member %s has no account record", username)
		}
	}
	return candidates, nil
}

func (native *Native) loadCandidates(ctx context.Context, candidates []Account) ([]Account, error) {
	accounts := make([]Account, 0, len(candidates))
	for _, candidate := range candidates {
		account, err := native.loadAccountGroups(ctx, candidate)
		if err != nil {
			return nil, fmt.Errorf("load Linux account candidate %s: %w", candidate.Username, err)
		}
		accounts = append(accounts, account)
	}
	sort.Slice(accounts, func(i, j int) bool { return accounts[i].Username < accounts[j].Username })
	return accounts, nil
}

func (native *Native) groups(ctx context.Context, username string) (map[string]bool, error) {
	result, err := native.run(ctx, "/usr/bin/id", "--name", "--groups", username)
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

func (native *Native) primaryGroup(ctx context.Context, username string) (string, error) {
	result, err := native.run(ctx, "/usr/bin/id", "--name", "--group", username)
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
	return Account{Username: fields[0], UID: uid, GID: gid, GECOS: fields[4], Home: fields[5], Shell: fields[6]}, nil
}

func validateAccountName(name string) error {
	if name == "" || strings.ContainsAny(name, ":\x00\r\n") {
		return errors.New("invalid Linux account name")
	}
	return nil
}

func parseInt(field, value string) (int, error) {
	number, err := strconv.Atoi(value)
	if err != nil || number < 0 {
		return 0, fmt.Errorf("invalid %s", field)
	}
	return number, nil
}

func PKExecCaller() (PKExecIdentity, error) {
	if os.Geteuid() != 0 {
		return PKExecIdentity{}, errors.New("privileged helper must run with effective UID 0")
	}
	rawUID, present := os.LookupEnv("PKEXEC_UID")
	if !present || rawUID == "" {
		return PKExecIdentity{}, errors.New("PKEXEC_UID is required")
	}
	uid, err := strconv.ParseUint(rawUID, 10, 32)
	if err != nil || uid == 0 {
		return PKExecIdentity{}, errors.New("PKEXEC_UID must identify a non-root caller")
	}
	account, err := user.LookupId(strconv.FormatUint(uid, 10))
	if err != nil {
		return PKExecIdentity{}, errors.New("resolve PKEXEC_UID")
	}
	return PKExecIdentity{Username: account.Username, UID: int(uid)}, nil
}
