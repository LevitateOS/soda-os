package workspace

import (
	"context"
	"errors"
	"testing"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/stretchr/testify/require"
)

func TestRemovalTargetsOnlyTheDerivedWorkspaceWithoutMutation(t *testing.T) {
	host := newFakeAccountHost()
	primary := primaryAccount("alice")
	alice := workspaceAccount(t, "alice", "site", 2001)
	bob := workspaceAccount(t, "bob", "site", 2002)
	host.accounts[alice.Username], host.accounts[bob.Username] = alice, bob
	remover := NewRemover(host, host)
	targets, err := remover.Targets(context.Background(), primary, "site")
	require.NoError(t, err)
	require.Equal(t, []linuxhost.Account{alice}, targets)
	require.Empty(t, host.calls.deleted)
	receipt := linuxhost.DeleteAccounts(context.Background(), host, targets)
	require.Equal(t, []string{alice.Username}, receipt.Removed)
	require.Contains(t, host.accounts, bob.Username)
	targets, err = remover.Targets(context.Background(), primary, "site")
	require.NoError(t, err)
	require.Empty(t, targets)
}

func TestProjectRemovalPreflightsEveryWorkspaceBeforeDeletingAny(t *testing.T) {
	host := newFakeAccountHost()
	first := workspaceAccount(t, "alice", "site", 2001)
	second := workspaceAccount(t, "bob", "site", 2002)
	host.accounts[first.Username], host.accounts[second.Username] = first, second
	host.candidates = []linuxhost.Account{first, second}
	host.failures.preflight[second.Username] = errors.New("second workspace failed preflight")
	targets, err := NewRemover(host, host).ProjectTargets(context.Background(), "site", host.uidMin)
	require.ErrorContains(t, err, "second workspace failed preflight")
	require.Empty(t, targets)
	require.Equal(t, []string{first.Username, second.Username}, host.calls.preflights)
	require.Empty(t, host.calls.deleted)
}

func TestProjectRemovalReceiptSeparatesConfirmedAndUncertainDeletion(t *testing.T) {
	host := newFakeAccountHost()
	first := workspaceAccount(t, "alice", "site", 2001)
	second := workspaceAccount(t, "bob", "site", 2002)
	host.accounts[first.Username], host.accounts[second.Username] = first, second
	host.candidates = []linuxhost.Account{second, first}
	remover := NewRemover(host, host)
	targets, err := remover.ProjectTargets(context.Background(), "site", host.uidMin)
	require.NoError(t, err)
	require.Less(t, targets[0].Username, targets[1].Username)
	host.failures.deletion[targets[1].Username] = errors.New("workspace process cannot terminate")
	receipt := linuxhost.DeleteAccounts(context.Background(), host, targets)
	require.Equal(t, []string{targets[0].Username}, receipt.Removed)
	require.Equal(t, targets[1].Username, receipt.Uncertain)
	require.Empty(t, receipt.NotAttempted)
	require.Contains(t, receipt.Diagnostic, "cannot terminate")
	delete(host.failures.deletion, targets[1].Username)
	host.candidates = targets[1:]
	targets, err = remover.ProjectTargets(context.Background(), "site", host.uidMin)
	require.NoError(t, err)
	receipt = linuxhost.DeleteAccounts(context.Background(), host, targets)
	require.Len(t, receipt.Removed, 1)
	require.Empty(t, receipt.Uncertain)
}

func TestProjectRemovalRejectsMalformedWorkspaceEvidence(t *testing.T) {
	host := newFakeAccountHost()
	malformed := workspaceAccount(t, "alice", "other", 2001)
	malformed.GECOS = "not-a-workspace-marker"
	host.candidates = []linuxhost.Account{malformed}
	_, err := NewRemover(host, host).ProjectTargets(context.Background(), "site", host.uidMin)
	require.ErrorContains(t, err, "invalid workspace account marker")
	require.Empty(t, host.calls.preflights)
}
