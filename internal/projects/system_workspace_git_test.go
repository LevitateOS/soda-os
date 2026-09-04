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
	calls []Command
	key   string
}

func (runner *workspaceGitRunner) Run(_ context.Context, command Command) (CommandResult, error) {
	runner.calls = append(runner.calls, command)
	arguments := strings.Join(command.Args, " ")
	if strings.Contains(arguments, "ssh-keygen -y") && len(runner.calls) == 1 {
		return CommandResult{ExitCode: 1, Stderr: "key does not exist"}, nil
	}
	if strings.Contains(arguments, "/usr/bin/git clone") {
		target := command.Args[len(command.Args)-1]
		if err := os.MkdirAll(filepath.Join(target, ".git"), 0o700); err != nil {
			return CommandResult{}, err
		}
	}
	if strings.Contains(arguments, "ssh-keygen -y") {
		return CommandResult{Stdout: runner.key}, nil
	}
	return CommandResult{}, nil
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

	require.Len(t, runner.calls, 5)
	generated := strings.Join(runner.calls[1].Args, " ")
	require.Contains(t, generated, "--user "+workspace.Username+" -- /usr/bin/ssh-keygen")
	require.Contains(t, generated, "-f "+workspaceGitKeyPath(workspace))
	clone := strings.Join(runner.calls[3].Args, " ")
	require.Contains(t, clone, "GIT_SSH_COMMAND=/usr/bin/ssh -i "+workspaceGitKeyPath(workspace)+" -o IdentitiesOnly=yes -o BatchMode=yes")
	require.Contains(t, clone, "/usr/bin/git clone -- git@forgejo.example.test:alice/site.git")
	require.NotContains(t, clone, "authorized_keys")
	require.DirExists(t, filepath.Join(workspace.Home, "Projects", "site", ".git"))
}
