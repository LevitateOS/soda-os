package host

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LevitateOS/soda-os/internal/domain"
)

func (s *System) CreateProject(ctx context.Context, project domain.Project) (Cleanup, error) {
	root := s.projectRoot(project)
	if err := s.createProjectAccount(ctx, project, root); err != nil {
		return nil, err
	}
	cleanup := s.projectCleanup(project, root)
	if err := s.createProjectKeys(ctx, project, root); err != nil {
		return nil, failWithCleanup(ctx, err, cleanup)
	}
	if err := s.initializeProjectRepository(ctx, project); err != nil {
		return nil, failWithCleanup(ctx, err, cleanup)
	}
	if err := s.finalizeProjectResources(ctx, project, root); err != nil {
		return nil, failWithCleanup(ctx, err, cleanup)
	}
	return cleanup, nil
}

func (s *System) projectCleanup(project domain.Project, root string) Cleanup {
	return func(ctx context.Context) error {
		var cleanupErrors []error
		if _, err := s.Runner.Run(ctx, "userdel", []string{"--remove", project.UnixUser}, nil, ""); err != nil {
			cleanupErrors = append(cleanupErrors, err)
		}
		if err := os.RemoveAll(root); err != nil {
			cleanupErrors = append(cleanupErrors, err)
		}
		return errors.Join(cleanupErrors...)
	}
}

func (s *System) createProjectAccount(ctx context.Context, project domain.Project, root string) error {
	_, err := s.Runner.Run(ctx, "useradd", []string{"--system", "--user-group", "--create-home", "--home-dir", root, "--shell", "/bin/bash", project.UnixUser}, nil, "")
	return err
}

func (s *System) initializeProjectRepository(ctx context.Context, project domain.Project) error {
	if _, empty := project.Source.(domain.EmptyProjectSource); !empty {
		return nil
	}
	return s.initializeEmptyRepository(ctx, s.repository(project))
}

func (s *System) finalizeProjectResources(ctx context.Context, project domain.Project, root string) error {
	if _, err := s.Runner.Run(ctx, "restorecon", []string{"-R", root}, nil, ""); err != nil {
		return err
	}
	return s.chown(ctx, project, root)
}

func (s *System) EnsureRepository(ctx context.Context, project domain.Project, bootstrap domain.Person) error {
	repository := s.repository(project)
	if info, err := os.Stat(repository); err == nil && info.IsDir() {
		return s.prepareSharedRepository(ctx, project, repository)
	}
	source, ok := project.Source.(domain.GitProjectSource)
	if !ok {
		return fmt.Errorf("project %s repository is missing", project.Slug)
	}
	if bootstrap.ID == "" {
		return fmt.Errorf("project %s has no bootstrap person", project.Slug)
	}
	key := s.gitPrivateKeyPath(bootstrap.Username)
	environment := map[string]string{"GIT_SSH_COMMAND": fmt.Sprintf("ssh -i %s -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new", key)}
	if _, err := s.Runner.Run(ctx, "git", []string{"clone", "--bare", source.RemoteURL, repository}, environment, ""); err != nil {
		_ = os.RemoveAll(repository)
		return errors.New("external repository clone failed; verify that the bootstrap Git account has repository access")
	}
	if err := s.prepareSharedRepository(ctx, project, repository); err != nil {
		_ = os.RemoveAll(repository)
		return err
	}
	return nil
}

func (s *System) prepareSharedRepository(ctx context.Context, project domain.Project, repository string) error {
	if err := s.chown(ctx, project, repository); err != nil {
		return err
	}
	if _, err := s.Runner.Run(ctx, "chmod", []string{"--recursive", "g+rwX", repository}, nil, ""); err != nil {
		return err
	}
	if _, err := s.Runner.Run(ctx, "find", []string{repository, "-type", "d", "-exec", "chmod", "g+s", "{}", "+"}, nil, ""); err != nil {
		return err
	}
	_, err := s.Runner.Run(ctx, "git", []string{"--git-dir", repository, "config", "core.sharedRepository", "group"}, nil, "")
	return err
}

func (s *System) DefaultBranch(ctx context.Context, project domain.Project) (string, error) {
	output, err := s.Runner.Run(ctx, "git", []string{"--git-dir", s.repository(project), "symbolic-ref", "--quiet", "--short", "HEAD"}, nil, "")
	if err != nil {
		return "", fmt.Errorf("resolve repository default branch: %w", err)
	}
	branch := strings.TrimSpace(output)
	if branch == "" || strings.ContainsAny(branch, "\r\n\x00") {
		return "", errors.New("repository default branch is unavailable")
	}
	return branch, nil
}

func (s *System) WriteProjectEnvironment(ctx context.Context, project domain.Project, source string) error {
	contents, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("read generated project environment: %w", err)
	}
	dir := filepath.Join(s.projectRoot(project), ".soda")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "environment.json"), contents, 0o644); err != nil {
		return err
	}
	return s.chown(ctx, project, dir)
}

func (s *System) DeployPublicKey(_ context.Context, project domain.Project) (string, error) {
	contents, err := os.ReadFile(filepath.Join(s.projectRoot(project), ".ssh", "deploy_key.pub"))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(contents)), nil
}

func (s *System) ConnectBuiltInRepository(ctx context.Context, project domain.Project, remote string) error {
	repository := s.repository(project)
	if _, err := s.Runner.Run(ctx, "git", []string{"--git-dir", repository, "remote", "get-url", "origin"}, nil, ""); err != nil {
		if _, err = s.Runner.Run(ctx, "git", []string{"--git-dir", repository, "remote", "add", "origin", remote}, nil, ""); err != nil {
			return err
		}
	}
	key := filepath.Join(s.projectRoot(project), ".ssh", "deploy_key")
	environment := map[string]string{"GIT_SSH_COMMAND": fmt.Sprintf("ssh -i %s -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new", key)}
	if _, err := s.Runner.Run(ctx, "git", []string{"--git-dir", repository, "push", "--set-upstream", "origin", "main"}, environment, ""); err != nil {
		return fmt.Errorf("publish Built-in Git repository: %w", err)
	}
	return nil
}

func (s *System) initializeEmptyRepository(ctx context.Context, repository string) error {
	if _, err := s.Runner.Run(ctx, "git", []string{"init", "--bare", "--initial-branch=main", repository}, nil, ""); err != nil {
		return err
	}
	tree, err := s.Runner.Run(ctx, "git", []string{"--git-dir", repository, "mktree"}, nil, "")
	if err != nil {
		return err
	}
	environment := map[string]string{"GIT_AUTHOR_NAME": "Soda OS", "GIT_AUTHOR_EMAIL": "soda@soda.local", "GIT_COMMITTER_NAME": "Soda OS", "GIT_COMMITTER_EMAIL": "soda@soda.local"}
	commit, err := s.Runner.Run(ctx, "git", []string{"--git-dir", repository, "commit-tree", strings.TrimSpace(tree), "-m", "Initialize Soda project"}, environment, "")
	if err != nil {
		return err
	}
	_, err = s.Runner.Run(ctx, "git", []string{"--git-dir", repository, "update-ref", "refs/heads/main", strings.TrimSpace(commit)}, nil, "")
	return err
}
