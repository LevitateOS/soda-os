package observe

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/LevitateOS/soda-os/internal/domain"
)

type GitCommandRunner interface {
	Output(context.Context, string, []string, []string) ([]byte, error)
}

type ExecGitCommandRunner struct{}

func (ExecGitCommandRunner) Output(ctx context.Context, command string, args, env []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Env = append(os.Environ(), env...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return output, nil
}

// SystemGitInspector runs porcelain v2 under the project Unix identity when
// sodad runs as root. Its runner is injectable so callers can prove the exact
// command and environment without shelling out.
type SystemGitInspector struct {
	Runner GitCommandRunner
	UID    func() int
	Lookup func(string) error
}

func NewSystemGitInspector(runner GitCommandRunner) *SystemGitInspector {
	if runner == nil {
		runner = ExecGitCommandRunner{}
	}
	return &SystemGitInspector{
		Runner: runner,
		UID:    os.Geteuid,
		Lookup: func(user string) error {
			return exec.Command("id", "-u", user).Run()
		},
	}
}

func (s *SystemGitInspector) Inspect(ctx context.Context, project domain.Project, worktree domain.Worktree) domain.WorktreeStatus {
	args := []string{"-C", worktree.Path, "status", "--porcelain=v2", "--branch", "--untracked-files=normal"}
	command := "git"
	if s.UID() == 0 && s.Lookup(project.UnixUser) == nil {
		command = "runuser"
		args = append([]string{"--user", project.UnixUser, "--", "git"}, args...)
	}
	output, err := s.Runner.Output(ctx, command, args, []string{"GIT_OPTIONAL_LOCKS=0", "LC_ALL=C"})
	if err != nil {
		return unavailableWorktree(worktree, err)
	}
	return ParseGitStatus(worktree, output)
}

// ParseGitStatus converts porcelain v2 output into totals only. It never
// returns filenames, because Cockpit's observability view intentionally does
// not disclose project content.
func ParseGitStatus(worktree domain.Worktree, output []byte) domain.WorktreeStatus {
	status := domain.WorktreeStatus{WorktreeID: worktree.ID, Branch: worktree.Branch, State: domain.WorktreeClean}
	for _, line := range strings.Split(string(output), "\n") {
		switch {
		case strings.HasPrefix(line, "# branch.head "):
			status.Branch = strings.TrimPrefix(line, "# branch.head ")
		case strings.HasPrefix(line, "# branch.oid "):
			status.ShortCommit = truncate(strings.TrimPrefix(line, "# branch.oid "), 12)
		case strings.HasPrefix(line, "# branch.upstream "):
			value := strings.TrimPrefix(line, "# branch.upstream ")
			status.Upstream = &value
		case strings.HasPrefix(line, "# branch.ab "):
			for _, field := range strings.Fields(strings.TrimPrefix(line, "# branch.ab ")) {
				if value, ok := strings.CutPrefix(field, "+"); ok {
					fmt.Sscan(value, &status.Ahead)
				}
				if value, ok := strings.CutPrefix(field, "-"); ok {
					fmt.Sscan(value, &status.Behind)
				}
			}
		case strings.HasPrefix(line, "u "):
			status.Conflicted++
		case strings.HasPrefix(line, "? "):
			status.Untracked++
		case strings.HasPrefix(line, "1 ") || strings.HasPrefix(line, "2 "):
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			xy := fields[1]
			if len(xy) >= 1 && xy[0] != '.' {
				status.Staged++
			}
			if len(xy) >= 2 && xy[1] != '.' {
				status.Modified++
			}
		}
	}
	if status.Staged+status.Modified+status.Untracked+status.Conflicted > 0 {
		status.State = domain.WorktreeDirty
	}
	return status
}

func truncate(value string, length int) string {
	if len(value) <= length {
		return value
	}
	return value[:length]
}
