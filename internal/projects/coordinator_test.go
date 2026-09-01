package projects

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakeEndpoints struct {
	forgejoURL string
}

func (endpoints fakeEndpoints) Endpoints(context.Context) (string, string, error) {
	forgejoURL := endpoints.forgejoURL
	if forgejoURL == "" {
		forgejoURL = "http://soda.example.ts.net:30000"
	}
	return forgejoURL, "soda.example.ts.net", nil
}

type fakePrivileged struct {
	action            string
	request           any
	err               error
	workspaceUsername string
}

func (privileged *fakePrivileged) record(action string, request any) error {
	privileged.action, privileged.request = action, request
	return privileged.err
}

func (privileged *fakePrivileged) CatalogAdd(_ context.Context, request HelperCatalogRequest) error {
	return privileged.record("catalog-add", request)
}

func (privileged *fakePrivileged) CatalogEdit(_ context.Context, request HelperCatalogRequest) error {
	return privileged.record("catalog-edit", request)
}

func (privileged *fakePrivileged) WorkspacePublish(_ context.Context, request HelperWorkspaceRequest) (MutationResponse, error) {
	err := privileged.record("workspace-publish", request)
	username := privileged.workspaceUsername
	if username == "" {
		username = "soda-w-example"
	}
	return MutationResponse{OK: err == nil, WorkspaceUsername: username}, err
}

func (privileged *fakePrivileged) ProjectRemove(_ context.Context, request ProjectRequest) error {
	return privileged.record("project-remove", request)
}

func (privileged *fakePrivileged) HumanDelete(_ context.Context, request HelperHumanRequest) error {
	return privileged.record("human-delete", request)
}

type fakeCloner struct {
	remote      string
	credentials CloneCredentials
}

func (cloner *fakeCloner) Clone(_ context.Context, remote, _ string, credentials CloneCredentials) error {
	cloner.remote, cloner.credentials = remote, credentials
	return nil
}

type coordinatorFixture struct {
	coordinator Coordinator
	platform    *fakePlatform
	privileged  *fakePrivileged
	cloner      *fakeCloner
}

func testCoordinator(t *testing.T) coordinatorFixture {
	t.Helper()
	catalog := testCatalog(t)
	platform := newFakePlatform()
	platform.accounts["alice"] = primaryAccount("alice", primaryRoleAdministrator)
	privileged := &fakePrivileged{}
	cloner := &fakeCloner{}
	lifecycle := Lifecycle{Catalog: catalog, Platform: platform}
	return coordinatorFixture{
		coordinator: Coordinator{Catalog: catalog, Lifecycle: lifecycle, Platform: platform, Privileged: privileged, Forgejo: ForgejoClient{}, Cloner: cloner, Endpoints: fakeEndpoints{}},
		platform:    platform,
		privileged:  privileged,
		cloner:      cloner,
	}
}

func configureForgejo(t *testing.T, coordinator *Coordinator, calls *int, password *string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		*calls = *calls + 1
		_, receivedPassword, _ := request.BasicAuth()
		*password = receivedPassword
		writer.WriteHeader(http.StatusCreated)
		_, _ = writer.Write([]byte(`{"name":"site","clone_url":"https://forgejo.example.test/alice/site.git","empty":true,"owner":{"login":"alice"}}`))
	}))
	coordinator.Endpoints = fakeEndpoints{forgejoURL: server.URL}
	coordinator.Forgejo = ForgejoClient{}
	return server
}

func TestCoordinatorListContract(t *testing.T) {
	coordinator := testCoordinator(t).coordinator
	require.NoError(t, coordinator.Catalog.Add(CatalogEntry{ID: "site", DisplayName: "Site", CanonicalURL: "https://git.example.test/site.git"}))
	response, err := coordinator.Execute(context.Background(), "alice", "list", strings.NewReader(`{}`))
	require.NoError(t, err)
	list := response.(ListResponse)
	require.Equal(t, CurrentUserView{Username: "alice", Administrator: true}, list.CurrentUser)
	require.Equal(t, "http://soda.example.ts.net:30000", list.ForgejoURL)
	require.Equal(t, "soda.example.ts.net", list.SSHHost)
	require.Len(t, list.Projects, 1)
	require.NotEmpty(t, list.Projects[0].WorkspaceUsername)
	require.NotEmpty(t, list.Projects[0].WorkspaceUsername)
}

func TestCoordinatorSetupKeepsCredentialsOutOfPrivilegedRequest(t *testing.T) {
	fixture := testCoordinator(t)
	coordinator, platform := fixture.coordinator, fixture.platform
	privileged, cloner := fixture.privileged, fixture.cloner
	entry := CatalogEntry{ID: "site", DisplayName: "Site", CanonicalURL: "https://git.example.test/site.git"}
	require.NoError(t, coordinator.Catalog.Add(entry))
	response, err := coordinator.Execute(context.Background(), "alice", "setup", strings.NewReader(`{"id":"site","git_username":"alice","git_password":"secret"}`))
	require.NoError(t, err)
	require.Equal(t, MutationResponse{OK: true, WorkspaceUsername: "soda-w-example"}, response)
	require.Equal(t, CloneCredentials{Username: "alice", Password: "secret"}, cloner.credentials)
	require.Equal(t, "workspace-publish", privileged.action)
	encoded, err := json.Marshal(privileged.request)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "secret")
	require.NotContains(t, string(encoded), "git_username")
	require.Equal(t, []string{"reset:site", "prepare:site", "cleanup:site"}, platform.calls.reset)
}

func TestCoordinatorSetupSurfacesCleanupFailureAfterPublication(t *testing.T) {
	fixture := testCoordinator(t)
	coordinator, platform := fixture.coordinator, fixture.platform
	require.NoError(t, coordinator.Catalog.Add(CatalogEntry{ID: "site", DisplayName: "Site", CanonicalURL: "https://git.example.test/site.git"}))
	platform.failures.cleanupErr = fmt.Errorf("staging remained")

	_, err := coordinator.Execute(context.Background(), "alice", "setup", strings.NewReader(`{"id":"site","git_username":"alice","git_password":"secret"}`))
	require.ErrorContains(t, err, "clean clone staging directory")
	require.Equal(t, []string{"reset:site", "prepare:site", "cleanup:site"}, platform.calls.reset)
}

func TestCoordinatorMissingKeyFailsBeforeCloneOrPrivilegedMutation(t *testing.T) {
	fixture := testCoordinator(t)
	coordinator, platform := fixture.coordinator, fixture.platform
	privileged, cloner := fixture.privileged, fixture.cloner
	entry := CatalogEntry{ID: "site", DisplayName: "Site", CanonicalURL: "https://git.example.test/site.git"}
	require.NoError(t, coordinator.Catalog.Add(entry))
	platform.keys = nil

	_, err := coordinator.Execute(context.Background(), "alice", "setup", strings.NewReader(`{"id":"site","git_username":"alice","git_password":"secret"}`))
	require.ErrorContains(t, err, "requires a public key")
	require.Empty(t, cloner.remote)
	require.Empty(t, privileged.action)
	require.Empty(t, platform.calls.reset)
}

func TestCoordinatorCreateForgejoPreservesPasswordAtUnprivilegedBoundary(t *testing.T) {
	fixture := testCoordinator(t)
	coordinator, privileged := fixture.coordinator, fixture.privileged
	var calls int
	var password string
	server := configureForgejo(t, &coordinator, &calls, &password)
	defer server.Close()
	response, err := coordinator.Execute(context.Background(), "alice", "create-forgejo", strings.NewReader(`{"id":"site","display_name":"Site","password":"secret"}`))
	require.NoError(t, err)
	require.True(t, response.(MutationResponse).OK)
	require.Equal(t, 1, calls)
	require.Equal(t, "secret", password)
	require.Equal(t, "catalog-add", privileged.action)
	encoded, err := json.Marshal(privileged.request)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "secret")
}

func TestCoordinatorCreateForgejoPreflightsCatalogBeforeRepositoryMutation(t *testing.T) {
	coordinator := testCoordinator(t).coordinator
	var calls int
	var password string
	server := configureForgejo(t, &coordinator, &calls, &password)
	defer server.Close()
	require.NoError(t, coordinator.Catalog.Add(CatalogEntry{ID: "site", DisplayName: "Site", CanonicalURL: "https://git.example.test/alice/site.git"}))
	_, err := coordinator.Execute(context.Background(), "alice", "create-forgejo", strings.NewReader(`{"id":"site","display_name":"Site","password":"secret"}`))
	require.ErrorContains(t, err, "already exists")
	require.Zero(t, calls, "Forgejo must not be called when the catalog ID already exists")
	require.Empty(t, password)
}

func TestCoordinatorReportsCreatedRepositoryWhenCatalogPublicationFails(t *testing.T) {
	fixture := testCoordinator(t)
	coordinator, privileged := fixture.coordinator, fixture.privileged
	var calls int
	var password string
	server := configureForgejo(t, &coordinator, &calls, &password)
	defer server.Close()
	privilegedError := errors.New("catalog filesystem is unavailable")
	privileged.err = privilegedError

	_, err := coordinator.Execute(context.Background(), "alice", "create-forgejo", strings.NewReader(`{"id":"site","display_name":"Site","password":"secret"}`))
	require.ErrorIs(t, err, privilegedError)
	require.ErrorContains(t, err, "repository was created at https://forgejo.example.test/alice/site.git")
	require.Equal(t, 1, calls)
	require.Equal(t, "secret", password)
}

func TestCoordinatorSetupReturnsCompleteWorkspaceWithoutCloneOrKeyPreflight(t *testing.T) {
	fixture := testCoordinator(t)
	coordinator, platform := fixture.coordinator, fixture.platform
	privileged, cloner := fixture.privileged, fixture.cloner
	entry := CatalogEntry{ID: "site", DisplayName: "Site", CanonicalURL: "https://git.example.test/site.git"}
	require.NoError(t, coordinator.Catalog.Add(entry))
	workspace, err := platform.CreateWorkspace(context.Background(), platform.accounts["alice"], entry.ID)
	require.NoError(t, err)
	platform.ready[workspace.Username+":"+entry.ID] = true
	platform.keys = nil
	privileged.workspaceUsername = workspace.Username

	response, err := coordinator.Execute(context.Background(), "alice", "setup", strings.NewReader(`{"id":"site","git_username":"","git_password":""}`))
	require.NoError(t, err)
	require.Equal(t, MutationResponse{OK: true, WorkspaceUsername: workspace.Username}, response)
	require.Empty(t, cloner.remote)
	require.Equal(t, "workspace-publish", privileged.action)
	require.Equal(t, HelperWorkspaceRequest{ID: entry.ID, CanonicalURL: entry.CanonicalURL}, privileged.request)
	require.Empty(t, platform.calls.reset)
}

func TestCoordinatorRejectsUnknownActionsAndFields(t *testing.T) {
	coordinator := testCoordinator(t).coordinator
	_, err := coordinator.Execute(context.Background(), "alice", "shell", strings.NewReader(`{}`))
	require.ErrorContains(t, err, "unsupported")
	_, err = coordinator.Execute(context.Background(), "alice", "remove", strings.NewReader(`{"id":"site","command":"rm"}`))
	require.ErrorContains(t, err, "unknown field")
}
