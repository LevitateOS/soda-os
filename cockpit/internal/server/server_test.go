package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/cockpit/internal/soda"
)

type fakeAuth struct{ err error }

func (a fakeAuth) Authenticate(_, _ string) error { return a.err }

type fakeAPI struct {
	people   []soda.Person
	projects []soda.Project
	created  *soda.CreatePersonRequest
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
func (f *fakeAPI) Worktrees(context.Context, string) ([]soda.Worktree, error)   { return nil, nil }
func (f *fakeAPI) Jobs(context.Context, string) ([]soda.ProvisioningJob, error) { return nil, nil }
func (f *fakeAPI) RetryProvisioning(context.Context, string) (soda.ProvisioningJob, error) {
	return soda.ProvisioningJob{}, nil
}
func (f *fakeAPI) Toolchain(context.Context, string) (*soda.ToolchainInstallation, error) {
	return nil, nil
}
func (f *fakeAPI) DeployKey(context.Context, string) (soda.DeployKey, error) {
	return soda.DeployKey{}, nil
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
	if response.Code != http.StatusNoContent || response.Header().Get("HX-Redirect") != "/people" {
		t.Fatalf("unexpected HTMX response: %d %#v", response.Code, response.Header())
	}
	if api.created == nil || api.created.Username != "bob" {
		t.Fatalf("person request was not forwarded: %#v", api.created)
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
