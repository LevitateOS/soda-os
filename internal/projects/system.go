package projects

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
)

var (
	ErrAccountNotFound     = errors.New("Linux account not found")
	ErrForgejoUserNotFound = errors.New("Forgejo account not found")
)

type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type Command struct {
	Directory   string
	Name        string
	Args        []string
	Input       io.Reader
	ExtraFiles  []*os.File
	Environment []string
}

type CommandRunner interface {
	Run(context.Context, Command) (CommandResult, error)
}

type ExecCommandRunner struct{}

func (ExecCommandRunner) Run(ctx context.Context, request Command) (CommandResult, error) {
	command := exec.CommandContext(ctx, request.Name, request.Args...)
	command.Dir = request.Directory
	command.Stdin = request.Input
	command.ExtraFiles = request.ExtraFiles
	command.Env = request.Environment
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	result := CommandResult{Stdout: stdout.String(), Stderr: stderr.String()}
	if err == nil {
		return result, nil
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		result.ExitCode = exitError.ExitCode()
		return result, nil
	}
	return result, err
}

type Platform interface {
	UIDMin() (int, error)
	LookupAccount(context.Context, string) (Account, error)
	WorkspaceAccounts(context.Context) ([]Account, error)
	ReadAuthorizedKeys(Account) ([]byte, error)
	WorkspaceOperationSharedLock() (io.Closer, error)
	WorkspaceOperationExclusiveLock() (io.Closer, error)
	SetupLock(Account, string) (io.Closer, error)
	WorkspaceReady(Account, string) (bool, error)
	ValidatePasswordLocked(context.Context, Account) error
	CreateWorkspace(context.Context, Account, string) (Account, error)
	CreatePrimary(context.Context, string, string) (Account, error)
	PublishHuman(context.Context, string, []byte) error
	InstallAuthorizedKeys(Account, []byte) error
	GenerateWorkspaceGitKey(context.Context, Account) (string, error)
	CloneWorkspace(context.Context, Account, string, string) error
	InstallMiseTools(context.Context, Account, string, []string, []string) error
	PreflightDeleteAccount(context.Context, Account) error
	DeleteForgejoUser(context.Context, string) error
	DeleteAccount(context.Context, Account) error
	RemoveMiseProject(string) error
}

type NativePlatform struct {
	Runner                CommandRunner
	LoginDefsPath         string
	HomeRoot              string
	RuntimeRoot           string
	OperationLockPath     string
	OperationLockOwnerUID int
	MiseRoot              string
}

func NewNativePlatform() *NativePlatform {
	return &NativePlatform{
		Runner:            ExecCommandRunner{},
		LoginDefsPath:     "/etc/login.defs",
		HomeRoot:          "/home",
		RuntimeRoot:       "/run/user",
		OperationLockPath: DefaultWorkspaceOperationLockPath,
		MiseRoot:          "/var/lib/soda/mise",
	}
}

func (platform *NativePlatform) run(ctx context.Context, name string, args ...string) (CommandResult, error) {
	return platform.runner().Run(ctx, Command{Name: name, Args: args})
}

func (platform *NativePlatform) runner() CommandRunner {
	if platform.Runner == nil {
		return ExecCommandRunner{}
	}
	return platform.Runner
}
