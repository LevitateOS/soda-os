package projects

import (
	"context"
	"errors"
	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type nativeIdentityRunner struct {
	results map[string]linuxhost.CommandResult
	calls   []linuxhost.Command
}

func (runner *nativeIdentityRunner) Run(_ context.Context, command linuxhost.Command) (linuxhost.CommandResult, error) {
	runner.calls = append(runner.calls, command)
	result, found := runner.results[identityCommandKey(command.Name, command.Args...)]
	if !found {
		return linuxhost.CommandResult{}, errors.New("unexpected native identity command")
	}
	return result, nil
}

func identityCommandKey(name string, args ...string) string {
	return name + "\x00" + strings.Join(args, "\x00")
}

func TestNativeWorkspaceCreationUsesRepresentableLinuxState(t *testing.T) {
	t.Parallel()
	username, err := DerivedUsername("alice", "site")
	require.NoError(t, err)
	marker, err := WorkspaceMarker("alice", "site")
	require.NoError(t, err)
	useraddArgs := []string{
		"--create-home",
		"--user-group",
		"--groups", "soda-workspaces",
		"--shell", "/bin/bash",
		"--home-dir", "/home/" + username,
		"--comment", "soda-workspace=alice/site",
		"--", username,
	}
	runner := &nativeIdentityRunner{results: map[string]linuxhost.CommandResult{
		identityCommandKey("/usr/sbin/useradd", useraddArgs...):           {},
		identityCommandKey("/usr/bin/getent", "passwd", username):         {Stdout: username + ":x:2000:2000:" + marker + ":/home/" + username + ":/bin/bash\n"},
		identityCommandKey("/usr/bin/id", "--name", "--groups", username): {Stdout: username + " soda-workspaces\n"},
		identityCommandKey("/usr/bin/id", "--name", "--group", username):  {Stdout: username + "\n"},
		identityCommandKey("/usr/bin/passwd", "--status", username):       {Stdout: username + " L 2026-09-01 0 99999 7 -1\n"},
	}}
	loginDefs := filepath.Join(t.TempDir(), "login.defs")
	require.NoError(t, os.WriteFile(loginDefs, []byte("UID_MIN 1000\n"), 0o600))
	platform := &NativePlatform{Host: &linuxhost.Native{Runner: runner, LoginDefsPath: loginDefs}}

	account, err := platform.CreateWorkspace(context.Background(), primaryAccount("alice", primaryRoleUser), "site")
	require.NoError(t, err)
	require.Equal(t, username, account.Username)
	require.Equal(t, marker, account.GECOS)
	require.Equal(t, "/usr/sbin/useradd", runner.calls[0].Name)
	require.Equal(t, useraddArgs, runner.calls[0].Args)
	require.NotContains(t, runner.calls[0].Args, "--password", "useradd's native no-password creation must leave the password locked")
}

func TestNativeForgejoDeletionUsesTheNonPurgeForgejoBoundary(t *testing.T) {
	runner := &nativeIdentityRunner{results: map[string]linuxhost.CommandResult{
		identityCommandKey("/usr/sbin/runuser", "--user", "git", "--", "/usr/bin/forgejo", "admin", "user", "delete", "--config", "/etc/forgejo/app.ini", "--username", "alice"): {},
	}}
	platform := &NativePlatform{Host: &linuxhost.Native{Runner: runner}}

	require.NoError(t, platform.DeleteForgejoUser(context.Background(), "alice"))
	require.Len(t, runner.calls, 1)
	require.NotContains(t, runner.calls[0].Args, "--purge")
}

func TestNativeForgejoDeletionRecognizesCompletedNativeDeletion(t *testing.T) {
	runner := &nativeIdentityRunner{results: map[string]linuxhost.CommandResult{
		identityCommandKey("/usr/sbin/runuser", "--user", "git", "--", "/usr/bin/forgejo", "admin", "user", "delete", "--config", "/etc/forgejo/app.ini", "--username", "alice"): {ExitCode: 1, Stderr: "user does not exist [name: alice]"},
	}}
	err := (&NativePlatform{Host: &linuxhost.Native{Runner: runner}}).DeleteForgejoUser(context.Background(), "alice")
	require.ErrorIs(t, err, ErrForgejoUserNotFound)
}
