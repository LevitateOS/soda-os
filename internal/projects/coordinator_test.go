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
	action             string
	actions            []string
	request            any
	err                error
	workspaceUsername  string
	workspacePublicKey string
	publishStarted     chan struct{}
	publishRelease     <-chan struct{}
}

func (privileged *fakePrivileged) record(action string, request any) error {
	privileged.action, privileged.request = action, request
	privileged.actions = append(privileged.actions, action)
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
	if privileged.publishStarted != nil {
		close(privileged.publishStarted)
		<-privileged.publishRelease
	}
	username := privileged.workspaceUsername
	if username == "" {
		username = "soda-w-example"
	}
	return MutationResponse{OK: err == nil, WorkspaceUsername: username}, err
}

func (privileged *fakePrivileged) WorkspacePrepare(_ context.Context, request HelperWorkspaceRequest) (MutationResponse, error) {
	err := privileged.record("workspace-prepare", request)
	username := privileged.workspaceUsername
	if username == "" {
		username = "soda-w-example"
	}
	return MutationResponse{OK: err == nil, WorkspaceUsername: username, WorkspacePublicKey: privileged.workspacePublicKey}, err
}

func (privileged *fakePrivileged) WorkspaceRemove(_ context.Context, request ProjectRequest) error {
	return privileged.record("workspace-remove", request)
}

func (privileged *fakePrivileged) ToolsInstall(_ context.Context, request HelperToolRequest) error {
	return privileged.record("tools-install", request)
}

func (privileged *fakePrivileged) ProjectRemove(_ context.Context, request ProjectRequest) error {
	return privileged.record("project-remove", request)
}

func (privileged *fakePrivileged) HumanDelete(_ context.Context, request HelperHumanRequest) error {
	return privileged.record("human-delete", request)
}

func (privileged *fakePrivileged) HumanCreate(_ context.Context, request HelperHumanCreateRequest) error {
	return privileged.record("human-create", request)
}

func (privileged *fakePrivileged) HumanPublish(_ context.Context, request HelperHumanPublishRequest) error {
	return privileged.record("human-publish", request)
}

type coordinatorFixture struct {
	coordinator Coordinator
	platform    *fakePlatform
	privileged  *fakePrivileged
}

func testCoordinator(t *testing.T) coordinatorFixture {
	t.Helper()
	catalog := testCatalog(t)
	platform := newFakePlatform()
	platform.accounts["alice"] = primaryAccount("alice", primaryRoleAdministrator)
	privileged := &fakePrivileged{workspacePublicKey: strings.TrimSpace(string(testAuthorizedKey(t)))}
	lifecycle := Lifecycle{Catalog: catalog, Platform: platform}
	return coordinatorFixture{
		coordinator: Coordinator{Catalog: catalog, Lifecycle: lifecycle, Platform: platform, Privileged: privileged, Forgejo: ForgejoClient{}, Endpoints: fakeEndpoints{}},
		platform:    platform,
		privileged:  privileged,
	}
}

func TestCoordinatorAddPersonRegistersForgejoKeyWithoutPrivilegedSecret(t *testing.T) {
	fixture := testCoordinator(t)
	key := strings.TrimSpace(string(testAuthorizedKey(t)))
	var registered string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		username, password, ok := request.BasicAuth()
		require.True(t, ok)
		require.Equal(t, "bob", username)
		require.Equal(t, "initial secret", password)
		switch request.URL.Path {
		case "/api/v1/user":
			_, _ = writer.Write([]byte(`{"login":"bob"}`))
		case "/api/v1/user/keys":
			if request.Method == http.MethodGet {
				_, _ = writer.Write([]byte(`[]`))
				return
			}
			var body struct {
				Key string `json:"key"`
			}
			require.NoError(t, json.NewDecoder(request.Body).Decode(&body))
			registered = body.Key
			writer.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected Forgejo request %s %s", request.Method, request.URL.Path)
		}
	}))
	defer server.Close()
	fixture.coordinator.Endpoints = fakeEndpoints{forgejoURL: server.URL}
	response, err := fixture.coordinator.Execute(context.Background(), "alice", "add-person", strings.NewReader(
		`{"username":"bob","password":"initial secret","authorized_key":`+fmt.Sprintf("%q", key)+`}`,
	))
	require.NoError(t, err)
	require.Equal(t, MutationResponse{OK: true}, response)
	require.Equal(t, []string{"human-create", "human-publish"}, fixture.privileged.actions)
	require.Equal(t, key, registered)
	encoded, err := json.Marshal(fixture.privileged.request)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "initial secret")
	require.NotContains(t, string(encoded), "token")
}

func TestCoordinatorAddPersonRetainsLinuxStateWhenForgejoRegistrationFails(t *testing.T) {
	fixture := testCoordinator(t)
	key := strings.TrimSpace(string(testAuthorizedKey(t)))
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte(`{"message":"wrong password"}`))
	}))
	defer server.Close()
	fixture.coordinator.Endpoints = fakeEndpoints{forgejoURL: server.URL}
	_, err := fixture.coordinator.Execute(context.Background(), "alice", "add-person", strings.NewReader(
		`{"username":"bob","password":"initial secret","authorized_key":`+fmt.Sprintf("%q", key)+`}`,
	))
	require.ErrorContains(t, err, "Linux account bob and its SSH key were retained")
	require.Equal(t, []string{"human-create", "human-publish"}, fixture.privileged.actions)
}

func TestCoordinatorAddPersonRequiresAdministratorBeforeMutation(t *testing.T) {
	fixture := testCoordinator(t)
	fixture.platform.accounts["alice"] = primaryAccount("alice", primaryRoleUser)
	_, err := fixture.coordinator.Execute(context.Background(), "alice", "add-person", strings.NewReader(
		`{"username":"bob","password":"initial secret","authorized_key":"bad"}`,
	))
	require.ErrorContains(t, err, "administrator")
	require.Empty(t, fixture.privileged.actions)
}

func configureForgejo(t *testing.T, coordinator *Coordinator, calls *int, password *string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		*calls = *calls + 1
		_, receivedPassword, _ := request.BasicAuth()
		*password = receivedPassword
		writer.WriteHeader(http.StatusCreated)
		_, _ = writer.Write([]byte(`{"name":"site","ssh_url":"git@forgejo.example.test:alice/site.git","empty":true,"owner":{"login":"alice"}}`))
	}))
	coordinator.Endpoints = fakeEndpoints{forgejoURL: server.URL}
	coordinator.Forgejo = ForgejoClient{}
	return server
}

func TestCoordinatorListContract(t *testing.T) {
	coordinator := testCoordinator(t).coordinator
	require.NoError(t, coordinator.Catalog.Add(CatalogEntry{ID: "site", DisplayName: "Site", CanonicalURL: "git@git.example.test:site.git"}))
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

func TestCoordinatorSetupRegistersWorkspacePublicKeyBeforeNativeSSHClone(t *testing.T) {
	fixture := testCoordinator(t)
	coordinator, privileged := fixture.coordinator, fixture.privileged
	entry := CatalogEntry{ID: "site", DisplayName: "Site", CanonicalURL: "git@git.example.test:team/site.git"}
	require.NoError(t, coordinator.Catalog.Add(entry))
	var registered, title string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		username, password, ok := request.BasicAuth()
		require.True(t, ok)
		require.Equal(t, "alice", username)
		require.Equal(t, "one-use", password)
		switch request.URL.Path {
		case "/api/v1/user":
			_, _ = writer.Write([]byte(`{"login":"alice"}`))
		case "/api/v1/user/keys":
			if request.Method == http.MethodGet {
				_, _ = writer.Write([]byte(`[]`))
				return
			}
			var body struct {
				Key   string `json:"key"`
				Title string `json:"title"`
			}
			require.NoError(t, json.NewDecoder(request.Body).Decode(&body))
			registered = body.Key
			title = body.Title
			writer.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected Forgejo request %s %s", request.Method, request.URL.Path)
		}
	}))
	defer server.Close()
	coordinator.Endpoints = fakeEndpoints{forgejoURL: server.URL}
	response, err := coordinator.Execute(context.Background(), "alice", "setup", strings.NewReader(`{"id":"site","forgejo_password":"one-use"}`))
	require.NoError(t, err)
	require.Equal(t, MutationResponse{OK: true, WorkspaceUsername: "soda-w-example"}, response)
	require.Equal(t, []string{"workspace-prepare", "workspace-publish"}, privileged.actions)
	require.Equal(t, privileged.workspacePublicKey, registered)
	require.Equal(t, "Soda OS workspace soda-w-example", title)
	encoded, err := json.Marshal(privileged.request)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "one-use")
}

func TestCoordinatorSetupRetainsWorkspaceKeyWhenForgejoRegistrationFails(t *testing.T) {
	fixture := testCoordinator(t)
	coordinator := fixture.coordinator
	require.NoError(t, coordinator.Catalog.Add(CatalogEntry{ID: "site", DisplayName: "Site", CanonicalURL: "git@git.example.test:site.git"}))
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte(`{"message":"wrong password"}`))
	}))
	defer server.Close()
	coordinator.Endpoints = fakeEndpoints{forgejoURL: server.URL}

	_, err := coordinator.Execute(context.Background(), "alice", "setup", strings.NewReader(`{"id":"site","forgejo_password":"one-use"}`))
	require.ErrorContains(t, err, "local outbound Git key were retained; Forgejo key registration can be retried")
	require.Equal(t, []string{"workspace-prepare"}, fixture.privileged.actions)
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
	require.NoError(t, coordinator.Catalog.Add(CatalogEntry{ID: "site", DisplayName: "Site", CanonicalURL: "git@git.example.test:alice/site.git"}))
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
	require.ErrorContains(t, err, "repository was created at git@forgejo.example.test:alice/site.git")
	require.Equal(t, 1, calls)
	require.Equal(t, "secret", password)
}

func TestCoordinatorRejectsUnknownActionsAndFields(t *testing.T) {
	coordinator := testCoordinator(t).coordinator
	_, err := coordinator.Execute(context.Background(), "alice", "shell", strings.NewReader(`{}`))
	require.ErrorContains(t, err, "unsupported")
	_, err = coordinator.Execute(context.Background(), "alice", "remove", strings.NewReader(`{"id":"site","command":"rm"}`))
	require.ErrorContains(t, err, "unknown field")
}

func TestCoordinatorRoutesOwnWorkspaceRemoval(t *testing.T) {
	fixture := testCoordinator(t)
	response, err := fixture.coordinator.Execute(context.Background(), "alice", "remove-workspace", strings.NewReader(`{"id":"site"}`))
	require.NoError(t, err)
	require.Equal(t, MutationResponse{OK: true}, response)
	require.Equal(t, "workspace-remove", fixture.privileged.action)
	require.Equal(t, ProjectRequest{ID: "site"}, fixture.privileged.request)
}

func TestCoordinatorRoutesArbitraryMiseSelectionsByEstablishedScope(t *testing.T) {
	fixture := testCoordinator(t)
	response, err := fixture.coordinator.Execute(context.Background(), "alice", "install-tools", strings.NewReader(
		`{"id":"site","scope":"project","tools":["aqua:BurntSushi/ripgrep@latest","python@3.13"]}`,
	))
	require.NoError(t, err)
	require.Equal(t, MutationResponse{OK: true}, response)
	require.Equal(t, "tools-install", fixture.privileged.action)
	require.Equal(t, HelperToolRequest{ID: "site", Scope: "project", Tools: []string{"aqua:BurntSushi/ripgrep@latest", "python@3.13"}}, fixture.privileged.request)
}
