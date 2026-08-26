package host

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/LevitateOS/soda-os/internal/domain"
)

var ErrNotFound = errors.New("host resource not found")

const DefaultAuthorizedKeysRoot = "/etc/soda/authorized_keys"

type Cleanup func(context.Context) error

type Operations interface {
	InstallerAdministrator(context.Context) (*domain.Person, error)
	CreatePerson(context.Context, domain.Person, string) (Cleanup, error)
	ImportPerson(context.Context, domain.Person) (Cleanup, error)
	CreateProject(context.Context, domain.Project) (Cleanup, error)
	EnsureRepository(context.Context, domain.Project) error
	CreateWorktree(context.Context, domain.Project, domain.Person, domain.Worktree, string) (Cleanup, error)
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
	PasswdPath         string
	GroupPath          string
	AuthorizedKeysRoot string
	authorizedKeysMu   sync.Mutex
}

func New(projectsRoot string, manageAccounts bool) *System {
	authorizedKeysRoot := DefaultAuthorizedKeysRoot
	if !manageAccounts {
		authorizedKeysRoot = filepath.Join(projectsRoot, ".authorized_keys")
	}
	return &System{ProjectsRoot: projectsRoot, ManageAccounts: manageAccounts, Runner: ExecRunner{}, PasswdPath: "/etc/passwd", GroupPath: "/etc/group", AuthorizedKeysRoot: authorizedKeysRoot}
}

func (s *System) InstallerAdministrator(ctx context.Context) (*domain.Person, error) {
	if !s.ManageAccounts {
		return nil, nil
	}
	passwd, err := os.ReadFile(s.PasswdPath)
	if err != nil {
		return nil, err
	}
	group, err := os.ReadFile(s.GroupPath)
	if err != nil {
		return nil, err
	}
	account := installerAdministrator(string(passwd), string(group))
	if account == nil {
		return nil, nil
	}
	key := ""
	if contents, readErr := os.ReadFile(filepath.Join(account.home, ".ssh", "authorized_keys")); readErr == nil {
		scanner := bufio.NewScanner(bytes.NewReader(contents))
		for scanner.Scan() {
			candidate := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(candidate, "ssh-ed25519 ") || strings.HasPrefix(candidate, "ssh-rsa ") {
				key = candidate
				break
			}
		}
	}
	return &domain.Person{Username: account.username, DisplayName: account.displayName, Email: account.username + "@soda.local", Role: domain.RoleAdmin, SSHPublicKey: key}, nil
}

func (s *System) CreatePerson(ctx context.Context, person domain.Person, password string) (Cleanup, error) {
	if err := ValidatePublicKey(person.SSHPublicKey, false); err != nil {
		return nil, err
	}
	if !s.ManageAccounts {
		return noopCleanup, nil
	}
	if err := s.ensureGroups(ctx); err != nil {
		return nil, err
	}
	group := roleGroup(person.Role)
	if _, err := s.Runner.Run(ctx, "useradd", []string{"--create-home", "--groups", group, "--shell", "/sbin/nologin", person.Username}, nil, ""); err != nil {
		return nil, err
	}
	cleanup := func(cleanupContext context.Context) error {
		_, err := s.Runner.Run(cleanupContext, "userdel", []string{"--remove", person.Username}, nil, "")
		return err
	}
	if strings.ContainsAny(password, "\r\n\x00") {
		return nil, failWithCleanup(ctx, errors.New("password contains a line or NUL delimiter"), cleanup)
	}
	if _, err := s.Runner.Run(ctx, "chpasswd", nil, nil, person.Username+":"+password+"\n"); err != nil {
		return nil, failWithCleanup(ctx, err, cleanup)
	}
	return cleanup, nil
}

func (s *System) ImportPerson(ctx context.Context, person domain.Person) (Cleanup, error) {
	if err := ValidatePublicKey(person.SSHPublicKey, true); err != nil {
		return nil, err
	}
	if !s.ManageAccounts {
		return noopCleanup, nil
	}
	if _, err := s.Runner.Run(ctx, "getent", []string{"passwd", person.Username}, nil, ""); err != nil {
		return nil, fmt.Errorf("%w: Linux account %s", ErrNotFound, person.Username)
	}
	if err := s.ensureGroups(ctx); err != nil {
		return nil, err
	}
	group := roleGroup(person.Role)
	groups, err := s.Runner.Run(ctx, "id", []string{"--groups", "--name", person.Username}, nil, "")
	if err != nil {
		return nil, err
	}
	if containsField(groups, group) {
		return noopCleanup, nil
	}
	if _, err = s.Runner.Run(ctx, "usermod", []string{"--append", "--groups", group, person.Username}, nil, ""); err != nil {
		return nil, err
	}
	return func(cleanupContext context.Context) error {
		_, cleanupErr := s.Runner.Run(cleanupContext, "gpasswd", []string{"--delete", person.Username, group}, nil, "")
		return cleanupErr
	}, nil
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

func (s *System) CreateWorktree(ctx context.Context, project domain.Project, person domain.Person, tree domain.Worktree, baseRef string) (Cleanup, error) {
	if err := ValidatePublicKey(person.SSHPublicKey, false); err != nil {
		return nil, err
	}
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
	keyFile := s.authorizedKeysPath(project)
	line := fmt.Sprintf("command=\"/usr/libexec/soda/soda-ssh --actor %s --worktree %s\" %s", person.Username, tree.Path, person.SSHPublicKey)
	if err := s.appendAuthorizedLine(ctx, keyFile, line); err != nil {
		return nil, failWithCleanups(ctx, err, cleanupSteps)
	}
	cleanupSteps = append(cleanupSteps, func(cleanupContext context.Context) error {
		return s.removeAuthorizedLine(cleanupContext, keyFile, line)
	})
	for _, path := range []string{repository, tree.Path} {
		if err := s.chown(ctx, project, path); err != nil {
			return nil, failWithCleanups(ctx, err, cleanupSteps)
		}
	}
	return combineCleanups(cleanupSteps), nil
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
func (s *System) ensureGroups(ctx context.Context) error {
	for _, group := range []string{"soda-admins", "soda-developers"} {
		if _, err := s.Runner.Run(ctx, "groupadd", []string{"--force", "--system", group}, nil, ""); err != nil {
			return err
		}
	}
	return nil
}
func roleGroup(role domain.Role) string {
	if role == domain.RoleAdmin {
		return "soda-admins"
	}
	return "soda-developers"
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

func (s *System) removeAuthorizedLine(ctx context.Context, path, line string) error {
	s.authorizedKeysMu.Lock()
	defer s.authorizedKeysMu.Unlock()
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.TrimSuffix(string(contents), "\n"), "\n")
	kept := lines[:0]
	for _, candidate := range lines {
		if candidate != line && candidate != "" {
			kept = append(kept, candidate)
		}
	}
	updated := ""
	if len(kept) != 0 {
		updated = strings.Join(kept, "\n") + "\n"
	}
	if err = s.writeAuthorizedKeys(ctx, path, updated); err != nil {
		if restoreErr := s.writeAuthorizedKeys(ctx, path, string(contents)); restoreErr != nil {
			return errors.Join(err, fmt.Errorf("restore authorized keys: %w", restoreErr))
		}
		return err
	}
	return nil
}

func (s *System) appendAuthorizedLine(ctx context.Context, path, line string) error {
	s.authorizedKeysMu.Lock()
	defer s.authorizedKeysMu.Unlock()
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	updated := string(contents)
	if updated != "" && !strings.HasSuffix(updated, "\n") {
		updated += "\n"
	}
	updated += line + "\n"
	if err = s.writeAuthorizedKeys(ctx, path, updated); err != nil {
		if restoreErr := s.writeAuthorizedKeys(ctx, path, string(contents)); restoreErr != nil {
			return errors.Join(err, fmt.Errorf("restore authorized keys: %w", restoreErr))
		}
		return err
	}
	return nil
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
		_, err = s.Runner.Run(ctx, "chown", []string{"root:root", path}, nil, "")
	}
	return err
}

func containsField(output, value string) bool {
	for _, field := range strings.Fields(output) {
		if field == value {
			return true
		}
	}
	return false
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

type localAccount struct{ username, displayName, home string }

func installerAdministrator(passwd, group string) *localAccount {
	wheel := ""
	for _, line := range strings.Split(group, "\n") {
		fields := strings.Split(line, ":")
		if len(fields) == 4 && fields[0] == "wheel" {
			wheel = fields[3]
			break
		}
	}
	for _, username := range strings.Split(wheel, ",") {
		if username == "" {
			continue
		}
		for _, line := range strings.Split(passwd, "\n") {
			fields := strings.Split(line, ":")
			if len(fields) < 7 || fields[0] != username {
				continue
			}
			display := strings.TrimSpace(strings.Split(fields[4], ",")[0])
			if display == "" {
				display = username
			}
			return &localAccount{username, display, fields[5]}
		}
	}
	return nil
}
