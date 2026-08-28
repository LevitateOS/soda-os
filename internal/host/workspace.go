package host

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/LevitateOS/soda-os/internal/domain"
)

func (s *System) CreateWorktree(ctx context.Context, project domain.Project, person domain.Person, tree domain.Worktree, baseRef string) (Cleanup, error) {
	if err := os.MkdirAll(filepath.Dir(tree.Path), 0o755); err != nil {
		return nil, err
	}
	if err := s.chown(ctx, project, filepath.Dir(tree.Path)); err != nil {
		return nil, err
	}
	gitCleanup, err := s.createGitWorktree(ctx, project, person, tree, baseRef)
	if err != nil {
		return nil, err
	}
	cleanupSteps := []Cleanup{gitCleanup}
	homeCleanup, err := s.createSessionHome(ctx, project, person, tree)
	if err != nil {
		return nil, failWithCleanups(ctx, err, cleanupSteps)
	}
	cleanupSteps = append(cleanupSteps, homeCleanup)
	for _, path := range []string{s.repository(project), tree.Path} {
		if err := s.chown(ctx, project, path); err != nil {
			return nil, failWithCleanups(ctx, err, cleanupSteps)
		}
	}
	return combineCleanups(cleanupSteps), nil
}

func (s *System) createGitWorktree(ctx context.Context, project domain.Project, person domain.Person, tree domain.Worktree, baseRef string) (Cleanup, error) {
	repository := s.repository(project)
	commands := [][]string{{"--git-dir", repository, "worktree", "add", "-b", tree.Branch, tree.Path, baseRef}, {"--git-dir", repository, "config", "extensions.worktreeConfig", "true"}, {"-C", tree.Path, "config", "--worktree", "core.bare", "false"}, {"-C", tree.Path, "config", "--worktree", "user.name", person.DisplayName}, {"-C", tree.Path, "config", "--worktree", "user.email", person.Email}}
	var cleanup Cleanup
	for index, args := range commands {
		if _, err := s.Runner.Run(ctx, "git", args, nil, ""); err != nil {
			if index == 0 {
				return nil, err
			}
			return nil, failWithCleanup(ctx, err, cleanup)
		}
		if index == 0 {
			cleanup = s.gitWorktreeCleanup(repository, tree)
		}
	}
	return cleanup, nil
}

func (s *System) gitWorktreeCleanup(repository string, tree domain.Worktree) Cleanup {
	return func(cleanupContext context.Context) error {
		_, removeErr := s.Runner.Run(cleanupContext, "git", []string{"--git-dir", repository, "worktree", "remove", "--force", tree.Path}, nil, "")
		_, branchErr := s.Runner.Run(cleanupContext, "git", []string{"--git-dir", repository, "branch", "-D", tree.Branch}, nil, "")
		_ = os.Remove(tree.Path)
		_ = os.Remove(filepath.Dir(tree.Path))
		return errors.Join(removeErr, branchErr)
	}
}

func (s *System) createSessionHome(ctx context.Context, project domain.Project, person domain.Person, tree domain.Worktree) (Cleanup, error) {
	peopleRoot := filepath.Join(s.projectRoot(project), ".soda", "people")
	personRoot := filepath.Join(peopleRoot, person.Username)
	home := s.sessionHome(project, person)
	for _, path := range []string{home, filepath.Join(home, ".config"), filepath.Join(home, ".cache"), filepath.Join(home, ".local", "share"), filepath.Join(home, ".local", "state")} {
		if err := os.MkdirAll(path, 0o750); err != nil {
			_ = os.RemoveAll(personRoot)
			return nil, err
		}
	}
	profile := "if [ -f \"$HOME/.bashrc\" ]; then . \"$HOME/.bashrc\"; fi\n"
	rc := fmt.Sprintf("case $- in *i*) ;; *) return;; esac\nPS1='%s@%s workspace \\$ '\n", person.Username, project.Slug)
	for path, contents := range map[string]string{filepath.Join(home, ".bash_profile"): profile, filepath.Join(home, ".bashrc"): rc} {
		if err := os.WriteFile(path, []byte(contents), 0o640); err != nil {
			_ = os.RemoveAll(personRoot)
			return nil, err
		}
	}
	workspace := filepath.Join(home, "workspace")
	if err := os.Symlink(tree.Path, workspace); err != nil && !errors.Is(err, os.ErrExist) {
		_ = os.RemoveAll(personRoot)
		return nil, err
	}
	if err := s.chown(ctx, project, peopleRoot); err != nil {
		_ = os.RemoveAll(personRoot)
		return nil, err
	}
	return func(context.Context) error { return os.RemoveAll(personRoot) }, nil
}

func (s *System) sessionHome(project domain.Project, person domain.Person) string {
	return filepath.Join(s.projectRoot(project), ".soda", "people", person.Username, "home")
}
