package projects

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
)

func (platform *NativePlatform) CreatePrimary(ctx context.Context, username, password string) (Account, error) {
	if err := ValidatePrimaryUsername(username); err != nil {
		return Account{}, err
	}
	if err := validateHumanPassword(password); err != nil {
		return Account{}, err
	}
	uidMin, err := platform.UIDMin()
	if err != nil {
		return Account{}, err
	}
	existing, found, err := platform.existingPrimary(ctx, username, uidMin)
	if err != nil || found {
		return existing, err
	}
	if err = platform.createPrimaryAccount(ctx, username); err != nil {
		return Account{}, err
	}
	if err = platform.setInitialPassword(ctx, username, password); err != nil {
		return Account{}, err
	}
	return platform.verifiedPrimary(ctx, username, uidMin)
}

func (platform *NativePlatform) existingPrimary(ctx context.Context, username string, uidMin int) (Account, bool, error) {
	account, err := platform.LookupAccount(ctx, username)
	if errors.Is(err, ErrAccountNotFound) {
		return Account{}, false, nil
	}
	if err != nil {
		return Account{}, false, err
	}
	if err = validateSupportedNewPrimary(account, username, uidMin); err != nil {
		return Account{}, true, err
	}
	return account, true, platform.validatePasswordUsable(ctx, account)
}

func (platform *NativePlatform) createPrimaryAccount(ctx context.Context, username string) error {
	result, err := platform.run(ctx, "/usr/sbin/useradd", "--create-home", "--user-group",
		"--shell", WorkspaceShell, "--home-dir", "/home/"+username, "--", username)
	if err != nil {
		return fmt.Errorf("create primary account %s: %w", username, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("create primary account %s: %s", username, strings.TrimSpace(result.Stderr))
	}
	return nil
}

func (platform *NativePlatform) setInitialPassword(ctx context.Context, username, password string) error {
	result, err := platform.runner().Run(ctx, Command{
		Name: "/usr/bin/passwd", Args: []string{"--stdin", "--", username}, Input: strings.NewReader(password + "\n"),
	})
	if err != nil {
		return fmt.Errorf("set initial Linux password for retained account %s: %w", username, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("set initial Linux password for retained account %s: %s", username, strings.TrimSpace(result.Stderr))
	}
	return nil
}

func (platform *NativePlatform) verifiedPrimary(ctx context.Context, username string, uidMin int) (Account, error) {
	account, err := platform.LookupAccount(ctx, username)
	if err != nil {
		return Account{}, fmt.Errorf("verify created primary account %s: %w", username, err)
	}
	if err = validateSupportedNewPrimary(account, username, uidMin); err != nil {
		return Account{}, err
	}
	if err = platform.validatePasswordUsable(ctx, account); err != nil {
		return Account{}, err
	}
	return account, nil
}

func validateSupportedNewPrimary(account Account, username string, uidMin int) error {
	if account.Username != username || !account.IsPrimary(uidMin) || account.PrimaryGroup != username ||
		account.Home != "/home/"+username || account.Shell != WorkspaceShell || account.Groups["wheel"] || account.Groups[WorkspaceGroup] {
		return fmt.Errorf("Linux account %s does not have the supported ordinary primary-account shape", username)
	}
	return nil
}

func (platform *NativePlatform) validatePasswordUsable(ctx context.Context, account Account) error {
	result, err := platform.run(ctx, "/usr/bin/passwd", "--status", account.Username)
	if err != nil {
		return fmt.Errorf("read Linux password status for %s: %w", account.Username, err)
	}
	fields := strings.Fields(result.Stdout)
	if result.ExitCode != 0 || len(fields) < 2 || fields[0] != account.Username || fields[1] != "P" {
		return fmt.Errorf("primary account %s does not have a usable Linux password", account.Username)
	}
	return nil
}

func (platform *NativePlatform) PublishHuman(ctx context.Context, username string, authorizedKey []byte) error {
	if err := ValidatePrimaryUsername(username); err != nil {
		return err
	}
	uidMin, err := platform.UIDMin()
	if err != nil {
		return err
	}
	target, err := platform.LookupAccount(ctx, username)
	if err != nil {
		return err
	}
	if err = validateSupportedNewPrimary(target, username, uidMin); err != nil {
		return err
	}
	return platform.installAuthorizedKeysIdempotent(target, append(bytes.TrimSpace(authorizedKey), '\n'))
}

func (platform *NativePlatform) installAuthorizedKeysIdempotent(account Account, contents []byte) error {
	if existing, err := platform.ReadAuthorizedKeys(account); err == nil {
		if bytes.Equal(existing, contents) {
			return nil
		}
		return errors.New("refusing to overwrite a different authorized_keys file")
	}
	return platform.InstallAuthorizedKeys(account, contents)
}

func (platform *NativePlatform) DeleteForgejoUser(ctx context.Context, username string) error {
	if err := ValidatePrimaryUsername(username); err != nil {
		return err
	}
	result, err := platform.run(ctx, "/usr/sbin/runuser", "--user", "git", "--",
		"/usr/bin/forgejo", "admin", "user", "delete", "--config", "/etc/forgejo/app.ini", "--username", username)
	if err != nil {
		return fmt.Errorf("run native Forgejo deletion for %s: %w", username, err)
	}
	if result.ExitCode == 0 {
		return nil
	}
	diagnostic := strings.TrimSpace(result.Stderr)
	if strings.Contains(diagnostic, "user does not exist") {
		return fmt.Errorf("%w: %s", ErrForgejoUserNotFound, username)
	}
	return fmt.Errorf("native Forgejo deletion for %s: %s", username, diagnostic)
}
