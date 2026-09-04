package projects

import (
	"context"
	"fmt"
	"strings"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
)

func (platform *NativePlatform) CreateWorkspace(ctx context.Context, primary linuxhost.Account, projectID string) (linuxhost.Account, error) {
	username, err := DerivedUsername(primary.Username, projectID)
	if err != nil {
		return linuxhost.Account{}, err
	}
	marker, err := WorkspaceMarker(primary.Username, projectID)
	if err != nil {
		return linuxhost.Account{}, err
	}
	uidMin, err := platform.host().UIDMin()
	if err != nil {
		return linuxhost.Account{}, err
	}
	result, err := platform.run(ctx, "/usr/sbin/useradd",
		"--create-home",
		"--user-group",
		"--groups", WorkspaceGroup,
		"--shell", WorkspaceShell,
		"--home-dir", "/home/"+username,
		"--comment", marker,
		"--", username,
	)
	if err != nil {
		return linuxhost.Account{}, fmt.Errorf("create workspace account %s: %w", username, err)
	}
	if result.ExitCode != 0 {
		return linuxhost.Account{}, fmt.Errorf("create workspace account %s: %s", username, strings.TrimSpace(result.Stderr))
	}
	account, err := platform.host().LookupAccount(ctx, username)
	if err != nil {
		return linuxhost.Account{}, fmt.Errorf("verify created workspace account %s: %w", username, err)
	}
	if err = validateWorkspaceAccount(account, primary.Username, projectID, uidMin); err != nil {
		return linuxhost.Account{}, fmt.Errorf("verify created workspace account %s: %w", username, err)
	}
	status, err := platform.host().PasswordStatus(ctx, account)
	if err != nil {
		return linuxhost.Account{}, err
	}
	if status != linuxhost.PasswordLocked {
		return linuxhost.Account{}, fmt.Errorf("workspace account %s does not have a locked password", username)
	}
	return account, nil
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
