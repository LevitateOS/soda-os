package people

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
)

var ErrForgejoUserNotFound = errors.New("Forgejo account not found")

type Forgejo struct {
	Runner linuxhost.CommandRunner
}

func (forgejo Forgejo) DeleteUser(ctx context.Context, username string) error {
	result, err := forgejo.Runner.Run(ctx, linuxhost.Command{
		Name: "/usr/sbin/runuser",
		Args: []string{
			"--user", "git", "--", "/usr/bin/forgejo", "admin", "user", "delete",
			"--config", "/etc/forgejo/app.ini", "--username", username,
		},
	})
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
