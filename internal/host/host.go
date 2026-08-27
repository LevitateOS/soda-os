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
	ManageAccounts     bool
	Runner             Runner
	AuthorizedKeysRoot string
	authorizedKeysMu   sync.Mutex
}

func New(projectsRoot string, manageAccounts bool) *System {
	authorizedKeysRoot := DefaultAuthorizedKeysRoot
	if !manageAccounts {
		authorizedKeysRoot = filepath.Join(projectsRoot, ".authorized_keys")
	}
	return &System{ProjectsRoot: projectsRoot, ManageAccounts: manageAccounts, Runner: ExecRunner{}, AuthorizedKeysRoot: authorizedKeysRoot}
}

func (s *System) CreatePerson(ctx context.Context, person domain.Person, password string) (Cleanup, error) {
	if strings.ContainsAny(password, "\r\n\x00") {
		return nil, errors.New("password contains a line or NUL delimiter")
	}
	if utf8.RuneCountInString(password) < 6 {
		return nil, errors.New("password must contain at least 6 characters")
	}
	if !s.ManageAccounts {
		return noopCleanup, nil
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
	if !s.ManageAccounts {
		return noopCleanup, nil
	}
	if _, err := s.Runner.Run(ctx, "getent", []string{"passwd", person.Username}, nil, ""); err != nil {
		return nil, fmt.Errorf("%w: Linux account %s", ErrNotFound, person.Username)
	}
	return noopCleanup, nil
}

func (s *System) CreateProject(ctx context.Context, project domain.Project) (Cleanup, error) {
	root := s.projectRoot(project)
	createdAccount := false
	if s.ManageAccounts {
		if _, err := s.Runner.Run(ctx, "useradd", []string{"--system", "--create-home", "--home-dir", root, "--shell", "/bin/bash", project.UnixUser}, nil, ""); err != nil {
			return nil, err
		}
		createdAccount = true
	} else if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	cleanup := func(cleanupContext context.Context) error {
		var cleanupErrors []error
		if createdAccount {
			if _, err := s.Runner.Run(cleanupContext, "userdel", []string{"--remove", project.UnixUser}, nil, ""); err != nil {
				cleanupErrors = append(cleanupErrors, err)
			}
		}
		if err := os.RemoveAll(root); err != nil {
			cleanupErrors = append(cleanupErrors, err)
		}
		if err := os.Remove(s.authorizedKeysPath(project)); err != nil && !errors.Is(err, os.ErrNotExist) {
			cleanupErrors = append(cleanupErrors, err)
		}
		return errors.Join(cleanupErrors...)
	}
	sshDir := filepath.Join(root, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return nil, failWithCleanup(ctx, err, cleanup)
	}
	if err := os.Chmod(sshDir, 0o700); err != nil {
		return nil, failWithCleanup(ctx, err, cleanup)
	}
	if _, err := s.Runner.Run(ctx, "ssh-keygen", []string{"-q", "-t", "ed25519", "-N", "", "-C", "soda-project-" + project.Slug, "-f", filepath.Join(sshDir, "deploy_key")}, nil, ""); err != nil {
		return nil, failWithCleanup(ctx, err, cleanup)
	}
	if err := os.MkdirAll(s.AuthorizedKeysRoot, 0o755); err != nil {
		return nil, failWithCleanup(ctx, err, cleanup)
	}
	if err := os.Chmod(s.AuthorizedKeysRoot, 0o755); err != nil {
		return nil, failWithCleanup(ctx, err, cleanup)
	}
	keyFile := s.authorizedKeysPath(project)
	file, err := os.OpenFile(keyFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, failWithCleanup(ctx, err, cleanup)
	}
	if err = file.Close(); err != nil {
		return nil, failWithCleanup(ctx, err, cleanup)
	}
	if err = os.Chmod(keyFile, 0o600); err != nil {
		return nil, failWithCleanup(ctx, err, cleanup)
	}
	if _, ok := project.Source.(domain.EmptyProjectSource); ok {
		if err = s.initializeEmptyRepository(ctx, s.repository(project)); err != nil {
			return nil, failWithCleanup(ctx, err, cleanup)
		}
	}
	if s.ManageAccounts {
		if _, err = s.Runner.Run(ctx, "chown", []string{"root:root", s.AuthorizedKeysRoot, keyFile}, nil, ""); err != nil {
			return nil, failWithCleanup(ctx, err, cleanup)
		}
		if _, err = s.Runner.Run(ctx, "restorecon", []string{"-R", root}, nil, ""); err != nil {
			return nil, failWithCleanup(ctx, err, cleanup)
		}
		if _, err = s.Runner.Run(ctx, "restorecon", []string{"-R", s.AuthorizedKeysRoot}, nil, ""); err != nil {
			return nil, failWithCleanup(ctx, err, cleanup)
		}
	}
	if err = s.chown(ctx, project, root); err != nil {
		return nil, failWithCleanup(ctx, err, cleanup)
	}
	return cleanup, nil
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

func (s *System) CreateWorktree(ctx context.Context, project domain.Project, person domain.Person, tree domain.Worktree, baseRef string) (Cleanup, error) {
	repository := s.repository(project)
	if err := os.MkdirAll(filepath.Dir(tree.Path), 0o755); err != nil {
		return nil, err
	}
	if err := s.chown(ctx, project, filepath.Dir(tree.Path)); err != nil {
		return nil, err
	}
	commands := [][]string{{"--git-dir", repository, "worktree", "add", "-b", tree.Branch, tree.Path, baseRef}, {"--git-dir", repository, "config", "extensions.worktreeConfig", "true"}, {"-C", tree.Path, "config", "--worktree", "core.bare", "false"}, {"-C", tree.Path, "config", "--worktree", "user.name", person.DisplayName}, {"-C", tree.Path, "config", "--worktree", "user.email", person.Email}}
	var cleanupSteps []Cleanup
	for index, args := range commands {
		if _, err := s.Runner.Run(ctx, "git", args, nil, ""); err != nil {
			return nil, failWithCleanups(ctx, err, cleanupSteps)
		}
		if index == 0 {
			cleanupSteps = append(cleanupSteps, func(cleanupContext context.Context) error {
				_, removeErr := s.Runner.Run(cleanupContext, "git", []string{"--git-dir", repository, "worktree", "remove", "--force", tree.Path}, nil, "")
				_, branchErr := s.Runner.Run(cleanupContext, "git", []string{"--git-dir", repository, "branch", "-D", tree.Branch}, nil, "")
				_ = os.Remove(tree.Path)
				_ = os.Remove(filepath.Dir(tree.Path))
				return errors.Join(removeErr, branchErr)
			})
		}
	}
	homeCleanup, err := s.createSessionHome(ctx, project, person, tree)
	if err != nil {
		return nil, failWithCleanups(ctx, err, cleanupSteps)
	}
	cleanupSteps = append(cleanupSteps, homeCleanup)
	for _, path := range []string{repository, tree.Path} {
		if err := s.chown(ctx, project, path); err != nil {
			return nil, failWithCleanups(ctx, err, cleanupSteps)
		}
	}
	return combineCleanups(cleanupSteps), nil
}

func (s *System) ReconcileAuthorizedKeys(ctx context.Context, project domain.Project, access []domain.ProjectAccess) error {
	s.authorizedKeysMu.Lock()
	defer s.authorizedKeysMu.Unlock()
	sort.Slice(access, func(i, j int) bool { return access[i].Person.Username < access[j].Person.Username })
	lines := make([]string, 0)
	for _, entry := range access {
		keys := append([]domain.SSHDeviceKey(nil), entry.Keys...)
		sort.Slice(keys, func(i, j int) bool {
			if keys[i].Label == keys[j].Label {
				return keys[i].Fingerprint < keys[j].Fingerprint
			}
			return keys[i].Label < keys[j].Label
		})
		for _, key := range keys {
			if err := ValidatePublicKey(key.PublicKey, false); err != nil {
				return fmt.Errorf("device key %s: %w", key.ID, err)
			}
			fields := strings.Fields(key.PublicKey)
			home := s.sessionHome(project, entry.Person)
			command := fmt.Sprintf("/usr/libexec/soda/soda-ssh --actor %s --project %s --worktree %s --home %s", entry.Person.Username, project.Slug, entry.Worktree.Path, home)
			lines = append(lines, fmt.Sprintf("command=\"%s\" %s %s", command, fields[0], fields[1]))
		}
	}
	contents := ""
	if len(lines) != 0 {
		contents = strings.Join(lines, "\n") + "\n"
	}
	return s.writeAuthorizedKeys(ctx, s.authorizedKeysPath(project), contents)
}

func (s *System) createSessionHome(ctx context.Context, project domain.Project, person domain.Person, tree domain.Worktree) (Cleanup, error) {
	personRoot := filepath.Dir(s.sessionHome(project, person))
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
	if err := s.chown(ctx, project, personRoot); err != nil {
		_ = os.RemoveAll(personRoot)
		return nil, err
	}
	return func(context.Context) error { return os.RemoveAll(personRoot) }, nil
}

func (s *System) sessionHome(project domain.Project, person domain.Person) string {
	return filepath.Join(s.projectRoot(project), ".soda", "people", person.Username, "home")
}

func (s *System) WriteProjectEnvironment(ctx context.Context, project domain.Project, contents string) error {
	dir := filepath.Join(s.projectRoot(project), ".soda")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "env"), []byte(contents), 0o644); err != nil {
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

func ValidatePublicKey(key string, optional bool) error {
	if optional && key == "" {
		return nil
	}
	if strings.ContainsAny(key, "\r\n\x00") || !(strings.HasPrefix(key, "ssh-ed25519 ") || strings.HasPrefix(key, "ssh-rsa ")) {
		return errors.New("SSH public key is not a supported single-line key")
	}
	if _, err := domain.SSHKeyFingerprint(key); err != nil {
		return err
	}
	return nil
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
	if !s.ManageAccounts {
		return nil
	}
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
	if err = temporary.Chmod(0o600); err == nil {
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
	if s.ManageAccounts {
		if _, err = s.Runner.Run(ctx, "chown", []string{"root:root", path}, nil, ""); err == nil {
			_, err = s.Runner.Run(ctx, "restorecon", []string{path}, nil, "")
		}
	}
	return err
}

func noopCleanup(context.Context) error { return nil }

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
