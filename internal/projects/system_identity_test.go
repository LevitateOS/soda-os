package projects

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type nativeIdentityRunner struct {
	results map[string]CommandResult
	calls   []Command
}

func (runner *nativeIdentityRunner) Run(_ context.Context, command Command) (CommandResult, error) {
	runner.calls = append(runner.calls, command)
	result, found := runner.results[identityCommandKey(command.Name, command.Args...)]
	if !found {
		return CommandResult{}, errors.New("unexpected native identity command")
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
	runner := &nativeIdentityRunner{results: map[string]CommandResult{
		identityCommandKey("/usr/sbin/useradd", useraddArgs...):           {},
		identityCommandKey("/usr/bin/getent", "passwd", username):         {Stdout: username + ":x:2000:2000:" + marker + ":/home/" + username + ":/bin/bash\n"},
		identityCommandKey("/usr/bin/id", "--name", "--groups", username): {Stdout: username + " soda-workspaces\n"},
		identityCommandKey("/usr/bin/id", "--name", "--group", username):  {Stdout: username + "\n"},
		identityCommandKey("/usr/bin/passwd", "--status", username):       {Stdout: username + " L 2026-09-01 0 99999 7 -1\n"},
	}}
	loginDefs := filepath.Join(t.TempDir(), "login.defs")
	require.NoError(t, os.WriteFile(loginDefs, []byte("UID_MIN 1000\n"), 0o600))
	platform := &NativePlatform{Runner: runner, LoginDefsPath: loginDefs}

	account, err := platform.CreateWorkspace(context.Background(), primaryAccount("alice", primaryRoleUser), "site")
	require.NoError(t, err)
	require.Equal(t, username, account.Username)
	require.Equal(t, marker, account.GECOS)
	require.Equal(t, "/usr/sbin/useradd", runner.calls[0].Name)
	require.Equal(t, useraddArgs, runner.calls[0].Args)
	require.NotContains(t, runner.calls[0].Args, "--password", "useradd's native no-password creation must leave the password locked")
}

func TestWorkspaceAccountsEnumeratesEveryLinuxEvidenceSource(t *testing.T) {
	t.Parallel()
	const passwd = "" +
		"root:x:0:0:root:/root:/bin/bash\n" +
		"primarygid:x:2000:997:Primary GID:/home/primarygid:/bin/bash\n" +
		"supplemental:x:2001:2001:Supplemental:/home/supplemental:/bin/bash\n" +
		"markeronly:x:2002:2002:soda-workspace=alice/tools:/home/markeronly:/bin/bash\n" +
		"malformed:x:2003:2003:soda-workspace=missing-separator:/home/malformed:/bin/bash\n"
	groups := map[string]string{
		"primarygid":   "soda-workspaces",
		"supplemental": "supplemental soda-workspaces",
		"markeronly":   "markeronly",
		"malformed":    "malformed",
	}
	primaryGroups := map[string]string{
		"primarygid":   "soda-workspaces",
		"supplemental": "supplemental",
		"markeronly":   "markeronly",
		"malformed":    "malformed",
	}
	results := map[string]CommandResult{
		identityCommandKey("/usr/bin/getent", "group", WorkspaceGroup): {Stdout: "soda-workspaces:x:997:supplemental\n"},
		identityCommandKey("/usr/bin/getent", "passwd"):                {Stdout: passwd},
	}
	for username, output := range groups {
		results[identityCommandKey("/usr/bin/id", "--name", "--groups", username)] = CommandResult{Stdout: output + "\n"}
		results[identityCommandKey("/usr/bin/id", "--name", "--group", username)] = CommandResult{Stdout: primaryGroups[username] + "\n"}
	}
	runner := &nativeIdentityRunner{results: results}

	accounts, err := (&NativePlatform{Runner: runner}).WorkspaceAccounts(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"malformed", "markeronly", "primarygid", "supplemental"}, accountUsernames(accounts))
	require.Equal(t, "soda-workspaces", accounts[2].PrimaryGroup, "primary-GID members must not be omitted")
	require.False(t, accounts[1].Groups[WorkspaceGroup], "marker-only candidates must be returned even without group membership")
	_, _, err = ParseWorkspaceMarker(accounts[0].GECOS)
	require.Error(t, err, "malformed marker candidates must reach fail-closed association validation")
}

func TestWorkspaceAccountsRejectsGroupMemberMissingFromPasswdEnumeration(t *testing.T) {
	t.Parallel()
	runner := &nativeIdentityRunner{results: map[string]CommandResult{
		identityCommandKey("/usr/bin/getent", "group", WorkspaceGroup): {Stdout: "soda-workspaces:x:997:missing\n"},
		identityCommandKey("/usr/bin/getent", "passwd"):                {Stdout: "root:x:0:0:root:/root:/bin/bash\n"},
	}}

	_, err := (&NativePlatform{Runner: runner}).WorkspaceAccounts(context.Background())
	require.ErrorContains(t, err, "has no Linux account record")
}

func accountUsernames(accounts []Account) []string {
	usernames := make([]string, len(accounts))
	for index, account := range accounts {
		usernames[index] = account.Username
	}
	return usernames
}
