package workspace

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
)

// validateLocalGit checks repository metadata without walking working files,
// running filters/hooks, refreshing the index, or fetching missing objects.
// Dirty checkouts and unborn branches are ordinary Git states, not setup failures.
func (repository Repository) validateLocalGit(ctx context.Context, account linuxhost.Account, path string) error {
	result, err := repository.inspectGit(ctx, account, path, "rev-parse", "--is-inside-work-tree", "--show-prefix")
	if err != nil {
		return err
	}
	if result.ExitCode != 0 || strings.TrimSpace(result.Stdout) != "true" {
		return fmt.Errorf("workspace is not a usable Git working tree: %s", strings.TrimSpace(result.Stderr))
	}
	result, err = repository.inspectGit(ctx, account, path, "cat-file", "-t", "HEAD")
	if err != nil {
		return err
	}
	if result.ExitCode == 0 && strings.TrimSpace(result.Stdout) == "commit" {
		return nil
	}
	return repository.validateUnbornHead(ctx, account, path)
}

func (repository Repository) validateUnbornHead(ctx context.Context, account linuxhost.Account, path string) error {
	result, err := repository.inspectGit(ctx, account, path, "symbolic-ref", "--quiet", "HEAD")
	if err != nil {
		return err
	}
	ref := strings.TrimSpace(result.Stdout)
	if result.ExitCode != 0 || !strings.HasPrefix(ref, "refs/heads/") {
		return errors.New("workspace Git HEAD cannot be read as a commit or an unborn branch")
	}
	result, err = repository.inspectGit(ctx, account, path, "show-ref", "--verify", "--quiet", ref)
	if err != nil {
		return err
	}
	if result.ExitCode != 1 {
		return errors.New("workspace Git HEAD references an unreadable commit")
	}
	return nil
}

func (repository Repository) inspectGit(ctx context.Context, account linuxhost.Account, path string, arguments ...string) (linuxhost.CommandResult, error) {
	args := []string{
		"--user", account.Username, "--", "/usr/bin/env", "-i",
		"HOME=" + account.Home, "USER=" + account.Username, "LOGNAME=" + account.Username,
		"PATH=/usr/local/bin:/usr/bin:/bin", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_NO_LAZY_FETCH=1", "GIT_OPTIONAL_LOCKS=0",
		"/usr/bin/git", "-C", path,
	}
	return repository.runner.Run(ctx, linuxhost.Command{Name: "/usr/sbin/runuser", Args: append(args, arguments...)})
}
