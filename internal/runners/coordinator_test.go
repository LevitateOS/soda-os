package runners

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/stretchr/testify/require"
)

type fakeAuthorizer struct{ err error }

func (authorizer fakeAuthorizer) RequireAdministrator(context.Context, linuxhost.PKExecIdentity) error {
	return authorizer.err
}

var testAdministrator = linuxhost.PKExecIdentity{Username: "alice", UID: 1000}

type fakeLocal struct{ views []RunnerView }

func (local fakeLocal) List(context.Context) ([]RunnerView, error) { return local.views, nil }

type fakePrivileged struct {
	action  string
	create  CreateRequest
	request RunnerRequest
}

func (privileged *fakePrivileged) Create(_ context.Context, request CreateRequest) error {
	privileged.action, privileged.create = "create", request
	return nil
}
func (privileged *fakePrivileged) Start(_ context.Context, request RunnerRequest) error {
	privileged.action, privileged.request = "start", request
	return nil
}
func (privileged *fakePrivileged) Stop(_ context.Context, request RunnerRequest) error {
	privileged.action, privileged.request = "stop", request
	return nil
}
func (privileged *fakePrivileged) Restart(_ context.Context, request RunnerRequest) error {
	privileged.action, privileged.request = "restart", request
	return nil
}
func (privileged *fakePrivileged) Remove(_ context.Context, request RunnerRequest) error {
	privileged.action, privileged.request = "remove", request
	return nil
}

func TestCoordinatorRequiresAdministratorBeforeReadingInputOrState(t *testing.T) {
	privileged := &fakePrivileged{}
	coordinator := Coordinator{Authorizer: fakeAuthorizer{err: errors.New("administrator status is required")}, Local: fakeLocal{}, Privileged: privileged}
	_, err := coordinator.Execute(context.Background(), testAdministrator, "create", strings.NewReader(`not-json`))
	require.ErrorContains(t, err, "administrator")
	require.Empty(t, privileged.action)
}

func TestCoordinatorUsesOnlyTheBundledForgejoEndpoint(t *testing.T) {
	privileged := &fakePrivileged{}
	coordinator := Coordinator{
		Authorizer: fakeAuthorizer{}, Local: fakeLocal{}, Privileged: privileged,
	}
	response, err := coordinator.Execute(context.Background(), testAdministrator, "create", strings.NewReader(`{
		"id":"forgejo-one","provider":"forgejo","registration_url":"https://external.invalid",
		"registration_id":"33834eef-e758-48c4-a676-1745426747aa",
		"labels":"soda-arm64:host","registration_token":"provider-input"
	}`))
	require.NoError(t, err)
	require.Equal(t, MutationResponse{OK: true}, response)
	require.Equal(t, BundledForgejoURL, privileged.create.RegistrationURL)
}

func TestCoordinatorReportsExactLocalListenerAndCapacityCounts(t *testing.T) {
	coordinator := Coordinator{Authorizer: fakeAuthorizer{}, Local: fakeLocal{views: []RunnerView{
		{Descriptor: Descriptor{ID: "one"}, Capacity: 1, Service: ServiceState{Active: "active", Sub: "running"}},
		{Descriptor: Descriptor{ID: "two"}, Capacity: 1, Service: ServiceState{Active: "failed", Sub: "failed"}},
	}}}
	response, err := coordinator.Execute(context.Background(), testAdministrator, "list", strings.NewReader(`{}`))
	require.NoError(t, err)
	require.Equal(t, 2, response.(ListResponse).RunnerCount)
	require.Equal(t, 1, response.(ListResponse).ActiveListeners)
	require.Equal(t, 2, response.(ListResponse).TotalCapacity)
}

func TestCoordinatorRetainsStrictRequestDecoding(t *testing.T) {
	coordinator := Coordinator{Authorizer: fakeAuthorizer{}, Local: fakeLocal{}, Privileged: &fakePrivileged{}}
	_, err := coordinator.Execute(context.Background(), testAdministrator, "start", strings.NewReader(`{"id":"one","id":"two"}`))
	require.ErrorContains(t, err, "duplicate")
	_, err = coordinator.Execute(context.Background(), testAdministrator, "start", strings.NewReader(`{"id":"one","project_id":"site"}`))
	require.ErrorContains(t, err, "unknown field")
}
