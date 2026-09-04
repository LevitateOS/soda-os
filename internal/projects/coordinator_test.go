package projects

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakePrivileged struct {
	action             string
	actions            []string
	request            any
	err                error
	publishErr         error
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

func (privileged *fakePrivileged) CatalogEdit(_ context.Context, request HelperEditRequest) error {
	return privileged.record("catalog-edit", request)
}

func (privileged *fakePrivileged) WorkspacePublish(_ context.Context, request HelperWorkspaceRequest) (MutationResponse, error) {
	err := privileged.record("workspace-publish", request)
	if err == nil {
		err = privileged.publishErr
	}
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

func (privileged *fakePrivileged) ProjectRemove(_ context.Context, request ProjectRequest) error {
	return privileged.record("project-remove", request)
}

func (privileged *fakePrivileged) HumanDelete(_ context.Context, request HelperHumanRequest) error {
	return privileged.record("human-delete", request)
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
	lifecycle := Lifecycle{Catalog: catalog, Host: platform, Platform: platform}
	return coordinatorFixture{
		coordinator: Coordinator{Catalog: catalog, Lifecycle: lifecycle, Platform: platform, Privileged: privileged},
		platform:    platform,
		privileged:  privileged,
	}
}

func TestCoordinatorPublishesArbitraryCatalogMetadataAtPrivilegedBoundary(t *testing.T) {
	fixture := testCoordinator(t)
	response, err := fixture.coordinator.Execute(context.Background(), "alice", "add-existing", strings.NewReader(
		`{"id":"site","display_name":"Site","canonical_url":"git@git.example.test:site.git","team":"web","labels":["public"]}`,
	))
	require.NoError(t, err)
	require.Equal(t, "catalog-add", fixture.privileged.action)
	request := fixture.privileged.request.(CatalogMutationRequest)
	require.JSONEq(t, `"web"`, string(request.Additional["team"]))
	require.JSONEq(t, `["public"]`, string(request.Additional["labels"]))
	project := response.(MutationResponse).Project
	require.NotNil(t, project)
	require.JSONEq(t, `"web"`, string(project.Additional["team"]))
}

func TestCoordinatorEditPublishesMetadataWithoutCanonicalURL(t *testing.T) {
	fixture := testCoordinator(t)
	require.NoError(t, fixture.coordinator.Catalog.Add(CatalogEntry{
		ID: "site", DisplayName: "Site", CanonicalURL: "git@git.example.test:site.git",
		Additional: map[string]json.RawMessage{"owner": json.RawMessage(`"old-owner"`)},
	}))
	response, err := fixture.coordinator.Execute(context.Background(), "alice", "edit", strings.NewReader(
		`{"id":"site","display_name":"Renamed","owner":"new-owner","labels":["public"]}`,
	))
	require.NoError(t, err)
	require.Equal(t, "catalog-edit", fixture.privileged.action)
	request := fixture.privileged.request.(EditRequest)
	require.JSONEq(t, `"new-owner"`, string(request.Additional["owner"]))
	require.JSONEq(t, `["public"]`, string(request.Additional["labels"]))
	encodedRequest, err := json.Marshal(request)
	require.NoError(t, err)
	require.JSONEq(t, `{"id":"site","display_name":"Renamed","owner":"new-owner","labels":["public"]}`, string(encodedRequest))

	project := response.(MutationResponse).Project
	require.NotNil(t, project)
	require.Equal(t, "Renamed", project.DisplayName)
	require.Equal(t, "git@git.example.test:site.git", project.CanonicalURL)
	require.JSONEq(t, `"new-owner"`, string(project.Additional["owner"]))
}

func TestCoordinatorEditRejectsEveryCanonicalURLBeforePrivilege(t *testing.T) {
	for name, canonicalURL := range map[string]string{
		"unchanged": "git@git.example.test:site.git",
		"changed":   "git@git.example.test:other.git",
	} {
		t.Run(name, func(t *testing.T) {
			fixture := testCoordinator(t)
			require.NoError(t, fixture.coordinator.Catalog.Add(CatalogEntry{
				ID: "site", DisplayName: "Site", CanonicalURL: "git@git.example.test:site.git",
			}))
			input := `{"id":"site","display_name":"Renamed","canonical_url":"` + canonicalURL + `"}`

			_, err := fixture.coordinator.Execute(context.Background(), "alice", "edit", strings.NewReader(input))

			require.ErrorContains(t, err, `must not include "canonical_url"`)
			require.Empty(t, fixture.privileged.actions)
		})
	}
}

func TestCoordinatorListHasNoEndpointDependency(t *testing.T) {
	coordinator := testCoordinator(t).coordinator
	require.NoError(t, coordinator.Catalog.Add(CatalogEntry{
		ID: "site", DisplayName: "Site", CanonicalURL: "git@git.example.test:site.git",
		Additional: map[string]json.RawMessage{
			"team": json.RawMessage(`"web"`), "catalog_metadata": json.RawMessage(`"catalog-value"`),
			"workspace_username": json.RawMessage(`"catalog-user"`), "workspace_exists": json.RawMessage(`"catalog-exists"`),
		},
	}))
	response, err := coordinator.Execute(context.Background(), "alice", "list", strings.NewReader(`{}`))
	require.NoError(t, err)
	list := response.(ListResponse)
	require.Equal(t, CurrentUserView{Username: "alice", Administrator: true}, list.CurrentUser)
	require.Len(t, list.Projects, 1)
	require.NotEmpty(t, list.Projects[0].WorkspaceUsername)
	require.JSONEq(t, `"web"`, string(list.Projects[0].Additional["team"]))
	encoded, err := json.Marshal(list)
	require.NoError(t, err)
	var wire struct {
		Projects []struct {
			CatalogMetadata   map[string]json.RawMessage `json:"catalog_metadata"`
			WorkspaceUsername string                     `json:"workspace_username"`
			WorkspaceExists   bool                       `json:"workspace_exists"`
		} `json:"projects"`
	}
	require.NoError(t, json.Unmarshal(encoded, &wire))
	require.Len(t, wire.Projects, 1)
	require.JSONEq(t, `"catalog-value"`, string(wire.Projects[0].CatalogMetadata["catalog_metadata"]))
	require.JSONEq(t, `"catalog-user"`, string(wire.Projects[0].CatalogMetadata["workspace_username"]))
	require.JSONEq(t, `"catalog-exists"`, string(wire.Projects[0].CatalogMetadata["workspace_exists"]))
	require.Equal(t, list.Projects[0].WorkspaceUsername, wire.Projects[0].WorkspaceUsername)
	require.Equal(t, list.Projects[0].WorkspaceExists, wire.Projects[0].WorkspaceExists)
	var object map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(encoded, &object))
	require.NotContains(t, object, "forgejo_url")
	require.NotContains(t, object, "ssh_host")
}

func TestCoordinatorListReportsValidatedWorkspaceAccountExistence(t *testing.T) {
	fixture := testCoordinator(t)
	entry := CatalogEntry{ID: "site", DisplayName: "Site", CanonicalURL: "git@git.example.test:site.git"}
	require.NoError(t, fixture.coordinator.Catalog.Add(entry))
	workspace, err := fixture.platform.CreateWorkspace(context.Background(), fixture.platform.accounts["alice"], entry.ID)
	require.NoError(t, err)
	require.False(t, fixture.platform.ready[workspace.Username+":"+entry.ID], "clone completion must remain independent")

	response, err := fixture.coordinator.Execute(context.Background(), "alice", "list", strings.NewReader(`{}`))

	require.NoError(t, err)
	project := response.(ListResponse).Projects[0]
	require.Equal(t, workspace.Username, project.WorkspaceUsername)
	require.True(t, project.WorkspaceExists)
}

func TestCoordinatorSetupHasNoEndpointDependencyAndAttemptsNativeSSHClone(t *testing.T) {
	fixture := testCoordinator(t)
	coordinator, privileged := fixture.coordinator, fixture.privileged
	entry := CatalogEntry{ID: "site", DisplayName: "Site", CanonicalURL: "git@localhost:team/site.git"}
	require.NoError(t, coordinator.Catalog.Add(entry))
	response, err := coordinator.Execute(context.Background(), "alice", "setup", strings.NewReader(`{"id":"site"}`))
	require.NoError(t, err)
	require.Equal(t, MutationResponse{OK: true, WorkspaceUsername: "soda-w-example"}, response)
	require.Equal(t, []string{"workspace-prepare", "workspace-publish"}, privileged.actions)
	encoded, err := json.Marshal(privileged.request)
	require.NoError(t, err)
	require.JSONEq(t, `{"id":"site","canonical_url":"git@localhost:team/site.git"}`, string(encoded))
}

func TestCoordinatorSetupLeavesKeyRegistrationToEveryAuthoritativeGitHost(t *testing.T) {
	for _, remote := range []string{
		"git@soda.example.ts.net:site.git",
		"ssh://git@git.example.test/team/site.git",
	} {
		t.Run(remote, func(t *testing.T) {
			fixture := testCoordinator(t)
			require.NoError(t, fixture.coordinator.Catalog.Add(CatalogEntry{
				ID: "site", DisplayName: "Site", CanonicalURL: remote,
			}))
			fixture.privileged.publishErr = errors.New("native SSH authentication failed")

			_, err := fixture.coordinator.Execute(context.Background(), "alice", "setup", strings.NewReader(`{"id":"site"}`))

			require.ErrorContains(t, err, "If repository authorization caused the clone failure")
			require.ErrorContains(t, err, "authoritative Git host")
			require.ErrorContains(t, err, fixture.privileged.workspacePublicKey)
			require.ErrorContains(t, err, "register that key")
			require.ErrorContains(t, err, "retry setup")
			require.ErrorContains(t, err, "native SSH authentication failed")
			require.Equal(t, []string{"workspace-prepare", "workspace-publish"}, fixture.privileged.actions)
		})
	}
}

func TestCoordinatorSetupRejectsLegacyForgejoPassword(t *testing.T) {
	fixture := testCoordinator(t)
	_, err := fixture.coordinator.Execute(context.Background(), "alice", "setup", strings.NewReader(`{"id":"site","forgejo_password":"secret"}`))
	require.ErrorContains(t, err, "unknown field")
	require.Empty(t, fixture.privileged.actions)
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
