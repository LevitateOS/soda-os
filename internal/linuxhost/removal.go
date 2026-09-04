package linuxhost

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type terminationStep struct {
	name string
	args []string
}

func (native *Native) PreflightDeleteAccount(ctx context.Context, account Account) error {
	_, err := native.revalidateAccountForDeletion(ctx, account)
	return err
}

func (native *Native) DeleteAccount(ctx context.Context, account Account) error {
	current, err := native.revalidateAccountForDeletion(ctx, account)
	if err != nil {
		return err
	}
	terminatedLogindUser, err := native.terminateLogindUser(ctx, current)
	if err != nil {
		return err
	}
	for _, step := range accountTerminationSteps(current) {
		if err = native.runTerminationStep(ctx, current, step); err != nil {
			return err
		}
	}
	if err = native.verifyNoOwnedProcesses(ctx, current); err != nil {
		return err
	}
	if terminatedLogindUser {
		if err = native.resetFailedUserManager(ctx, current); err != nil {
			return err
		}
	}
	current, err = native.revalidateAccountForDeletion(ctx, current)
	if err != nil {
		return err
	}
	return native.removeLinuxAccount(ctx, current)
}

func (native *Native) revalidateAccountForDeletion(ctx context.Context, expected Account) (Account, error) {
	current, err := native.LookupAccount(ctx, expected.Username)
	if err != nil {
		return Account{}, err
	}
	if !sameAccount(current, expected) {
		return Account{}, fmt.Errorf("Linux account %s changed before deletion", expected.Username)
	}
	if err = native.validateAccountHome(current); err != nil {
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

func accountTerminationSteps(account Account) []terminationStep {
	uid := strconv.Itoa(account.UID)
	return []terminationStep{
		{name: "/usr/bin/pkill", args: []string{"--signal", "TERM", "--uid", uid}},
		{name: "/usr/bin/pkill", args: []string{"--signal", "KILL", "--uid", uid}},
	}
}

func (native *Native) terminateLogindUser(ctx context.Context, account Account) (bool, error) {
	result, err := native.run(ctx, "/usr/bin/loginctl", "list-users", "--no-legend", "--no-pager")
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
	result, err = native.run(ctx, "/usr/bin/loginctl", "terminate-user", account.Username)
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

func (native *Native) runTerminationStep(ctx context.Context, account Account, step terminationStep) error {
	result, err := native.run(ctx, step.name, step.args...)
	if err != nil {
		return err
	}
	if result.ExitCode != 0 && result.ExitCode != 1 {
		return fmt.Errorf("terminate %s processes: %s", account.Username, strings.TrimSpace(result.Stderr))
	}
	return nil
}

func (native *Native) verifyNoOwnedProcesses(ctx context.Context, account Account) error {
	deadline := time.Now().Add(5 * time.Second)
	for {
		result, err := native.run(ctx, "/usr/bin/pgrep", "--uid", strconv.Itoa(account.UID))
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

func (native *Native) removeLinuxAccount(ctx context.Context, account Account) error {
	result, err := native.run(ctx, "/usr/sbin/userdel", "--remove", account.Username)
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("delete Linux account %s: %s", account.Username, strings.TrimSpace(result.Stderr))
	}
	return nil
}
