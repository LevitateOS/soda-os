package people

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/LevitateOS/soda-os/internal/projects/workspace"
	"github.com/stretchr/testify/require"
)

type deletionHost struct {
	uidMin       int
	accounts     map[string]linuxhost.Account
	candidates   []linuxhost.Account
	preflightErr map[string]error
	deleteErr    map[string]error
	events       []string
}

func newDeletionHost() *deletionHost {
	return &deletionHost{
		uidMin: 1000, accounts: map[string]linuxhost.Account{},
		preflightErr: map[string]error{}, deleteErr: map[string]error{},
	}
}

func (host *deletionHost) UIDMin() (int, error) { return host.uidMin, nil }

func (host *deletionHost) LookupAccount(_ context.Context, username string) (linuxhost.Account, error) {
	account, found := host.accounts[username]
	if !found {
		return linuxhost.Account{}, linuxhost.ErrAccountNotFound
	}
	return account, nil
}

func (host *deletionHost) CandidateAccounts(context.Context, string, string) ([]linuxhost.Account, error) {
	return append([]linuxhost.Account(nil), host.candidates...), nil
}

func (host *deletionHost) PasswordStatus(_ context.Context, account linuxhost.Account) (linuxhost.PasswordStatus, error) {
	host.events = append(host.events, "password:"+account.Username)
	return linuxhost.PasswordLocked, nil
}

func (host *deletionHost) PreflightDeleteAccount(_ context.Context, account linuxhost.Account) error {
	host.events = append(host.events, "preflight:"+account.Username)
	return host.preflightErr[account.Username]
}

func (host *deletionHost) DeleteAccount(_ context.Context, account linuxhost.Account) error {
	host.events = append(host.events, "linux:"+account.Username)
	if err := host.deleteErr[account.Username]; err != nil {
		return err
	}
	delete(host.accounts, account.Username)
	return nil
}

type deletionForgejo struct {
	err    error
	events *[]string
}

func (forgejo deletionForgejo) DeleteUser(_ context.Context, username string) error {
	*forgejo.events = append(*forgejo.events, "forgejo:"+username)
	return forgejo.err
}

func primaryAccount(username string) linuxhost.Account {
	groups := map[string]bool{username: true}
	return linuxhost.Account{
		Username: username, UID: 1000, GID: 1000, PrimaryGroup: username,
		Home: "/home/" + username, Shell: "/bin/bash", Groups: groups,
	}
}

func administratorAccount(username string) linuxhost.Account {
	account := primaryAccount(username)
	account.Groups[linuxhost.AdministratorGroup] = true
	return account
}

func derivedAccount(t *testing.T, primary, projectID string, uid int) linuxhost.Account {
	t.Helper()
	username, err := workspace.DerivedUsername(primary, projectID)
	require.NoError(t, err)
	marker, err := workspace.Marker(primary, projectID)
	require.NoError(t, err)
	return linuxhost.Account{
		Username: username, UID: uid, GID: uid, PrimaryGroup: username, GECOS: marker,
		Home: "/home/" + username, Shell: workspace.Shell,
		Groups: map[string]bool{username: true, workspace.Group: true},
	}
}

type deletionFixture struct {
	deletion   Deletion
	host       *deletionHost
	forgejo    *deletionForgejo
	workspaces []linuxhost.Account
}

func humanDeletion(t *testing.T, projects ...string) deletionFixture {
	t.Helper()
	host := newDeletionHost()
	host.accounts["admin"] = administratorAccount("admin")
	host.accounts["alice"] = primaryAccount("alice")
	workspaces := make([]linuxhost.Account, 0, len(projects))
	for index, projectID := range projects {
		account := derivedAccount(t, "alice", projectID, 2000+index)
		host.accounts[account.Username] = account
		workspaces = append(workspaces, account)
	}
	host.candidates = append([]linuxhost.Account(nil), workspaces...)
	forgejo := &deletionForgejo{events: &host.events}
	return deletionFixture{
		deletion: Deletion{Host: host, Forgejo: forgejo},
		host:     host, forgejo: forgejo, workspaces: workspaces,
	}
}

func deleteAlice(deletion Deletion, host *deletionHost) error {
	return deletion.Delete(context.Background(), host.accounts["admin"], host.uidMin, "alice")
}

func TestHumanDeletionRemovesWorkspacesThenForgejoThenLinuxPrimary(t *testing.T) {
	fixture := humanDeletion(t, "tools", "site")

	require.NoError(t, deleteAlice(fixture.deletion, fixture.host))
	sort.Slice(fixture.workspaces, func(i, j int) bool { return fixture.workspaces[i].Username < fixture.workspaces[j].Username })
	wantTail := []string{"linux:" + fixture.workspaces[0].Username, "linux:" + fixture.workspaces[1].Username, "forgejo:alice", "linux:alice"}
	require.Equal(t, wantTail, fixture.host.events[len(fixture.host.events)-len(wantTail):])
	require.NotContains(t, fixture.host.accounts, "alice")
}

func TestHumanDeletionPreflightsEveryAccountBeforeMutation(t *testing.T) {
	fixture := humanDeletion(t, "site", "tools")
	fixture.host.preflightErr[fixture.workspaces[1].Username] = errors.New("second workspace failed preflight")

	err := deleteAlice(fixture.deletion, fixture.host)
	require.ErrorContains(t, err, "second workspace failed preflight")
	require.NotContains(t, strings.Join(fixture.host.events, "\n"), "linux:")
	for _, account := range append(fixture.workspaces, fixture.host.accounts["alice"]) {
		require.Contains(t, fixture.host.accounts, account.Username)
	}
	require.Equal(t, "preflight:alice", fixture.host.events[0], "the primary account must be safe before workspace deletion begins")
}

func TestHumanDeletionReportsPartialWorkspaceProgress(t *testing.T) {
	fixture := humanDeletion(t, "site", "tools", "docs")
	sort.Slice(fixture.workspaces, func(i, j int) bool { return fixture.workspaces[i].Username < fixture.workspaces[j].Username })
	fixture.host.deleteErr[fixture.workspaces[1].Username] = errors.New("workspace process cannot terminate")

	err := deleteAlice(fixture.deletion, fixture.host)
	require.ErrorContains(t, err, "removed Soda workspaces "+fixture.workspaces[0].Username)
	require.ErrorContains(t, err, "workspaces "+fixture.workspaces[1].Username+", "+fixture.workspaces[2].Username+", Forgejo account, and primary Linux account remain")
	require.Contains(t, fixture.host.accounts, fixture.workspaces[2].Username)
	require.Contains(t, fixture.host.accounts, "alice")
}

func TestHumanDeletionRetainsPrimaryAfterForgejoFailure(t *testing.T) {
	fixture := humanDeletion(t, "site")
	fixture.forgejo.err = errors.New("Forgejo refuses deletion")

	err := deleteAlice(fixture.deletion, fixture.host)
	require.ErrorContains(t, err, "removed Soda workspaces "+fixture.workspaces[0].Username+"; Forgejo account and primary Linux account alice remain")
	require.Contains(t, fixture.host.accounts, "alice")
	require.Contains(t, fixture.host.events, "forgejo:alice")
	require.NotContains(t, fixture.host.events, "linux:alice")
}

func TestHumanDeletionCompletesWhenForgejoUserWasAlreadyRemoved(t *testing.T) {
	fixture := humanDeletion(t)
	fixture.forgejo.err = ErrForgejoUserNotFound

	require.NoError(t, deleteAlice(fixture.deletion, fixture.host))
	require.NotContains(t, fixture.host.accounts, "alice")
}

func TestHumanDeletionRequiresLinuxAdministrator(t *testing.T) {
	fixture := humanDeletion(t)
	fixture.host.accounts["admin"] = primaryAccount("admin")

	err := deleteAlice(fixture.deletion, fixture.host)
	require.ErrorContains(t, err, "administrator status is required")
	require.Empty(t, fixture.host.events)
}

func TestHumanDeletionPrimaryPreflightFailureBlocksEveryMutation(t *testing.T) {
	fixture := humanDeletion(t, "site")
	fixture.host.preflightErr["alice"] = errors.New("primary failed preflight")

	err := deleteAlice(fixture.deletion, fixture.host)
	require.ErrorContains(t, err, "primary failed preflight")
	require.Equal(t, []string{"preflight:alice"}, fixture.host.events)
	require.Contains(t, fixture.host.accounts, "alice")
	require.Contains(t, fixture.host.accounts, fixture.workspaces[0].Username)
}

func TestHumanDeletionRetainsAnotherHumansWorkspace(t *testing.T) {
	fixture := humanDeletion(t, "site")
	bob := derivedAccount(t, "bob", "site", 3000)
	fixture.host.accounts[bob.Username] = bob
	fixture.host.candidates = append(fixture.host.candidates, bob)

	require.NoError(t, deleteAlice(fixture.deletion, fixture.host))
	require.Contains(t, fixture.host.accounts, bob.Username)
	require.NotContains(t, fixture.host.events, "linux:"+bob.Username)
}

func TestHumanDeletionRejectsNonPrimaryTarget(t *testing.T) {
	fixture := humanDeletion(t, "site")
	fixture.host.accounts["alice"] = fixture.workspaces[0]

	err := deleteAlice(fixture.deletion, fixture.host)
	require.ErrorContains(t, err, "target is not a supported primary Linux account")
	require.Empty(t, fixture.host.events)
}
