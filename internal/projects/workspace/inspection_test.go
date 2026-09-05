package workspace

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/stretchr/testify/require"
)

type inspectionHomes struct{ path string }

func (homes inspectionHomes) OpenAccountHome(linuxhost.Account) (*os.File, error) {
	return os.Open(homes.path)
}

func TestInspectionOfMissingWorkspaceOnlyReadsPrimaryKeys(t *testing.T) {
	host := newFakeAccountHost()
	host.keys = nil
	accounts := NewAccounts(host, host, host, nil)
	result, err := accounts.Inspect(context.Background(), Repository{}, primaryAccount("alice"), projectEntry("site"))
	require.NoError(t, err)
	require.False(t, result.Exists)
	require.False(t, result.CheckoutReady)
	require.NotEmpty(t, result.Username)
	require.Contains(t, result.PrimaryKeyProblem, "public key")
	require.Empty(t, result.PublicKey)
	require.Empty(t, result.CheckoutPath)
	require.Empty(t, host.accounts)
	require.Empty(t, host.installedKeys)
	require.Equal(t, 1, host.calls.keyReads)
}

func TestInspectionKeepsAccountAndIndependentKeyProblemsWithoutRepairingThem(t *testing.T) {
	host := newFakeAccountHost()
	account := workspaceAccount(t, "alice", "site", os.Getuid())
	host.uidMin = os.Getuid()
	host.accounts[account.Username] = account
	host.keys = nil
	accounts := NewAccounts(host, host, host, nil)
	key := strings.TrimSpace(string(testAuthorizedKey(t)))
	runner := commandRunnerFunc(func(_ context.Context, command linuxhost.Command) (linuxhost.CommandResult, error) {
		require.Equal(t, "/usr/sbin/runuser", command.Name)
		require.Equal(t, []string{"--user", account.Username, "--", "/usr/bin/ssh-keygen", "-y", "-P", "", "-f", outboundKeyPath(account)}, command.Args)
		return linuxhost.CommandResult{Stdout: key}, nil
	})
	repository := NewRepository(inspectionHomes{path: t.TempDir()}, runner)
	result, err := accounts.Inspect(context.Background(), repository, primaryAccount("alice"), projectEntry("site"))
	require.NoError(t, err)
	require.True(t, result.Exists)
	require.False(t, result.CheckoutReady)
	require.Empty(t, result.CheckoutProblem, "no checkout is different from failed inspection")
	require.NotEmpty(t, result.PrimaryKeyProblem)
	require.NotEmpty(t, result.WorkspaceKeyProblem)
	require.Equal(t, key, result.PublicKey)
	require.Empty(t, result.GitKeyProblem)
	require.Equal(t, account.Home+"/Projects/site", result.CheckoutPath)
	require.Empty(t, host.installedKeys, "inspection must not copy keys")
	require.Len(t, host.accounts, 1)
}
