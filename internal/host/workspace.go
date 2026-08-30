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
	if _, err := s.Runner.Run(ctx, "usermod", []string{"--append", "--groups", project.UnixUser, person.Username}, nil, ""); err != nil {
		return nil, err
	}
	membershipCleanup := func(cleanupContext context.Context) error {
		_, err := s.Runner.Run(cleanupContext, "gpasswd", []string{"--delete", person.Username, project.UnixUser}, nil, "")
		return err
	}
	if err := os.MkdirAll(filepath.Dir(tree.Path), 0o755); err != nil {
		return nil, failWithCleanup(ctx, err, membershipCleanup)
	}
	gitCleanup, err := s.createGitWorktree(ctx, project, person, tree, baseRef)
	if err != nil {
		return nil, failWithCleanup(ctx, err, membershipCleanup)
	}
	cleanupSteps := []Cleanup{membershipCleanup, gitCleanup}
	homeCleanup, err := s.createSessionHome(ctx, project, person, tree)
	if err != nil {
		return nil, failWithCleanups(ctx, err, cleanupSteps)
	}
	cleanupSteps = append(cleanupSteps, homeCleanup)
	if err := s.prepareSharedRepository(ctx, project, s.repository(project)); err != nil {
		return nil, failWithCleanups(ctx, err, cleanupSteps)
	}
	if err := s.chownPerson(ctx, project, person, tree.Path); err != nil {
		return nil, failWithCleanups(ctx, err, cleanupSteps)
	}
	if err := os.Chmod(tree.Path, 0o700); err != nil {
		return nil, failWithCleanups(ctx, err, cleanupSteps)
	}
	return combineCleanups(cleanupSteps), nil
}

func (s *System) createGitWorktree(ctx context.Context, project domain.Project, person domain.Person, tree domain.Worktree, baseRef string) (Cleanup, error) {
	repository := s.repository(project)
	sshCommand := fmt.Sprintf("ssh -i %s -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new", s.gitPrivateKeyPath(person.Username))
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
	if _, err := s.Runner.Run(ctx, "git", []string{"-C", tree.Path, "config", "--worktree", "core.sshCommand", sshCommand}, nil, ""); err != nil {
		return nil, failWithCleanup(ctx, errors.New("configure personal workspace Git key"), cleanup)
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
	if err := s.chownPerson(ctx, project, person, personRoot); err != nil {
		_ = os.RemoveAll(personRoot)
		return nil, err
	}
	if err := os.Chmod(personRoot, 0o700); err != nil {
		_ = os.RemoveAll(personRoot)
		return nil, err
	}
	return func(context.Context) error { return os.RemoveAll(personRoot) }, nil
}

func (s *System) sessionHome(project domain.Project, person domain.Person) string {
	return filepath.Join(s.projectRoot(project), ".soda", "people", person.Username, "home")
}
