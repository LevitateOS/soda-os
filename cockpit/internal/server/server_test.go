package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/LevitateOS/soda-os/cockpit/internal/soda"
)

type fakeAuth struct{ err error }

func (a fakeAuth) Authenticate(_, _ string) error { return a.err }

type fakeAPI struct {
	people       []soda.Person
	projects     []soda.Project
	worktrees    []soda.Worktree
	jobs         []soda.ProvisioningJob
	toolchain    *soda.ToolchainInstallation
	statuses     []soda.WorktreeStatus
	active       []soda.ActiveSSHConnection
	events       <-chan soda.Event
	created      *soda.CreatePersonRequest
	retried      bool
	eventProject string
}

func (f *fakeAPI) People(context.Context) ([]soda.Person, error)    { return f.people, nil }
func (f *fakeAPI) Projects(context.Context) ([]soda.Project, error) { return f.projects, nil }
func (f *fakeAPI) ProjectsForPerson(context.Context, string) ([]soda.Project, error) {
	return f.projects, nil
}
func (f *fakeAPI) CreatePerson(_ context.Context, request soda.CreatePersonRequest) (soda.Person, error) {
	f.created = &request
	return soda.Person{Username: request.Username}, nil
}
func (f *fakeAPI) CreateProject(context.Context, soda.CreateProjectRequest) (soda.Project, error) {
	return soda.Project{ID: "project-1"}, nil
}
func (f *fakeAPI) AddCollaborator(context.Context, string, string) (soda.Worktree, error) {
	return soda.Worktree{}, nil
}
func (f *fakeAPI) CreateWorktree(context.Context, string, string, string, string) (soda.Worktree, error) {
	return soda.Worktree{}, nil
}
func (f *fakeAPI) Worktrees(context.Context, string) ([]soda.Worktree, error) {
	return f.worktrees, nil
}
func (f *fakeAPI) Jobs(context.Context, string) ([]soda.ProvisioningJob, error) {
	return f.jobs, nil
}
func (f *fakeAPI) RetryProvisioning(context.Context, string) (soda.ProvisioningJob, error) {
	f.retried = true
	job := soda.ProvisioningJob{ID: "job-new", ProjectID: "project-1", State: "installing"}
	f.jobs = append([]soda.ProvisioningJob{job}, f.jobs...)
	return job, nil
}
func (f *fakeAPI) Toolchain(context.Context, string) (*soda.ToolchainInstallation, error) {
	return f.toolchain, nil
}
func (f *fakeAPI) DeployKey(context.Context, string) (soda.DeployKey, error) {
	return soda.DeployKey{}, nil
}
func (f *fakeAPI) HostStatus(context.Context) (soda.HostStatus, error) {
	return soda.HostStatus{Overall: "ready"}, nil
}
func (f *fakeAPI) WorktreeStatuses(context.Context, string) ([]soda.WorktreeStatus, error) {
	return f.statuses, nil
}
func (f *fakeAPI) ActiveSessions(context.Context) ([]soda.ActiveSSHConnection, error) {
	return f.active, nil
}
func (f *fakeAPI) Events(_ context.Context, projectID string) (<-chan soda.Event, error) {
	f.eventProject = projectID
	if f.events != nil {
		return f.events, nil
	}
	events := make(chan soda.Event)
	return events, nil
}

func TestHealthAndLoginPageArePublic(t *testing.T) {
	app := testServer(t, &fakeAPI{}, fakeAuth{})
	for path, expected := range map[string]string{"/healthz": "ok\n", "/login": "Sign in to Soda OS"} {
		response := request(app, http.MethodGet, path, "", nil)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), expected) {
			t.Fatalf("unexpected %s response: %d %q", path, response.Code, response.Body.String())
		}
	}
}

func TestProtectedPageRedirectsToLogin(t *testing.T) {
	app := testServer(t, &fakeAPI{}, fakeAuth{})
	response := request(app, http.MethodGet, "/", "", nil)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/login" {
		t.Fatalf("unexpected response: %d %q", response.Code, response.Header().Get("Location"))
	}
}

func TestPAMLoginCreatesSessionForRegisteredPerson(t *testing.T) {
	api := &fakeAPI{people: []soda.Person{{ID: "person-1", Username: "alice", DisplayName: "Alice", Role: soda.RoleDeveloper}}}
	app := testServer(t, api, fakeAuth{})
	form := url.Values{"username": {"alice"}, "password": {"correct"}}.Encode()
	login := request(app, http.MethodPost, "/login", form, nil)
	if login.Code != http.StatusSeeOther {
		t.Fatalf("unexpected login response: %d %q", login.Code, login.Body.String())
	}
	cookies := login.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].Secure || !cookies[0].HttpOnly {
		t.Fatalf("expected secure session cookie, got %#v", cookies)
	}
	home := request(app, http.MethodGet, "/", "", cookies[0])
	if home.Code != http.StatusOK || !strings.Contains(home.Body.String(), "Alice") {
		t.Fatalf("unexpected home response: %d %q", home.Code, home.Body.String())
	}
	people := request(app, http.MethodGet, "/people", "", cookies[0])
	if people.Code != http.StatusForbidden {
		t.Fatalf("developer accessed people page: %d", people.Code)
	}
}

func TestFailedAuthenticationDoesNotCreateSession(t *testing.T) {
	app := testServer(t, &fakeAPI{}, fakeAuth{err: errors.New("denied")})
	form := url.Values{"username": {"alice"}, "password": {"wrong"}}.Encode()
	response := request(app, http.MethodPost, "/login", form, nil)
	if response.Code != http.StatusUnauthorized || !strings.Contains(response.Body.String(), "Invalid username or password") {
		t.Fatalf("unexpected response: %d %q", response.Code, response.Body.String())
	}
}

func TestProvisioningFragmentUsesEventsWhileInstalling(t *testing.T) {
	admin := soda.Person{ID: "admin-1", Username: "admin", DisplayName: "Admin", Role: soda.RoleAdmin}
	project := soda.Project{ID: "project-1", Name: "Live project"}
	api := &fakeAPI{
		people:   []soda.Person{admin},
		projects: []soda.Project{project},
		jobs:     []soda.ProvisioningJob{{ID: "job-1", ProjectID: project.ID, State: "installing"}},
	}
	app := testServer(t, api, fakeAuth{})
	token, err := app.sessions.create(admin)
	if err != nil {
		t.Fatal(err)
	}
	cookie := &http.Cookie{Name: sessionCookie, Value: token}

	installing := request(app, http.MethodGet, "/projects/project-1/provisioning", "", cookie)
	if installing.Code != http.StatusOK || !strings.Contains(installing.Body.String(), `sse:provisioning_changed`) ||
		strings.Contains(installing.Body.String(), `every 2s`) ||
		!strings.Contains(installing.Body.String(), `disabled>Provisioning…`) {
		t.Fatalf("expected live installing fragment, got %d %q", installing.Code, installing.Body.String())
	}

	api.jobs[0].State = "ready"
	ready := request(app, http.MethodGet, "/projects/project-1/provisioning", "", cookie)
	if ready.Code != http.StatusOK || !strings.Contains(ready.Body.String(), `sse:provisioning_changed`) ||
		strings.Contains(ready.Body.String(), `every 2s`) ||
		!strings.Contains(ready.Body.String(), `>Retry provisioning</button>`) {
		t.Fatalf("expected completed fragment without polling, got %d %q", ready.Code, ready.Body.String())
	}
}

func TestHTMXRetryReturnsInstallingFragment(t *testing.T) {
	admin := soda.Person{ID: "admin-1", Username: "admin", DisplayName: "Admin", Role: soda.RoleAdmin}
	project := soda.Project{ID: "project-1", Name: "Live project"}
	api := &fakeAPI{people: []soda.Person{admin}, projects: []soda.Project{project}}
	app := testServer(t, api, fakeAuth{})
	token, err := app.sessions.create(admin)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/projects/project-1/provisioning", nil)
	req.Header.Set("HX-Request", "true")
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
	response := httptest.NewRecorder()
	app.Handler().ServeHTTP(response, req)

	if !api.retried || response.Code != http.StatusOK || response.Header().Get("HX-Redirect") != "" ||
		!strings.Contains(response.Body.String(), `sse:provisioning_changed`) {
		t.Fatalf("expected HTMX retry fragment, got retried=%t status=%d headers=%v body=%q",
			api.retried, response.Code, response.Header(), response.Body.String())
	}
}

func TestAdminHTMXPersonFlow(t *testing.T) {
	admin := soda.Person{ID: "admin-1", Username: "admin", DisplayName: "Admin", Role: soda.RoleAdmin}
	api := &fakeAPI{people: []soda.Person{admin}}
	app := testServer(t, api, fakeAuth{})
	token, err := app.sessions.create(admin)
	if err != nil {
		t.Fatal(err)
	}
	form := url.Values{
		"username": {"bob"}, "display_name": {"Bob"}, "email": {"bob@example.test"},
		"role": {"developer"}, "password": {"temporary"}, "ssh_public_key": {"ssh-ed25519 test"},
	}.Encode()
	req := httptest.NewRequest(http.MethodPost, "/people", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
	response := httptest.NewRecorder()
	app.Handler().ServeHTTP(response, req)
	if response.Code != http.StatusOK || response.Header().Get("HX-Redirect") != "" ||
		!strings.Contains(response.Body.String(), "Person added.") {
		t.Fatalf("unexpected HTMX response: %d %#v", response.Code, response.Header())
	}
	if api.created == nil || api.created.Username != "bob" {
		t.Fatalf("person request was not forwarded: %#v", api.created)
	}
}

func TestProjectSSEForwardsAllowedEvents(t *testing.T) {
	developer := soda.Person{ID: "person-1", Username: "alice", DisplayName: "Alice", Role: soda.RoleDeveloper}
	project := soda.Project{ID: "project-1", Name: "Live project"}
	events := make(chan soda.Event, 1)
	projectID := project.ID
	events <- soda.Event{Kind: "git_changed", ProjectID: &projectID, Sequence: 2}
	api := &fakeAPI{people: []soda.Person{developer}, projects: []soda.Project{project}, events: events}
	app := testServer(t, api, fakeAuth{})
	token, err := app.sessions.create(developer)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/events?project_id=project-1", nil).WithContext(ctx)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		app.Handler().ServeHTTP(response, req)
		close(done)
	}()
	time.Sleep(25 * time.Millisecond)
	cancel()
	<-done
	if api.eventProject != project.ID || !strings.Contains(response.Body.String(), "event: git_changed") {
		t.Fatalf("project event was not streamed: project=%q body=%q", api.eventProject, response.Body.String())
	}
}

func TestSessionFragmentRedactsClientForDevelopers(t *testing.T) {
	developer := soda.Person{ID: "person-1", Username: "alice", DisplayName: "Alice", Role: soda.RoleDeveloper}
	admin := soda.Person{ID: "admin-1", Username: "admin", DisplayName: "Admin", Role: soda.RoleAdmin}
	project := soda.Project{ID: "project-1", Name: "Live project"}
	api := &fakeAPI{
		people:   []soda.Person{developer, admin},
		projects: []soda.Project{project},
		active: []soda.ActiveSSHConnection{{
			ProjectID: project.ID, PersonID: developer.ID, ConnectedAt: 1,
			ClientAddress: "192.0.2.10", ClientPort: 54321,
		}},
	}
	app := testServer(t, api, fakeAuth{})
	for _, test := range []struct {
		user       soda.Person
		wantClient bool
	}{{developer, false}, {admin, true}} {
		token, err := app.sessions.create(test.user)
		if err != nil {
			t.Fatal(err)
		}
		response := request(app, http.MethodGet, "/projects/project-1/sessions", "", &http.Cookie{Name: sessionCookie, Value: token})
		contains := strings.Contains(response.Body.String(), "192.0.2.10:54321")
		if response.Code != http.StatusOK || contains != test.wantClient || !strings.Contains(response.Body.String(), "Alice") {
			t.Fatalf("unexpected session visibility for %s: %d %q", test.user.Role, response.Code, response.Body.String())
		}
	}
}

func testServer(t *testing.T, api soda.API, authenticator fakeAuth) *Server {
	t.Helper()
	app, err := New(api, authenticator)
	if err != nil {
		t.Fatal(err)
	}
	return app
}

func request(app *Server, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	app.Handler().ServeHTTP(response, req)
	return response
}
