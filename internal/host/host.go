package host

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/LevitateOS/soda-os/internal/domain"
)

var ErrNotFound = errors.New("host resource not found")

const DefaultAuthorizedKeysRoot = "/etc/soda/authorized_keys"

type Cleanup func(context.Context) error

type Operations interface {
	CreatePerson(context.Context, domain.Person, string) (Cleanup, error)
	ImportPerson(context.Context, domain.Person) (Cleanup, error)
	CreateProject(context.Context, domain.Project) (Cleanup, error)
	EnsureRepository(context.Context, domain.Project) error
	DefaultBranch(context.Context, domain.Project) (string, error)
	CreateWorktree(context.Context, domain.Project, domain.Person, domain.Worktree, string) (Cleanup, error)
	ReconcileAuthorizedKeys(context.Context, domain.Project, []domain.ProjectAccess) error
	WriteProjectEnvironment(context.Context, domain.Project, string) error
	DeployPublicKey(context.Context, domain.Project) (string, error)
}

type Runner interface {
	Run(context.Context, string, []string, map[string]string, string) (string, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args []string, environment map[string]string, input string) (string, error) {
	command := exec.CommandContext(ctx, name, args...)
	if len(environment) != 0 {
		command.Env = os.Environ()
		for key, value := range environment {
			command.Env = append(command.Env, key+"="+value)
		}
	}
	if input != "" {
		command.Stdin = strings.NewReader(input)
	}
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		return "", fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	if stdout.Len() == 0 {
		return stderr.String(), nil
	}
	return stdout.String(), nil
}

type System struct {
	ProjectsRoot       string
	Runner             Runner
	AuthorizedKeysRoot string
	authorizedKeysMu   sync.Mutex
}

func New(projectsRoot string) *System {
	return &System{ProjectsRoot: projectsRoot, Runner: ExecRunner{}, AuthorizedKeysRoot: DefaultAuthorizedKeysRoot}
}

func (s *System) CreatePerson(ctx context.Context, person domain.Person, password string) (Cleanup, error) {
	if strings.ContainsAny(password, "\r\n\x00") {
		return nil, errors.New("password contains a line or NUL delimiter")
	}
	if utf8.RuneCountInString(password) < 6 {
		return nil, errors.New("password must contain at least 6 characters")
	}
	if _, err := s.Runner.Run(ctx, "useradd", []string{"--create-home", "--shell", "/sbin/nologin", person.Username}, nil, ""); err != nil {
		return nil, err
	}
	cleanup := func(cleanupContext context.Context) error {
		_, err := s.Runner.Run(cleanupContext, "userdel", []string{"--remove", person.Username}, nil, "")
		return err
	}
	if _, err := s.Runner.Run(ctx, "chpasswd", nil, nil, person.Username+":"+password+"\n"); err != nil {
		return nil, failWithCleanup(ctx, err, cleanup)
	}
	if _, err := s.Runner.Run(ctx, "chage", []string{"--lastday", "0", person.Username}, nil, ""); err != nil {
		return nil, failWithCleanup(ctx, err, cleanup)
	}
	return cleanup, nil
}

func (s *System) ImportPerson(ctx context.Context, person domain.Person) (Cleanup, error) {
	if _, err := s.Runner.Run(ctx, "getent", []string{"passwd", person.Username}, nil, ""); err != nil {
		return nil, fmt.Errorf("%w: Linux account %s", ErrNotFound, person.Username)
	}
	return func(context.Context) error { return nil }, nil
}

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
		if err := os.Remove(s.authorizedKeysPath(project)); err != nil && !errors.Is(err, os.ErrNotExist) {
			cleanupErrors = append(cleanupErrors, err)
		}
		return errors.Join(cleanupErrors...)
	}
}

func (s *System) createProjectAccount(ctx context.Context, project domain.Project, root string) error {
	_, err := s.Runner.Run(ctx, "useradd", []string{"--system", "--create-home", "--home-dir", root, "--shell", "/bin/bash", project.UnixUser}, nil, "")
	return err
}

func (s *System) createProjectKeys(ctx context.Context, project domain.Project, root string) error {
	sshDir := filepath.Join(root, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(sshDir, 0o700); err != nil {
		return err
	}
	if _, err := s.Runner.Run(ctx, "ssh-keygen", []string{"-q", "-t", "ed25519", "-N", "", "-C", "soda-project-" + project.Slug, "-f", filepath.Join(sshDir, "deploy_key")}, nil, ""); err != nil {
		return err
	}
	return s.createAuthorizedKeysFile(project)
}

func (s *System) createAuthorizedKeysFile(project domain.Project) error {
	if err := os.MkdirAll(s.AuthorizedKeysRoot, 0o755); err != nil {
		return err
	}
	if err := os.Chmod(s.AuthorizedKeysRoot, 0o755); err != nil {
		return err
	}
	keyFile := s.authorizedKeysPath(project)
	file, err := os.OpenFile(keyFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Chmod(keyFile, 0o644)
}

func (s *System) initializeProjectRepository(ctx context.Context, project domain.Project) error {
	if _, empty := project.Source.(domain.EmptyProjectSource); !empty {
		return nil
	}
	return s.initializeEmptyRepository(ctx, s.repository(project))
}

func (s *System) finalizeProjectResources(ctx context.Context, project domain.Project, root string) error {
	keyFile := s.authorizedKeysPath(project)
	if _, err := s.Runner.Run(ctx, "chown", []string{"root:root", s.AuthorizedKeysRoot, keyFile}, nil, ""); err != nil {
		return err
	}
	if _, err := s.Runner.Run(ctx, "restorecon", []string{"-R", root}, nil, ""); err != nil {
		return err
	}
	if _, err := s.Runner.Run(ctx, "restorecon", []string{"-R", s.AuthorizedKeysRoot}, nil, ""); err != nil {
		return err
	}
	return s.chown(ctx, project, root)
}

func (s *System) EnsureRepository(ctx context.Context, project domain.Project) error {
	repository := s.repository(project)
	if info, err := os.Stat(repository); err == nil && info.IsDir() {
		return nil
	}
	source, ok := project.Source.(domain.GitProjectSource)
	if !ok {
		return fmt.Errorf("project %s repository is missing", project.Slug)
	}
	key := filepath.Join(s.projectRoot(project), ".ssh", "deploy_key")
	environment := map[string]string{"GIT_SSH_COMMAND": fmt.Sprintf("ssh -i %s -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new", key)}
	if _, err := s.Runner.Run(ctx, "git", []string{"clone", "--bare", source.RemoteURL, repository}, environment, ""); err != nil {
		_ = os.RemoveAll(repository)
		return err
	}
	if err := s.chown(ctx, project, repository); err != nil {
		_ = os.RemoveAll(repository)
		return err
	}
	return nil
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

func (s *System) ReconcileAuthorizedKeys(ctx context.Context, project domain.Project, access []domain.ProjectAccess) error {
	s.authorizedKeysMu.Lock()
	defer s.authorizedKeysMu.Unlock()
	return s.writeAuthorizedKeys(ctx, s.authorizedKeysPath(project), s.authorizedKeyContents(project, access))
}

func (s *System) authorizedKeyContents(project domain.Project, access []domain.ProjectAccess) string {
	sort.Slice(access, func(i, j int) bool { return access[i].Person.Username < access[j].Person.Username })
	lines := make([]string, 0)
	for _, entry := range access {
		lines = append(lines, s.authorizedKeyLines(project, entry)...)
	}
	if len(lines) != 0 {
		return strings.Join(lines, "\n") + "\n"
	}
	return ""
}

func (s *System) authorizedKeyLines(project domain.Project, access domain.ProjectAccess) []string {
	keys := append([]domain.SSHDeviceKey(nil), access.Keys...)
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Label == keys[j].Label {
			return keys[i].Fingerprint < keys[j].Fingerprint
		}
		return keys[i].Label < keys[j].Label
	})
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, s.authorizedKeyLine(project, access.Person, access.Worktree, key))
	}
	return lines
}

func (s *System) authorizedKeyLine(project domain.Project, person domain.Person, tree domain.Worktree, key domain.SSHDeviceKey) string {
	home := s.sessionHome(project, person)
	command := fmt.Sprintf("/usr/libexec/soda/soda-ssh --actor %s --project %s --worktree %s --home %s", person.Username, project.Slug, tree.Path, home)
	return fmt.Sprintf("command=\"%s\" %s", command, key.PublicKey)
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

func (s *System) projectRoot(project domain.Project) string {
	return filepath.Join(s.ProjectsRoot, project.Slug)
}
func (s *System) repository(project domain.Project) string {
	return filepath.Join(s.projectRoot(project), "repository.git")
}
func (s *System) authorizedKeysPath(project domain.Project) string {
	return filepath.Join(s.AuthorizedKeysRoot, project.UnixUser)
}
func (s *System) chown(ctx context.Context, project domain.Project, path string) error {
	_, err := s.Runner.Run(ctx, "chown", []string{"--recursive", project.UnixUser + ":" + project.UnixUser, path}, nil, "")
	return err
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

func (s *System) writeAuthorizedKeys(ctx context.Context, path, contents string) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".authorized-keys-")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	// OpenSSH reads this root-owned file after switching to the project account.
	// Public keys are not secret; world-readability preserves root-only writes.
	if err = temporary.Chmod(0o644); err == nil {
		_, err = temporary.WriteString(contents)
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(temporaryPath, path); err != nil {
		return err
	}
	if _, err = s.Runner.Run(ctx, "chown", []string{"root:root", path}, nil, ""); err == nil {
		_, err = s.Runner.Run(ctx, "restorecon", []string{path}, nil, "")
	}
	return err
}

func combineCleanups(cleanups []Cleanup) Cleanup {
	return func(ctx context.Context) error {
		var cleanupErrors []error
		for index := len(cleanups) - 1; index >= 0; index-- {
			if err := cleanups[index](ctx); err != nil {
				cleanupErrors = append(cleanupErrors, err)
			}
		}
		return errors.Join(cleanupErrors...)
	}
}

func failWithCleanup(ctx context.Context, operationErr error, cleanup Cleanup) error {
	return failWithCleanups(ctx, operationErr, []Cleanup{cleanup})
}

func failWithCleanups(ctx context.Context, operationErr error, cleanups []Cleanup) error {
	cleanupContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()
	if cleanupErr := combineCleanups(cleanups)(cleanupContext); cleanupErr != nil {
		return errors.Join(operationErr, fmt.Errorf("cleanup failed: %w", cleanupErr))
	}
	return operationErr
}
