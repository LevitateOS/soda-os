package linuxhost

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type identityRunner struct {
	results map[string]CommandResult
	calls   []Command
}

func (runner *identityRunner) Run(_ context.Context, command Command) (CommandResult, error) {
	runner.calls = append(runner.calls, command)
	result, found := runner.results[commandKey(command.Name, command.Args...)]
	if !found {
		return CommandResult{}, errors.New("unexpected native identity command")
	}
	return result, nil
}

func commandKey(name string, args ...string) string {
	return name + "\x00" + strings.Join(args, "\x00")
}

func TestNativeReadsUIDMinFromLinuxPolicy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "login.defs")
	require.NoError(t, os.WriteFile(path, []byte("# native policy\nUID_MIN 1500\n"), 0o600))
	native := NewNative()
	native.LoginDefsPath = path

	uidMin, err := native.UIDMin()
	require.NoError(t, err)
	require.Equal(t, 1500, uidMin)
}

func TestNativeLookupAccountRequiresExactNameAndLoadsLinuxGroups(t *testing.T) {
	runner := &identityRunner{results: map[string]CommandResult{
		commandKey("/usr/bin/getent", "passwd", "alice"):         {Stdout: "alice:x:1000:1000:Alice:/home/alice:/bin/bash\n"},
		commandKey("/usr/bin/id", "--name", "--groups", "alice"): {Stdout: "alice wheel\n"},
		commandKey("/usr/bin/id", "--name", "--group", "alice"):  {Stdout: "alice\n"},
	}}
	native := NewNative()
	native.Runner = runner

	account, err := native.LookupAccount(context.Background(), "alice")
	require.NoError(t, err)
	require.Equal(t, Account{
		Username: "alice", UID: 1000, GID: 1000, PrimaryGroup: "alice", GECOS: "Alice",
		Home: "/home/alice", Shell: "/bin/bash", Groups: map[string]bool{"alice": true, "wheel": true},
	}, account)

	runner.results[commandKey("/usr/bin/getent", "passwd", "1000")] = CommandResult{
		Stdout: "alice:x:1000:1000:Alice:/home/alice:/bin/bash\n",
	}
	_, err = native.LookupAccount(context.Background(), "1000")
	require.ErrorContains(t, err, "different username")
}

func TestPKExecCallerRequiresThePrivilegedBoundary(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("this negative boundary test requires a non-root test process")
	}
	t.Setenv("PKEXEC_UID", "1000")
	_, err := PKExecCaller()
	require.ErrorContains(t, err, "effective UID 0")
}

func TestNativeCandidateAccountsReturnsEverySuppliedEvidenceSource(t *testing.T) {
	const passwd = "" +
		"root:x:0:0:root:/root:/bin/bash\n" +
		"primarygid:x:2000:997:Primary GID:/home/primarygid:/bin/bash\n" +
		"supplemental:x:2001:2001:Supplemental:/home/supplemental:/bin/bash\n" +
		"markeronly:x:2002:2002:soda-workspace=alice/tools:/home/markeronly:/bin/bash\n"
	runner := &identityRunner{results: map[string]CommandResult{
		commandKey("/usr/bin/getent", "group", "soda-workspaces"):       {Stdout: "soda-workspaces:x:997:supplemental\n"},
		commandKey("/usr/bin/getent", "passwd"):                         {Stdout: passwd},
		commandKey("/usr/bin/id", "--name", "--groups", "primarygid"):   {Stdout: "soda-workspaces\n"},
		commandKey("/usr/bin/id", "--name", "--group", "primarygid"):    {Stdout: "soda-workspaces\n"},
		commandKey("/usr/bin/id", "--name", "--groups", "supplemental"): {Stdout: "supplemental soda-workspaces\n"},
		commandKey("/usr/bin/id", "--name", "--group", "supplemental"):  {Stdout: "supplemental\n"},
		commandKey("/usr/bin/id", "--name", "--groups", "markeronly"):   {Stdout: "markeronly\n"},
		commandKey("/usr/bin/id", "--name", "--group", "markeronly"):    {Stdout: "markeronly\n"},
	}}
	native := NewNative()
	native.Runner = runner

	accounts, err := native.CandidateAccounts(context.Background(), "soda-workspaces", "soda-workspace=")
	require.NoError(t, err)
	require.Equal(t, []string{"markeronly", "primarygid", "supplemental"}, accountNames(accounts))
	require.Equal(t, "soda-workspaces", accounts[1].PrimaryGroup)
	require.False(t, accounts[0].HasGroup("soda-workspaces"), "native evidence must be returned for the caller to classify")
}

func TestNativeCandidateAccountsRejectsGroupMemberMissingFromPasswd(t *testing.T) {
	runner := &identityRunner{results: map[string]CommandResult{
		commandKey("/usr/bin/getent", "group", "soda-workspaces"): {Stdout: "soda-workspaces:x:997:missing\n"},
		commandKey("/usr/bin/getent", "passwd"):                   {Stdout: "root:x:0:0:root:/root:/bin/bash\n"},
	}}
	native := NewNative()
	native.Runner = runner

	_, err := native.CandidateAccounts(context.Background(), "soda-workspaces", "soda-workspace=")
	require.ErrorContains(t, err, "has no account record")
}

func accountNames(accounts []Account) []string {
	names := make([]string, len(accounts))
	for index, account := range accounts {
		names[index] = account.Username
	}
	return names
}
