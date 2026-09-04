package runners

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// Registration and account commands are simulated; copying, client execution,
// descriptor publication and state cleanup use the real local filesystem.
type lifecycleCommands struct {
	t                 *testing.T
	native            *Native
	cleanupError      error
	registrationError error
	failDescriptor    bool
	registered        bool
	deletedAccounts   []string
}

func (commands *lifecycleCommands) Run(ctx context.Context, command Command) (CommandResult, error) {
	switch command.Name {
	case "restorecon":
		return CommandResult{}, nil
	case "systemctl":
		return CommandResult{Stdout: "LoadState=loaded\nActiveState=inactive\nSubState=dead\nUnitFileState=disabled\n"}, nil
	case "userdel":
		commands.deletedAccounts = append(commands.deletedAccounts, command.Args[0])
		return CommandResult{}, commands.cleanupError
	case "/usr/sbin/runuser":
		require.Equal(commands.t, []string{"--user", "soda-runner-one", "--"}, command.Args[:3])
		require.Equal(commands.t, []string{"--version"}, command.Args[4:])
		command.Name, command.Args = command.Args[3], command.Args[4:]
	}
	return (ExecCommandRunner{}).Run(ctx, command)
}

func (commands *lifecycleCommands) RunSecret(_ context.Context, command Command, _ string) error {
	commands.registered = true
	if commands.registrationError != nil {
		return commands.registrationError
	}
	if err := os.WriteFile(filepath.Join(command.Directory, ".runner"), []byte("{}\n"), 0o600); err != nil {
		return err
	}
	if commands.failDescriptor {
		return os.Mkdir(commands.native.descriptorPath("one"), 0o700)
	}
	return nil
}

func runnerFixture(t *testing.T) (*Native, *lifecycleCommands, preparedRunner) {
	t.Helper()
	root := t.TempDir()
	native := &Native{RootPath: filepath.Join(root, "runners"), LockPath: filepath.Join(root, "runners.lock"), GitHubSource: filepath.Join(root, "bundled")}
	commands := &lifecycleCommands{t: t, native: native}
	native.Runner = commands
	prepared := preparedRunner{account: "soda-runner-one", state: native.statePath("one"), owner: identity{uint32(os.Getuid()), uint32(os.Getgid())}}
	require.NoError(t, createOwnedDirectory(prepared.state, prepared.owner))
	writeClientVersion(t, native.githubSource(), "2.337.0")
	return native, commands, prepared
}

func writeClientVersion(t *testing.T, app, version string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(app, "bin"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(app, "bin", "Runner.Listener"), []byte("#!/bin/sh\n[ \"$1\" = --version ] || exit 2\nprintf '%s\\n' '"+version+"'\n"), 0o755))
}

func githubRequest() CreateRequest {
	return CreateRequest{ID: "one", Provider: ProviderGitHub, RegistrationURL: "https://github.com/example/repository", Labels: "soda", RegistrationToken: "test-token"}
}

func TestRunnerVersionFollowsInstalledClientAcrossImageAndRunnerReplacement(t *testing.T) {
	native, _, prepared := runnerFixture(t)
	ctx := context.Background()
	require.NoError(t, native.registerPrepared(ctx, prepared, githubRequest()))
	writeClientVersion(t, native.githubSource(), "2.338.0")
	views, err := native.List(ctx)
	require.NoError(t, err)
	require.Len(t, views, 1)
	require.Equal(t, "2.337.0", views[0].Version, "updating the bundled client must not change the installed runner's reported version")
	require.NoError(t, native.Remove(ctx, "one"))
	require.NoError(t, createOwnedDirectory(prepared.state, prepared.owner))
	require.NoError(t, native.registerPrepared(ctx, prepared, githubRequest()))
	views, err = native.List(ctx)
	require.NoError(t, err)
	require.Equal(t, "2.338.0", views[0].Version)
}

func TestGitHubDescriptorFailureReportsProviderAndLocalCleanup(t *testing.T) {
	for _, cleanupFails := range []bool{false, true} {
		name := "removed"
		if cleanupFails {
			name = "retained"
		}
		t.Run(name, func(t *testing.T) {
			native, commands, prepared := runnerFixture(t)
			commands.failDescriptor = true
			if cleanupFails {
				commands.cleanupError = errors.New("userdel failed")
			}
			err := native.registerPrepared(context.Background(), prepared, githubRequest())
			require.True(t, commands.registered)
			require.ErrorContains(t, err, "GitHub registration completed")
			require.ErrorContains(t, err, "inspect/remove the GitHub runner record before retrying")
			require.Equal(t, []string{prepared.account}, commands.deletedAccounts)
			if cleanupFails {
				require.ErrorIs(t, err, commands.cleanupError)
				require.ErrorContains(t, err, "account removal is unconfirmed and state remains")
				require.DirExists(t, prepared.state)
			} else {
				require.ErrorContains(t, err, "were removed")
				require.NoDirExists(t, prepared.state)
			}
		})
	}
}

func TestGitHubRegistrationErrorDoesNotClaimFailedCleanupSucceeded(t *testing.T) {
	native, commands, prepared := runnerFixture(t)
	commands.registrationError = errors.New("registration interrupted")
	commands.cleanupError = errors.New("userdel failed")
	err := native.registerPrepared(context.Background(), prepared, githubRequest())
	require.ErrorContains(t, err, "GitHub registration failed")
	require.ErrorContains(t, err, "account removal is unconfirmed")
	require.NotContains(t, err.Error(), "no local runner was retained")
	require.NotContains(t, err.Error(), "were removed")
	require.DirExists(t, prepared.state)
}
