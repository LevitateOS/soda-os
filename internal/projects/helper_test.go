package projects

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/LevitateOS/soda-os/internal/projects/catalog"
	"github.com/LevitateOS/soda-os/internal/projects/people"
	"github.com/LevitateOS/soda-os/internal/projects/workspace"
	"github.com/stretchr/testify/require"
)

type helperFixture struct {
	helper Helper
	store  *catalog.Store
	host   *rootTestHost
}

type accountSequence struct {
	uidMin   int
	accounts []linuxhost.Account
	lookups  int
}

func (sequence *accountSequence) UIDMin() (int, error) { return sequence.uidMin, nil }

func (sequence *accountSequence) LookupAccount(_ context.Context, username string) (linuxhost.Account, error) {
	if len(sequence.accounts) == 0 || sequence.accounts[0].Username != username {
		return linuxhost.Account{}, linuxhost.ErrAccountNotFound
	}
	index := sequence.lookups
	if index >= len(sequence.accounts) {
		index = len(sequence.accounts) - 1
	}
	sequence.lookups++
	return sequence.accounts[index], nil
}

func newHelperFixture(t *testing.T) helperFixture {
	t.Helper()
	host := newRootTestHost()
	host.accounts["alice"] = rootAdministrator("alice")
	store := rootTestStore(t)
	return helperFixture{
		helper: Helper{
			store:          store,
			authorizer:     NewAuthorizer(host),
			workspaces:     workspace.NewAccounts(host, host, host, host),
			remover:        workspace.NewRemover(host, host),
			people:         people.Deletion{Host: host, Forgejo: rootTestForgejo{events: &host.events}},
			operationLocks: rootTestOperationLocker(t),
		},
		store: store, host: host,
	}
}

func helperAlice() linuxhost.PKExecIdentity {
	return linuxhost.PKExecIdentity{Username: "alice", UID: os.Getuid()}
}

func TestHelperCatalogActionsPreserveArbitraryMetadataAndImmutableURL(t *testing.T) {
	fixture := newHelperFixture(t)
	response, err := fixture.helper.Execute(context.Background(), helperAlice(), "catalog-add", strings.NewReader(
		`{"id":"site","display_name":"Site","canonical_url":"git@git.example.test:site.git","team":"web","labels":["public"]}`,
	))
	require.NoError(t, err)
	mutation := response.(ProjectMutationResponse)
	require.True(t, mutation.OK)
	require.Equal(t, "site", mutation.Project.ID)
	require.JSONEq(t, `"web"`, string(mutation.Project.CatalogMetadata["team"]))
	entry, err := fixture.store.Get("site")
	require.NoError(t, err)
	require.JSONEq(t, `"web"`, string(entry.Additional["team"]))

	response, err = fixture.helper.Execute(context.Background(), helperAlice(), "catalog-edit", strings.NewReader(
		`{"id":"site","display_name":"Renamed","owner":"alice"}`,
	))
	require.NoError(t, err)
	mutation = response.(ProjectMutationResponse)
	require.True(t, mutation.OK)
	require.Equal(t, "Renamed", mutation.Project.DisplayName)
	require.Equal(t, "git@git.example.test:site.git", mutation.Project.CanonicalURL)
	entry, err = fixture.store.Get("site")
	require.NoError(t, err)
	require.Equal(t, "Renamed", entry.DisplayName)
	require.Equal(t, "git@git.example.test:site.git", entry.CanonicalURL)
	require.JSONEq(t, `"alice"`, string(entry.Additional["owner"]))

	_, err = fixture.helper.Execute(context.Background(), helperAlice(), "catalog-edit", strings.NewReader(
		`{"id":"site","display_name":"Changed","canonical_url":"git@git.example.test:site.git"}`,
	))
	require.ErrorContains(t, err, `must not include "canonical_url"`)
	entry, err = fixture.store.Get("site")
	require.NoError(t, err)
	require.Equal(t, "Renamed", entry.DisplayName)
}

func TestHelperRejectsUnsupportedCommandsUnknownFieldsAndCallerMismatch(t *testing.T) {
	fixture := newHelperFixture(t)
	_, err := fixture.helper.Execute(context.Background(), helperAlice(), "run", strings.NewReader(`{}`))
	require.ErrorContains(t, err, "unsupported")

	_, err = fixture.helper.Execute(context.Background(), helperAlice(), "workspace-publish", strings.NewReader(
		`{"id":"site","canonical_url":"git@git.example.test:site.git","path":"/etc"}`,
	))
	require.ErrorContains(t, err, "unknown field")

	_, err = fixture.helper.Execute(context.Background(), linuxhost.PKExecIdentity{Username: "alice", UID: os.Getuid() + 1}, "catalog-add", strings.NewReader(`{}`))
	require.ErrorContains(t, err, "no longer matches")

	workspaceAccount := rootWorkspace(t, "alice", "site", 2000)
	fixture.host.accounts[workspaceAccount.Username] = workspaceAccount
	_, err = fixture.helper.Execute(context.Background(), linuxhost.PKExecIdentity{Username: workspaceAccount.Username, UID: workspaceAccount.UID}, "catalog-add", strings.NewReader(`{}`))
	require.ErrorContains(t, err, "not a supported primary")
}

func TestHelperWorkspaceBoundaryRejectsInjectedCanonicalURL(t *testing.T) {
	fixture := newHelperFixture(t)
	require.NoError(t, fixture.store.Add(catalog.Entry{
		ID: "site", DisplayName: "Site", CanonicalURL: "git@git.example.test:site.git",
	}))

	_, err := fixture.helper.Execute(context.Background(), helperAlice(), "workspace-prepare", strings.NewReader(
		`{"id":"site","canonical_url":"git@git.example.test:other.git"}`,
	))
	require.ErrorContains(t, err, "project URL changed")
	require.Empty(t, fixture.host.events)
}

func TestHelperOwnWorkspaceRemovalLeavesCatalogAndOtherWorkspaces(t *testing.T) {
	fixture := newHelperFixture(t)
	entry := catalog.Entry{ID: "site", DisplayName: "Site", CanonicalURL: "git@git.example.test:site.git"}
	require.NoError(t, fixture.store.Add(entry))
	aliceWorkspace := rootWorkspace(t, "alice", "site", 2000)
	bobWorkspace := rootWorkspace(t, "bob", "site", 2001)
	fixture.host.accounts[aliceWorkspace.Username] = aliceWorkspace
	fixture.host.accounts[bobWorkspace.Username] = bobWorkspace

	response, err := fixture.helper.Execute(context.Background(), helperAlice(), "workspace-remove", strings.NewReader(`{"id":"site"}`))
	require.NoError(t, err)
	require.Equal(t, SuccessResponse{OK: true}, response)
	require.NotContains(t, fixture.host.accounts, aliceWorkspace.Username)
	require.Contains(t, fixture.host.accounts, bobWorkspace.Username)
	_, err = fixture.store.Get(entry.ID)
	require.NoError(t, err)
}

func TestHelperOwnWorkspaceRemovalDoesNotRequireCatalogEntry(t *testing.T) {
	fixture := newHelperFixture(t)
	aliceWorkspace := rootWorkspace(t, "alice", "retired", 2000)
	fixture.host.accounts[aliceWorkspace.Username] = aliceWorkspace

	response, err := fixture.helper.Execute(context.Background(), helperAlice(), "workspace-remove", strings.NewReader(`{"id":"retired"}`))
	require.NoError(t, err)
	require.Equal(t, SuccessResponse{OK: true}, response)
	require.NotContains(t, fixture.host.accounts, aliceWorkspace.Username)
}

func TestHelperProjectRemovalDeletesWorkspacesBeforeCatalog(t *testing.T) {
	fixture := newHelperFixture(t)
	entry := catalog.Entry{ID: "site", DisplayName: "Site", CanonicalURL: "git@git.example.test:site.git"}
	require.NoError(t, fixture.store.Add(entry))
	first := rootWorkspace(t, "alice", "site", 2000)
	second := rootWorkspace(t, "bob", "site", 2001)
	fixture.host.accounts[first.Username] = first
	fixture.host.accounts[second.Username] = second
	fixture.host.candidates = []linuxhost.Account{second, first}

	response, err := fixture.helper.Execute(context.Background(), helperAlice(), "project-remove", strings.NewReader(`{"id":"site"}`))
	require.NoError(t, err)
	require.Equal(t, SuccessResponse{OK: true}, response)
	_, err = fixture.store.Get(entry.ID)
	require.ErrorContains(t, err, "does not exist")
	require.NotContains(t, fixture.host.accounts, first.Username)
	require.NotContains(t, fixture.host.accounts, second.Username)

	encoded, err := json.Marshal(response)
	require.NoError(t, err)
	require.JSONEq(t, `{"ok":true}`, string(encoded))
}

func TestHelperProjectRemovalRetainsCatalogAfterPartialFailure(t *testing.T) {
	fixture := newHelperFixture(t)
	entry := catalog.Entry{ID: "site", DisplayName: "Site", CanonicalURL: "git@git.example.test:site.git"}
	require.NoError(t, fixture.store.Add(entry))
	workspaces := []linuxhost.Account{
		rootWorkspace(t, "alice", "site", 2000),
		rootWorkspace(t, "bob", "site", 2001),
	}
	sort.Slice(workspaces, func(i, j int) bool { return workspaces[i].Username < workspaces[j].Username })
	for _, account := range workspaces {
		fixture.host.accounts[account.Username] = account
	}
	fixture.host.candidates = []linuxhost.Account{workspaces[1], workspaces[0]}
	fixture.host.deleteErr[workspaces[1].Username] = errors.New("workspace process cannot terminate")

	_, err := fixture.helper.Execute(context.Background(), helperAlice(), "project-remove", strings.NewReader(`{"id":"site"}`))
	require.ErrorContains(t, err, "removed local workspaces "+workspaces[0].Username)
	require.ErrorContains(t, err, "shared catalog entry, and canonical repository remain")
	_, getErr := fixture.store.Get(entry.ID)
	require.NoError(t, getErr)
	require.NotContains(t, fixture.host.accounts, workspaces[0].Username)
	require.Contains(t, fixture.host.accounts, workspaces[1].Username)
}

func TestHelperReauthorizesAdministratorInsideProjectRemovalLocks(t *testing.T) {
	fixture := newHelperFixture(t)
	entry := catalog.Entry{ID: "site", DisplayName: "Site", CanonicalURL: "git@git.example.test:site.git"}
	require.NoError(t, fixture.store.Add(entry))
	sequence := &accountSequence{
		uidMin: fixture.host.uidMin,
		accounts: []linuxhost.Account{
			rootAdministrator("alice"),
			rootPrimary("alice"),
		},
	}
	fixture.helper.authorizer = NewAuthorizer(sequence)

	_, err := fixture.helper.Execute(context.Background(), helperAlice(), "project-remove", strings.NewReader(`{"id":"site"}`))
	require.ErrorContains(t, err, "administrator status is required")
	require.Equal(t, 2, sequence.lookups)
	_, err = fixture.store.Get(entry.ID)
	require.NoError(t, err)
}

func TestHelperHumanDeletionUsesNoCatalogLock(t *testing.T) {
	fixture := newHelperFixture(t)
	fixture.host.accounts["target"] = rootPrimary("target")
	workspaceAccount := rootWorkspace(t, "target", "site", 2000)
	fixture.host.accounts[workspaceAccount.Username] = workspaceAccount
	fixture.host.candidates = []linuxhost.Account{workspaceAccount}

	locked, err := fixture.store.Lock()
	require.NoError(t, err)
	result := make(chan error, 1)
	go func() {
		_, executeErr := fixture.helper.Execute(context.Background(), helperAlice(), "human-delete", strings.NewReader(`{"username":"target"}`))
		result <- executeErr
	}()
	require.NoError(t, requireResult(t, result), "human deletion must not wait for the catalog lock")
	require.NoError(t, locked.Close())
	require.Equal(t, []string{"linux:" + workspaceAccount.Username, "forgejo:target", "linux:target"}, deletionEvents(fixture.host.events))
}

func deletionEvents(events []string) []string {
	result := []string{}
	for _, event := range events {
		if strings.HasPrefix(event, "linux:") || strings.HasPrefix(event, "forgejo:") {
			result = append(result, event)
		}
	}
	return result
}
