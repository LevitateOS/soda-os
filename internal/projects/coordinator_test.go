package projects

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/projects/catalog"
	"github.com/LevitateOS/soda-os/internal/projects/workspace"
	"github.com/stretchr/testify/require"
)

type coordinatorFixture struct {
	coordinator Coordinator
	store       *catalog.Store
	host        *rootTestHost
	pkexec      testPKExec
}

func newCoordinatorFixture(t *testing.T, failAction string) coordinatorFixture {
	t.Helper()
	host := newRootTestHost()
	alice := rootAdministrator("alice")
	host.accounts[alice.Username] = alice
	store := rootTestStore(t)
	pkexec := newTestPKExec(t, failAction)
	return coordinatorFixture{
		coordinator: Coordinator{
			store:          store,
			authorizer:     NewAuthorizer(host),
			workspaces:     workspace.NewAccounts(host, host, host, host),
			setupLocks:     rootTestSetupLocker(t, alice),
			operationLocks: rootTestOperationLocker(t),
			privileged:     pkexec.invoker,
		},
		store: store, host: host, pkexec: pkexec,
	}
}

func TestCoordinatorAddExistingPreservesArbitraryCatalogMetadata(t *testing.T) {
	fixture := newCoordinatorFixture(t, "")
	response, err := fixture.coordinator.Execute(context.Background(), "alice", "add-existing", strings.NewReader(
		`{"id":"site","display_name":"Site","canonical_url":"git@git.example.test:site.git","team":"web","labels":["public"]}`,
	))
	require.NoError(t, err)
	require.Equal(t, []string{"catalog-add"}, fixture.pkexec.actions(t))
	require.Len(t, fixture.pkexec.requests(t), 1)
	require.JSONEq(t, `{"id":"site","display_name":"Site","canonical_url":"git@git.example.test:site.git","team":"web","labels":["public"]}`, fixture.pkexec.requests(t)[0])

	mutation := response.(ProjectMutationResponse)
	require.True(t, mutation.OK)
	require.JSONEq(t, `"web"`, string(mutation.Project.CatalogMetadata["team"]))
}

func TestCoordinatorEditPreservesImmutableCanonicalURL(t *testing.T) {
	fixture := newCoordinatorFixture(t, "")
	require.NoError(t, fixture.store.Add(catalog.Entry{
		ID: "site", DisplayName: "Site", CanonicalURL: "git@git.example.test:site.git",
		Additional: map[string]json.RawMessage{"owner": json.RawMessage(`"old-owner"`)},
	}))
	response, err := fixture.coordinator.Execute(context.Background(), "alice", "edit", strings.NewReader(
		`{"id":"site","display_name":"Renamed","owner":"new-owner","labels":["public"]}`,
	))
	require.NoError(t, err)
	require.Equal(t, []string{"catalog-edit"}, fixture.pkexec.actions(t))
	require.JSONEq(t, `{"id":"site","display_name":"Renamed","owner":"new-owner","labels":["public"]}`, fixture.pkexec.requests(t)[0])

	mutation := response.(ProjectMutationResponse)
	require.Equal(t, "Renamed", mutation.Project.DisplayName)
	require.Equal(t, "git@git.example.test:site.git", mutation.Project.CanonicalURL)
	require.JSONEq(t, `"new-owner"`, string(mutation.Project.CatalogMetadata["owner"]))
}

func TestCoordinatorEditRejectsInjectedCanonicalURLBeforePrivilege(t *testing.T) {
	fixture := newCoordinatorFixture(t, "")
	require.NoError(t, fixture.store.Add(catalog.Entry{ID: "site", DisplayName: "Site", CanonicalURL: "git@git.example.test:site.git"}))

	_, err := fixture.coordinator.Execute(context.Background(), "alice", "edit", strings.NewReader(
		`{"id":"site","display_name":"Renamed","canonical_url":"git@git.example.test:site.git"}`,
	))
	require.ErrorContains(t, err, `must not include "canonical_url"`)
	require.Empty(t, fixture.pkexec.actions(t))
}

func TestCoordinatorListReportsOnlyValidatedAccountExistence(t *testing.T) {
	fixture := newCoordinatorFixture(t, "")
	entry := catalog.Entry{
		ID: "site", DisplayName: "Site", CanonicalURL: "git@git.example.test:site.git",
		Additional: map[string]json.RawMessage{
			"catalog_metadata": json.RawMessage(`"catalog-value"`),
			"workspace_exists": json.RawMessage(`"catalog-value"`),
		},
	}
	require.NoError(t, fixture.store.Add(entry))
	workspaceAccount := rootWorkspace(t, "alice", "site", 2000)
	fixture.host.accounts[workspaceAccount.Username] = workspaceAccount

	response, err := fixture.coordinator.Execute(context.Background(), "alice", "list", strings.NewReader(`{}`))
	require.NoError(t, err)
	list := response.(ListResponse)
	require.Equal(t, CurrentUserView{Username: "alice", Administrator: true}, list.CurrentUser)
	require.Len(t, list.Projects, 1)
	require.Equal(t, workspaceAccount.Username, list.Projects[0].WorkspaceUsername)
	require.True(t, list.Projects[0].WorkspaceExists)
	encoded, err := json.Marshal(list)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"catalog_metadata":{"catalog_metadata":"catalog-value","workspace_exists":"catalog-value"}`)
}

func TestCoordinatorSetupHoldsCompositionAndReturnsSetupResponse(t *testing.T) {
	fixture := newCoordinatorFixture(t, "")
	require.NoError(t, fixture.store.Add(catalog.Entry{
		ID: "site", DisplayName: "Site", CanonicalURL: "git@localhost:team/site.git",
	}))

	response, err := fixture.coordinator.Execute(context.Background(), "alice", "setup", strings.NewReader(`{"id":"site"}`))
	require.NoError(t, err)
	require.Equal(t, SetupResponse{OK: true, WorkspaceUsername: "soda-w-example"}, response)
	require.Equal(t, []string{"workspace-prepare", "workspace-publish"}, fixture.pkexec.actions(t))
	for _, request := range fixture.pkexec.requests(t) {
		require.JSONEq(t, `{"id":"site","canonical_url":"git@localhost:team/site.git"}`, request)
	}
}

func TestCoordinatorSetupReportsManualAuthoritativeHostRetry(t *testing.T) {
	fixture := newCoordinatorFixture(t, "workspace-publish")
	require.NoError(t, fixture.store.Add(catalog.Entry{
		ID: "site", DisplayName: "Site", CanonicalURL: "ssh://git@git.example.test/team/site.git",
	}))

	_, err := fixture.coordinator.Execute(context.Background(), "alice", "setup", strings.NewReader(`{"id":"site"}`))
	require.ErrorContains(t, err, "workspace soda-w-example and its outbound Git key were retained")
	require.ErrorContains(t, err, "ssh-ed25519 test")
	require.ErrorContains(t, err, "authoritative Git host")
	require.ErrorContains(t, err, "register that key")
	require.ErrorContains(t, err, "retry setup")
	require.ErrorContains(t, err, "native SSH authentication failed")
}

func TestCoordinatorRejectsUnknownActionAndFields(t *testing.T) {
	fixture := newCoordinatorFixture(t, "")
	_, err := fixture.coordinator.Execute(context.Background(), "alice", "shell", strings.NewReader(`{}`))
	require.ErrorContains(t, err, "unsupported")

	_, err = fixture.coordinator.Execute(context.Background(), "alice", "remove", strings.NewReader(`{"id":"site","command":"rm"}`))
	require.ErrorContains(t, err, "unknown field")
	require.Empty(t, fixture.pkexec.actions(t))
}
