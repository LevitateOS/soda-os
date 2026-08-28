package host

import (
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
