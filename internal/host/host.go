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

	"github.com/LevitateOS/soda-os/internal/domain"
)

var ErrNotFound = errors.New("host resource not found")

type Operations interface {
	InstallerAdministrator(context.Context) (*domain.Person, error)
	CreatePerson(context.Context, domain.Person, string) error
	ImportPerson(context.Context, domain.Person) error
	CreateProject(context.Context, domain.Project) error
	EnsureRepository(context.Context, domain.Project) error
	CreateWorktree(context.Context, domain.Project, domain.Person, domain.Worktree, string) error
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
	ProjectsRoot   string
	ManageAccounts bool
	Runner         Runner
	PasswdPath     string
	GroupPath      string
}

func New(projectsRoot string, manageAccounts bool) *System {
	return &System{ProjectsRoot: projectsRoot, ManageAccounts: manageAccounts, Runner: ExecRunner{}, PasswdPath: "/etc/passwd", GroupPath: "/etc/group"}
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

func (s *System) CreatePerson(ctx context.Context, person domain.Person, password string) error {
	if err := ValidatePublicKey(person.SSHPublicKey, false); err != nil {
		return err
	}
	if !s.ManageAccounts {
		return nil
	}
	if err := s.ensureGroups(ctx); err != nil {
		return err
	}
	group := roleGroup(person.Role)
	if _, err := s.Runner.Run(ctx, "useradd", []string{"--create-home", "--groups", group, "--shell", "/sbin/nologin", person.Username}, nil, ""); err != nil {
		return err
	}
	_, err := s.Runner.Run(ctx, "chpasswd", nil, nil, person.Username+":"+password+"\n")
	return err
}

func (s *System) ImportPerson(ctx context.Context, person domain.Person) error {
	if err := ValidatePublicKey(person.SSHPublicKey, true); err != nil {
		return err
	}
	if !s.ManageAccounts {
		return nil
	}
	if _, err := s.Runner.Run(ctx, "getent", []string{"passwd", person.Username}, nil, ""); err != nil {
		return fmt.Errorf("%w: Linux account %s", ErrNotFound, person.Username)
	}
	if err := s.ensureGroups(ctx); err != nil {
		return err
	}
	_, err := s.Runner.Run(ctx, "usermod", []string{"--append", "--groups", roleGroup(person.Role), person.Username}, nil, "")
	return err
}

func (s *System) CreateProject(ctx context.Context, project domain.Project) error {
	root := s.projectRoot(project)
	if s.ManageAccounts {
		if _, err := s.Runner.Run(ctx, "useradd", []string{"--system", "--create-home", "--home-dir", root, "--shell", "/bin/bash", project.UnixUser}, nil, ""); err != nil {
			return err
		}
	} else if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
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
	keyFile := filepath.Join(sshDir, "authorized_keys")
	file, err := os.OpenFile(keyFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	if err = os.Chmod(keyFile, 0o600); err != nil {
		return err
	}
	if _, ok := project.Source.(domain.EmptyProjectSource); ok {
		if err = s.initializeEmptyRepository(ctx, s.repository(project)); err != nil {
			return err
		}
	}
	if s.ManageAccounts {
		if _, err = s.Runner.Run(ctx, "restorecon", []string{"-R", root}, nil, ""); err != nil {
			return err
		}
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
		return err
	}
	return s.chown(ctx, project, repository)
}

func (s *System) CreateWorktree(ctx context.Context, project domain.Project, person domain.Person, tree domain.Worktree, baseRef string) error {
	if err := ValidatePublicKey(person.SSHPublicKey, false); err != nil {
		return err
	}
	repository := s.repository(project)
	if err := os.MkdirAll(filepath.Dir(tree.Path), 0o755); err != nil {
		return err
	}
	if err := s.chown(ctx, project, filepath.Dir(tree.Path)); err != nil {
		return err
	}
	commands := [][]string{{"--git-dir", repository, "worktree", "add", "-b", tree.Branch, tree.Path, baseRef}, {"--git-dir", repository, "config", "extensions.worktreeConfig", "true"}, {"-C", tree.Path, "config", "--worktree", "core.bare", "false"}, {"-C", tree.Path, "config", "--worktree", "user.name", person.DisplayName}, {"-C", tree.Path, "config", "--worktree", "user.email", person.Email}}
	for _, args := range commands {
		if _, err := s.Runner.Run(ctx, "git", args, nil, ""); err != nil {
			return err
		}
	}
	keyFile := filepath.Join(s.projectRoot(project), ".ssh", "authorized_keys")
	file, err := os.OpenFile(keyFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := fmt.Fprintf(file, "command=\"/usr/libexec/soda/soda-ssh --actor %s --worktree %s\" %s\n", person.Username, tree.Path, person.SSHPublicKey)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	for _, path := range []string{repository, tree.Path, keyFile} {
		if err := s.chown(ctx, project, path); err != nil {
			return err
		}
	}
	return nil
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
	if strings.ContainsAny(key, "\r\n") || !(strings.HasPrefix(key, "ssh-ed25519 ") || strings.HasPrefix(key, "ssh-rsa ")) {
		return errors.New("SSH public key is not a supported single-line key")
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
