package runners

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakeAuthorizer struct{ err error }

func (authorizer fakeAuthorizer) RequireAdministrator(context.Context, string) error {
	return authorizer.err
}

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

type fakeEndpoints struct{ url string }

func (endpoints fakeEndpoints) Endpoints(context.Context) (string, string, error) {
	return endpoints.url, "", nil
}

func TestCoordinatorRequiresAdministratorBeforeReadingInputOrState(t *testing.T) {
	privileged := &fakePrivileged{}
	coordinator := Coordinator{Authorizer: fakeAuthorizer{err: errors.New("administrator status is required")}, Local: fakeLocal{}, Privileged: privileged}
	_, err := coordinator.Execute(context.Background(), "alice", "create", strings.NewReader(`not-json`))
	require.ErrorContains(t, err, "administrator")
	require.Empty(t, privileged.action)
}

func TestCoordinatorUsesOnlyTheBundledForgejoEndpoint(t *testing.T) {
	privileged := &fakePrivileged{}
	coordinator := Coordinator{
		Authorizer: fakeAuthorizer{}, Local: fakeLocal{}, Privileged: privileged,
		Endpoints: fakeEndpoints{url: "http://soda.tail.example:30000"},
	}
	response, err := coordinator.Execute(context.Background(), "alice", "create", strings.NewReader(`{
		"id":"forgejo-one","provider":"forgejo","registration_url":"https://external.invalid",
		"registration_id":"33834eef-e758-48c4-a676-1745426747aa",
		"labels":"soda-arm64:host","registration_token":"provider-input"
	}`))
	require.NoError(t, err)
	require.Equal(t, MutationResponse{OK: true}, response)
	require.Equal(t, "http://soda.tail.example:30000", privileged.create.RegistrationURL)
}

func TestCoordinatorReportsExactLocalListenerAndCapacityCounts(t *testing.T) {
	coordinator := Coordinator{Authorizer: fakeAuthorizer{}, Endpoints: fakeEndpoints{url: "http://soda.example.test:30000"}, Local: fakeLocal{views: []RunnerView{
		{Descriptor: Descriptor{ID: "one"}, Capacity: 1, Service: ServiceState{Active: "active", Sub: "running"}},
		{Descriptor: Descriptor{ID: "two"}, Capacity: 1, Service: ServiceState{Active: "failed", Sub: "failed"}},
	}}}
	response, err := coordinator.Execute(context.Background(), "alice", "list", strings.NewReader(`{}`))
	require.NoError(t, err)
	require.Equal(t, 2, response.(ListResponse).RunnerCount)
	require.Equal(t, 1, response.(ListResponse).ActiveListeners)
	require.Equal(t, 2, response.(ListResponse).TotalCapacity)
	require.Equal(t, "http://soda.example.test:30000", response.(ListResponse).ForgejoURL)
}
