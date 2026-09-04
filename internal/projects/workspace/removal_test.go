package workspace

import (
	"context"
	"errors"
	"testing"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/stretchr/testify/require"
)

func TestRemoverDeletesOnlyTheDerivedWorkspaceAndIsIdempotent(t *testing.T) {
	host := newFakeAccountHost()
	primary := primaryAccount("alice")
	entry := projectEntry("site")
	alice := workspaceAccount(t, primary.Username, entry.ID, 2001)
	bob := workspaceAccount(t, "bob", entry.ID, 2002)
	host.accounts[alice.Username] = alice
	host.accounts[bob.Username] = bob
	remover := NewRemover(host, host)

	require.NoError(t, remover.Remove(context.Background(), primary, entry.ID))
	require.Equal(t, []string{alice.Username}, host.calls.deleted)
	require.NotContains(t, host.accounts, alice.Username)
	require.Contains(t, host.accounts, bob.Username)
	require.NoError(t, remover.Remove(context.Background(), primary, entry.ID))
	require.Equal(t, []string{alice.Username}, host.calls.deleted)
}

func TestRemoverReportsEveryRetainedOwnerOnFailure(t *testing.T) {
	host := newFakeAccountHost()
	primary := primaryAccount("alice")
	entry := projectEntry("site")
	account := workspaceAccount(t, primary.Username, entry.ID, 2001)
	host.accounts[account.Username] = account
	host.failures.deletion[account.Username] = errors.New("workspace process cannot terminate")

	err := NewRemover(host, host).Remove(context.Background(), primary, entry.ID)
	require.ErrorContains(t, err, "workspace "+account.Username+" deletion did not complete; catalog state, other local workspaces, and canonical repository were not modified")
	require.Contains(t, host.accounts, account.Username)
}

func TestProjectRemovalPreflightsEveryWorkspaceBeforeDeletingAny(t *testing.T) {
	host := newFakeAccountHost()
	entry := projectEntry("site")
	first := workspaceAccount(t, "alice", entry.ID, 2001)
	second := workspaceAccount(t, "bob", entry.ID, 2002)
	host.accounts[first.Username] = first
	host.accounts[second.Username] = second
	host.candidates = []linuxhost.Account{first, second}
	host.failures.preflight[second.Username] = errors.New("second workspace failed preflight")

	removed, err := NewRemover(host, host).RemoveProjectWorkspaces(
		context.Background(),
		entry,
		host.uidMin,
	)
	require.ErrorContains(t, err, "second workspace failed preflight")
	require.ErrorContains(t, err, "no local workspaces were removed; all local workspaces, shared catalog entry, and canonical repository remain")
	require.Empty(t, removed)
	require.Equal(t, []string{first.Username, second.Username}, host.calls.preflights)
	require.Empty(t, host.calls.deleted)
}

func TestProjectRemovalReportsPartialProgressAndSupportsRetry(t *testing.T) {
	host := newFakeAccountHost()
	entry := projectEntry("site")
	first := workspaceAccount(t, "alice", entry.ID, 2001)
	second := workspaceAccount(t, "bob", entry.ID, 2002)
	removedAccount, retainedAccount := first, second
	if retainedAccount.Username < removedAccount.Username {
		removedAccount, retainedAccount = retainedAccount, removedAccount
	}
	host.accounts[first.Username] = first
	host.accounts[second.Username] = second
	host.candidates = []linuxhost.Account{second, first}
	host.failures.deletion[retainedAccount.Username] = errors.New("workspace process cannot terminate")
	remover := NewRemover(host, host)

	removed, err := remover.RemoveProjectWorkspaces(context.Background(), entry, host.uidMin)
	require.ErrorContains(t, err, "removed local workspaces "+removedAccount.Username)
	require.ErrorContains(t, err, "local workspaces "+retainedAccount.Username+", shared catalog entry, and canonical repository remain")
	require.Equal(t, []string{removedAccount.Username}, removed)
	require.NotContains(t, host.accounts, removedAccount.Username)
	require.Contains(t, host.accounts, retainedAccount.Username)

	delete(host.failures.deletion, retainedAccount.Username)
	host.candidates = []linuxhost.Account{retainedAccount}
	removed, err = remover.RemoveProjectWorkspaces(context.Background(), entry, host.uidMin)
	require.NoError(t, err)
	require.Equal(t, []string{retainedAccount.Username}, removed)
	require.NotContains(t, host.accounts, retainedAccount.Username)
}

func TestProjectRemovalRejectsMalformedWorkspaceEvidence(t *testing.T) {
	host := newFakeAccountHost()
	entry := projectEntry("site")
	malformed := workspaceAccount(t, "alice", "other", 2001)
	malformed.GECOS = "not-a-workspace-marker"
	host.candidates = []linuxhost.Account{malformed}

	_, err := NewRemover(host, host).RemoveProjectWorkspaces(context.Background(), entry, host.uidMin)
	require.ErrorContains(t, err, "invalid workspace account marker")
	require.Empty(t, host.calls.preflights)
}
