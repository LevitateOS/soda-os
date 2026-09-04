package projects

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type workspaceGitRunner struct {
	calls         []Command
	key           string
	cloneFailures int
}

func (runner *workspaceGitRunner) Run(_ context.Context, command Command) (CommandResult, error) {
	runner.calls = append(runner.calls, command)
	arguments := strings.Join(command.Args, " ")
	if strings.Contains(arguments, "/usr/bin/rm --recursive --force") {
		return CommandResult{}, os.RemoveAll(command.Args[len(command.Args)-1])
	}
	if strings.Contains(arguments, "ssh-keygen -y") && len(runner.calls) == 1 {
		return CommandResult{ExitCode: 1, Stderr: "key does not exist"}, nil
	}
	if strings.Contains(arguments, "/usr/bin/git clone") {
		target := command.Args[len(command.Args)-1]
		if runner.cloneFailures > 0 {
			runner.cloneFailures--
			return CommandResult{ExitCode: 1, Stderr: "native clone failed"}, os.WriteFile(filepath.Join(target, "partial"), []byte("partial"), 0o600)
		}
		if err := os.MkdirAll(filepath.Join(target, ".git"), 0o700); err != nil {
			return CommandResult{}, err
		}
	}
	if strings.Contains(arguments, "ssh-keygen -y") {
		return CommandResult{Stdout: runner.key}, nil
	}
	return CommandResult{}, nil
}

func TestNativeWorkspaceCloneRetryRemovesOnlyPreviousOperationTemporary(t *testing.T) {
	root := t.TempDir()
	workspace := Account{Username: "soda-w-example", UID: os.Getuid(), GID: os.Getgid(), PrimaryGroup: "soda-w-example", Home: filepath.Join(root, "soda-w-example")}
	require.NoError(t, os.MkdirAll(workspace.Home, 0o700))
	runner := &workspaceGitRunner{cloneFailures: 1}
	platform := &NativePlatform{Runner: runner, HomeRoot: root}

	err := platform.CloneWorkspace(context.Background(), workspace, "site", "git@forgejo.example.test:alice/site.git")
	require.ErrorContains(t, err, "native clone failed")
	require.FileExists(t, filepath.Join(workspace.Home, "Projects", ".soda-site.tmp", "partial"))
	require.NoError(t, platform.CloneWorkspace(context.Background(), workspace, "site", "git@forgejo.example.test:alice/site.git"))
	require.NoFileExists(t, filepath.Join(workspace.Home, "Projects", ".soda-site.tmp", "partial"))
	require.DirExists(t, filepath.Join(workspace.Home, "Projects", "site", ".git"))
}

func TestNativeWorkspaceGitKeyAndCloneStayInWorkspace(t *testing.T) {
	root := t.TempDir()
	workspace := Account{
		Username: "soda-w-example", UID: os.Getuid(), GID: os.Getgid(), PrimaryGroup: "soda-w-example",
		Home: filepath.Join(root, "soda-w-example"),
	}
	require.NoError(t, os.MkdirAll(workspace.Home, 0o700))
	runner := &workspaceGitRunner{key: strings.TrimSpace(string(testAuthorizedKey(t)))}
	platform := &NativePlatform{Runner: runner, HomeRoot: root}

	publicKey, err := platform.GenerateWorkspaceGitKey(context.Background(), workspace)
	require.NoError(t, err)
	require.Equal(t, runner.key, publicKey)
	require.NoError(t, platform.CloneWorkspace(context.Background(), workspace, "site", "git@forgejo.example.test:alice/site.git"))

	require.Len(t, runner.calls, 6)
	generated := strings.Join(runner.calls[1].Args, " ")
	require.Contains(t, generated, "--user "+workspace.Username+" -- /usr/bin/ssh-keygen")
	require.Contains(t, generated, "-f "+workspaceGitKeyPath(workspace))
	cleanup := strings.Join(runner.calls[3].Args, " ")
	require.Contains(t, cleanup, "--user "+workspace.Username+" -- /usr/bin/rm --recursive --force -- ")
	clone := strings.Join(runner.calls[4].Args, " ")
	require.Contains(t, clone, "GIT_SSH_COMMAND=/usr/bin/ssh -i "+workspaceGitKeyPath(workspace)+" -o IdentitiesOnly=yes -o BatchMode=yes")
	require.Contains(t, clone, "/usr/bin/git clone -- git@forgejo.example.test:alice/site.git")
	require.NotContains(t, clone, "authorized_keys")
	require.DirExists(t, filepath.Join(workspace.Home, "Projects", "site", ".git"))
}
