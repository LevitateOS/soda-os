package projects

import (
	"context"
	"errors"
	"io"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
)

var ErrForgejoUserNotFound = errors.New("Forgejo account not found")

// LinuxHost is the consumer-owned slice of native Linux behavior required by
// the Projects workflow. linuxhost.Native is its production implementation.
type LinuxHost interface {
	UIDMin() (int, error)
	LookupAccount(context.Context, string) (linuxhost.Account, error)
	CandidateAccounts(context.Context, string, string) ([]linuxhost.Account, error)
	ReadAuthorizedKeys(linuxhost.Account) ([]byte, error)
	InstallAuthorizedKeys(linuxhost.Account, []byte) error
	PasswordStatus(context.Context, linuxhost.Account) (linuxhost.PasswordStatus, error)
	PreflightDeleteAccount(context.Context, linuxhost.Account) error
	DeleteAccount(context.Context, linuxhost.Account) error
}

// Platform contains only Projects-owned workspace and Forgejo transitions.
type Platform interface {
	WorkspaceOperationSharedLock() (io.Closer, error)
	WorkspaceOperationExclusiveLock() (io.Closer, error)
	SetupLock(linuxhost.Account, string) (io.Closer, error)
	WorkspaceReady(linuxhost.Account, string) (bool, error)
	CreateWorkspace(context.Context, linuxhost.Account, string) (linuxhost.Account, error)
	GenerateWorkspaceGitKey(context.Context, linuxhost.Account) (string, error)
	CloneWorkspace(context.Context, linuxhost.Account, string, string) error
	DeleteForgejoUser(context.Context, string) error
}

type NativePlatform struct {
	Host                  *linuxhost.Native
	OperationLockPath     string
	OperationLockOwnerUID int
	RuntimeRoot           string
}

func NewNativePlatform(host *linuxhost.Native) *NativePlatform {
	return &NativePlatform{
		Host:              host,
		OperationLockPath: DefaultWorkspaceOperationLockPath,
		RuntimeRoot:       "/run/user",
	}
}

func (platform *NativePlatform) host() *linuxhost.Native {
	if platform.Host == nil {
		platform.Host = linuxhost.NewNative()
	}
	return platform.Host
}

func (platform *NativePlatform) run(ctx context.Context, name string, args ...string) (linuxhost.CommandResult, error) {
	return platform.host().Run(ctx, linuxhost.Command{Name: name, Args: args})
}

func (platform *NativePlatform) runner() linuxhost.CommandRunner {
	return platform.host()
}

func (platform *NativePlatform) runtimeRoot() string {
	if platform.RuntimeRoot == "" {
		return "/run/user"
	}
	return platform.RuntimeRoot
}
