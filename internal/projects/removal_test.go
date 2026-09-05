package projects

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/LevitateOS/soda-os/internal/projects/catalog"
	"github.com/stretchr/testify/require"
)

func confirmedRemovalInput(t *testing.T, fixture helperFixture, action, target string) string {
	t.Helper()
	request, err := json.Marshal(RemovalInspectionRequest{Action: action, Target: target})
	require.NoError(t, err)
	response, err := fixture.helper.Execute(context.Background(), helperAlice(), "removal-inspect", strings.NewReader(string(request)))
	require.NoError(t, err)
	preview := response.(RemovalInspectionResponse).Preview
	var payload any = RemoveProjectRequest{ID: target, Expected: preview.Revision}
	if action == "delete-human" {
		payload = DeleteHumanRequest{Username: target, Expected: preview.Revision}
	}
	encoded, err := json.Marshal(payload)
	require.NoError(t, err)
	return string(encoded)
}

func TestRemovalInspectionIsReadOnlyAndRestrictedToOwnedScope(t *testing.T) {
	fixture := newHelperFixture(t)
	fixture.host.accounts["alice"] = rootPrimary("alice")
	own := rootWorkspace(t, "alice", "site", 2001)
	other := rootWorkspace(t, "bob", "site", 2002)
	fixture.host.accounts[own.Username], fixture.host.accounts[other.Username] = own, other
	fixture.host.candidates = []linuxhost.Account{other, own}
	response, err := fixture.helper.Execute(context.Background(), helperAlice(), "removal-inspect", strings.NewReader(`{"action":"remove-workspace","target":"site"}`))
	require.NoError(t, err)
	preview := response.(RemovalInspectionResponse).Preview
	require.Equal(t, []RemovalAccount{{UID: own.UID, Username: own.Username, PrimaryUsername: "alice", ProjectID: "site", Home: own.Home}}, preview.Accounts)
	require.Empty(t, deletionEvents(fixture.host.events))
	for _, action := range []string{"remove", "delete-human", "run"} {
		_, err = fixture.helper.Execute(context.Background(), helperAlice(), "removal-inspect", strings.NewReader(`{"action":"`+action+`","target":"site"}`))
		require.Error(t, err)
	}
	_, err = fixture.helper.Execute(context.Background(), helperAlice(), "removal-inspect", strings.NewReader(`{"action":"remove-workspace","target":"site","path":"/etc"}`))
	require.ErrorContains(t, err, "unknown field")
}

func TestRemovalRejectsUnreviewedOrChangedScopeBeforeMutation(t *testing.T) {
	fixture := newHelperFixture(t)
	require.NoError(t, fixture.store.Add(catalog.Entry{ID: "site", DisplayName: "Site", CanonicalURL: "git@git.test:site.git"}))
	request := confirmedRemovalInput(t, fixture, "remove", "site")
	added := rootWorkspace(t, "bob", "site", 2001)
	fixture.host.accounts[added.Username] = added
	fixture.host.candidates = []linuxhost.Account{added}
	response, err := fixture.helper.Execute(context.Background(), helperAlice(), "project-remove", strings.NewReader(request))
	require.NoError(t, err)
	require.False(t, response.(RemovalResponse).OK)
	require.Contains(t, response.(RemovalResponse).Problem, "scope changed")
	require.Empty(t, deletionEvents(fixture.host.events))
	_, err = fixture.helper.Execute(context.Background(), helperAlice(), "project-remove", strings.NewReader(`{"id":"site"}`))
	require.ErrorContains(t, err, "inspect removal")
}

func TestRemovalInspectionWaitsForExclusiveOperations(t *testing.T) {
	fixture := newHelperFixture(t)
	held, err := fixture.helper.operationLocks.Exclusive()
	require.NoError(t, err)
	result := make(chan error, 1)
	go func() {
		_, err := fixture.helper.Execute(context.Background(), helperAlice(), "removal-inspect", strings.NewReader(`{"action":"remove-workspace","target":"site"}`))
		result <- err
	}()
	requireBlocked(t, result, "inspection bypassed removal lock")
	require.NoError(t, held.Close())
	require.NoError(t, requireResult(t, result))
	require.Empty(t, deletionEvents(fixture.host.events))
}

func TestCoordinatorRemovalUsesInspectionAndConfirmedWireRequest(t *testing.T) {
	fixture := newCoordinatorFixture(t, "")
	response, err := fixture.coordinator.Execute(context.Background(), "alice", "removal-inspect", strings.NewReader(`{"action":"remove","target":"site"}`))
	require.NoError(t, err)
	revision := response.(RemovalInspectionResponse).Preview.Revision
	request, err := json.Marshal(RemoveProjectRequest{ID: "site", Expected: revision})
	require.NoError(t, err)
	response, err = fixture.coordinator.Execute(context.Background(), "alice", "remove", strings.NewReader(string(request)))
	require.NoError(t, err)
	require.True(t, response.(RemovalResponse).OK)
	require.NoError(t, response.(RemovalResponse).Validate())
}

func TestRemovalRevisionTracksRepositoryNotCosmeticMetadata(t *testing.T) {
	scope := removalScope{Action: "remove", Target: "site", Accounts: []linuxhost.Account{}, Project: &catalog.Entry{ID: "site", DisplayName: "Site", CanonicalURL: "git@git.test:site.git"}}
	revision := scope.preview().Revision
	scope.Project.DisplayName = "Renamed"
	scope.Project.Additional = map[string]json.RawMessage{"team": json.RawMessage(`"web"`)}
	require.Equal(t, revision, scope.preview().Revision)
	scope.Project.CanonicalURL = "git@git.test:replacement.git"
	require.NotEqual(t, revision, scope.preview().Revision)
}

func TestFailedDeletionWithEmptyDiagnosticDoesNotRemoveCatalog(t *testing.T) {
	fixture := newHelperFixture(t)
	require.NoError(t, fixture.store.Add(catalog.Entry{ID: "site", DisplayName: "Site", CanonicalURL: "git@git.test:site.git"}))
	account := rootWorkspace(t, "alice", "site", 2001)
	fixture.host.accounts[account.Username] = account
	fixture.host.candidates = []linuxhost.Account{account}
	fixture.host.deleteErr[account.Username] = errors.New("")
	response, err := fixture.helper.Execute(context.Background(), helperAlice(), "project-remove", strings.NewReader(confirmedRemovalInput(t, fixture, "remove", "site")))
	require.NoError(t, err)
	receipt := response.(RemovalResponse)
	require.False(t, receipt.OK)
	require.NoError(t, receipt.Validate())
	require.Equal(t, account.Username, receipt.Result.Uncertain)
	require.Equal(t, "not_attempted", receipt.Catalog)
	_, err = fixture.store.Get("site")
	require.NoError(t, err)
}

func TestRemovalRejectsChangedAccountIdentity(t *testing.T) {
	fixture := newHelperFixture(t)
	account := rootWorkspace(t, "alice", "site", 2001)
	fixture.host.accounts[account.Username] = account
	request := confirmedRemovalInput(t, fixture, "remove-workspace", "site")
	account.UID++
	fixture.host.accounts[account.Username] = account
	response, err := fixture.helper.Execute(context.Background(), helperAlice(), "workspace-remove", strings.NewReader(request))
	require.NoError(t, err)
	require.False(t, response.(RemovalResponse).OK)
	require.Empty(t, deletionEvents(fixture.host.events))
}
