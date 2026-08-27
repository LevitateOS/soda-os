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

	"github.com/LevitateOS/soda-os/cockpit/internal/auth"
	"github.com/LevitateOS/soda-os/cockpit/internal/soda"
)

type fakeAuth struct {
	result    auth.Result
	err       error
	changeErr error
}

func (a fakeAuth) Authenticate(_, _ string) (auth.Result, error) {
	if a.err != nil {
		return "", a.err
	}
	if a.result == "" {
		return auth.Authenticated, nil
	}
	return a.result, nil
}
func (a fakeAuth) ChangePassword(_, _, _ string) error { return a.changeErr }

type fakeAPI struct {
	people         []soda.Person
	projects       []soda.Project
	members        []soda.Person
	worktrees      []soda.Worktree
	jobs           []soda.ProvisioningJob
	toolchain      *soda.ToolchainInstallation
	statuses       []soda.WorktreeStatus
	active         []soda.ActiveSSHConnection
	events         <-chan soda.Event
	created        *soda.CreatePersonRequest
	createdProject *soda.CreateProjectRequest
	keys           []soda.SSHDeviceKey
	keyPersonIDs   []string
	retried        bool
	eventProject   string
	hostCalls      int
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
func (f *fakeAPI) SSHDeviceKeys(_ context.Context, personID string) ([]soda.SSHDeviceKey, error) {
	f.keyPersonIDs = append(f.keyPersonIDs, personID)
	var keys []soda.SSHDeviceKey
	for _, key := range f.keys {
		if key.PersonID == personID {
			keys = append(keys, key)
		}
	}
	return keys, nil
}
func (f *fakeAPI) CreateSSHDeviceKey(_ context.Context, personID, label, publicKey, hint string) (soda.SSHDeviceKey, error) {
	keyType := "unknown"
	if fields := strings.Fields(publicKey); len(fields) != 0 {
		keyType = fields[0]
	}
	key := soda.SSHDeviceKey{ID: "key-new", PersonID: personID, Label: label, Type: keyType, PublicKey: publicKey, Fingerprint: "SHA256:new", IdentityFileHint: hint}
	f.keys = append(f.keys, key)
	return key, nil
}
func (f *fakeAPI) RevokeSSHDeviceKey(_ context.Context, personID, keyID string) (soda.SSHDeviceKey, error) {
	for index, key := range f.keys {
		if key.PersonID == personID && key.ID == keyID {
			f.keys = append(f.keys[:index], f.keys[index+1:]...)
			return key, nil
		}
	}
	return soda.SSHDeviceKey{}, errors.New("key not found")
}
func (f *fakeAPI) CreateProject(_ context.Context, request soda.CreateProjectRequest) (soda.Project, error) {
	f.createdProject = &request
	return soda.Project{ID: "project-1"}, nil
}
func (f *fakeAPI) Members(context.Context, string) ([]soda.Person, error) {
	if f.members != nil {
		return f.members, nil
	}
	return f.people, nil
}
func (f *fakeAPI) AddCollaborator(context.Context, string, string) (soda.Worktree, error) {
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
	f.hostCalls++
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
	if home.Code != http.StatusOK || !strings.Contains(home.Body.String(), "Your Soda projects") {
		t.Fatalf("unexpected home response: %d %q", home.Code, home.Body.String())
	}
	people := request(app, http.MethodGet, "/team", "", cookies[0])
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
		!strings.Contains(installing.Body.String(), `aria-busy="true"`) ||
		strings.Contains(installing.Body.String(), `Retry project setup`) {
		t.Fatalf("expected live installing fragment, got %d %q", installing.Code, installing.Body.String())
	}

	api.jobs[0].State = "ready"
	ready := request(app, http.MethodGet, "/projects/project-1/provisioning", "", cookie)
	if ready.Code != http.StatusOK || !strings.Contains(ready.Body.String(), `sse:provisioning_changed`) ||
		strings.Contains(ready.Body.String(), `every 2s`) ||
		strings.Contains(ready.Body.String(), `Retry project setup`) || !strings.Contains(ready.Body.String(), `>Ready<`) {
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
		"role": {"developer"}, "password": {"temporary"},
	}.Encode()
	req := httptest.NewRequest(http.MethodPost, "/team", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
	response := httptest.NewRecorder()
	app.Handler().ServeHTTP(response, req)
	if response.Code != http.StatusOK || response.Header().Get("HX-Redirect") != "" ||
		!strings.Contains(response.Body.String(), "Team member added.") {
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

type changingAuth struct {
	result    auth.Result
	changes   [][3]string
	changeErr error
}

func (a *changingAuth) Authenticate(_, _ string) (auth.Result, error) { return a.result, nil }
func (a *changingAuth) ChangePassword(username, current, replacement string) error {
	a.changes = append(a.changes, [3]string{username, current, replacement})
	return a.changeErr
}

func TestFirstLoginRequiresPasswordReplacementThenSignsIn(t *testing.T) {
	alice := soda.Person{ID: "person-1", Username: "alice", DisplayName: "Alice", Role: soda.RoleDeveloper}
	authenticator := &changingAuth{result: auth.PasswordChangeRequired}
	app := testServer(t, &fakeAPI{people: []soda.Person{alice}}, authenticator)
	login := request(app, http.MethodPost, "/login", url.Values{"username": {"alice"}, "password": {"temporary"}}.Encode(), nil)
	if login.Code != http.StatusOK || !strings.Contains(login.Body.String(), "Activate your Soda account") || len(login.Result().Cookies()) != 0 {
		t.Fatalf("unexpected activation response: %d %q", login.Code, login.Body.String())
	}
	short := request(app, http.MethodPost, "/activate-password", url.Values{
		"username": {"alice"}, "current_password": {"temporary"}, "new_password": {"short"}, "confirm_password": {"short"},
	}.Encode(), nil)
	if short.Code != http.StatusUnprocessableEntity || len(authenticator.changes) != 0 {
		t.Fatalf("short password reached PAM: %d %#v", short.Code, authenticator.changes)
	}
	activated := request(app, http.MethodPost, "/activate-password", url.Values{
		"username": {"alice"}, "current_password": {"temporary"}, "new_password": {"simple"}, "confirm_password": {"simple"},
	}.Encode(), nil)
	if activated.Code != http.StatusSeeOther || activated.Header().Get("Location") != "/account" || len(activated.Result().Cookies()) != 1 {
		t.Fatalf("unexpected activation completion: %d %v", activated.Code, activated.Header())
	}
	if len(authenticator.changes) != 1 || authenticator.changes[0] != [3]string{"alice", "temporary", "simple"} {
		t.Fatalf("password change = %#v", authenticator.changes)
	}
}

func TestMyAccountManagesOnlyCurrentUsersSSHDevices(t *testing.T) {
	alice := soda.Person{ID: "person-1", Username: "alice", DisplayName: "Alice", Role: soda.RoleAdmin}
	bob := soda.Person{ID: "person-2", Username: "bob", DisplayName: "Bob", Role: soda.RoleDeveloper}
	api := &fakeAPI{people: []soda.Person{alice, bob}, keys: []soda.SSHDeviceKey{{ID: "bob-key", PersonID: bob.ID, Label: "Bob laptop"}}}
	app := testServer(t, api, fakeAuth{})
	token, err := app.sessions.create(alice)
	if err != nil {
		t.Fatal(err)
	}
	cookie := &http.Cookie{Name: sessionCookie, Value: token}
	form := url.Values{"label": {"Alice laptop"}, "public_key": {"ssh-ed25519 AAAA alice"}, "identity_file_hint": {"~/.ssh/id_ed25519"}}.Encode()
	created := request(app, http.MethodPost, "/account/ssh-keys", form, cookie)
	if created.Code != http.StatusSeeOther || len(api.keys) != 2 || api.keys[1].PersonID != alice.ID {
		t.Fatalf("device creation = %d %#v", created.Code, api.keys)
	}
	revoked := request(app, http.MethodPost, "/account/ssh-keys/bob-key/revoke", "", cookie)
	if revoked.Code != http.StatusUnprocessableEntity || len(api.keys) != 2 {
		t.Fatalf("administrator revoked another account's key: %d %#v", revoked.Code, api.keys)
	}
}

func TestProjectCreationForwardsInitialTeamAndHasNoWorktreeCreationRoute(t *testing.T) {
	admin := soda.Person{ID: "admin-1", Username: "admin", DisplayName: "Admin", Role: soda.RoleAdmin}
	bob := soda.Person{ID: "person-2", Username: "bob", DisplayName: "Bob", Role: soda.RoleDeveloper}
	api := &fakeAPI{people: []soda.Person{admin, bob}}
	app := testServer(t, api, fakeAuth{})
	token, err := app.sessions.create(admin)
	if err != nil {
		t.Fatal(err)
	}
	cookie := &http.Cookie{Name: sessionCookie, Value: token}
	form := url.Values{"slug": {"demo"}, "name": {"Demo"}, "profile": {"go"}, "member_ids": {admin.ID, bob.ID}}.Encode()
	response := request(app, http.MethodPost, "/projects", form, cookie)
	if response.Code != http.StatusSeeOther || api.createdProject == nil || len(api.createdProject.InitialPersonIDs) != 2 {
		t.Fatalf("project creation = %d %#v", response.Code, api.createdProject)
	}
	removed := request(app, http.MethodPost, "/projects/project-1/worktrees", "", cookie)
	if removed.Code != http.StatusMethodNotAllowed {
		t.Fatalf("removed worktree creation route returned %d", removed.Code)
	}
}

func TestProjectShowsMembershipWhilePersonalWorkspaceIsPreparing(t *testing.T) {
	admin := soda.Person{ID: "admin-1", Username: "admin", DisplayName: "Admin", Role: soda.RoleAdmin}
	bob := soda.Person{ID: "person-2", Username: "bob", DisplayName: "Bob", Role: soda.RoleDeveloper}
	project := soda.Project{ID: "project-1", Slug: "demo", Name: "Demo", UnixUser: "soda-p-demo", Profile: "go"}
	api := &fakeAPI{
		people: []soda.Person{admin, bob}, members: []soda.Person{bob}, projects: []soda.Project{project},
		jobs: []soda.ProvisioningJob{{ID: "job-1", ProjectID: project.ID, State: "installing"}},
	}
	app := testServer(t, api, fakeAuth{})
	token, err := app.sessions.create(admin)
	if err != nil {
		t.Fatal(err)
	}
	page := request(app, http.MethodGet, "/projects/project-1", "", &http.Cookie{Name: sessionCookie, Value: token})
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "Bob <code>bob</code>") || !strings.Contains(page.Body.String(), ">Preparing<") {
		t.Fatalf("pending member workspace was not visible: %d %q", page.Code, page.Body.String())
	}
}

func TestConnectFragmentRendersPersonalizedSSHConfiguration(t *testing.T) {
	alice := soda.Person{ID: "person-1", Username: "alice", DisplayName: "Alice", Role: soda.RoleDeveloper}
	project := soda.Project{ID: "project-1", Slug: "storefront", Name: "Storefront", UnixUser: "soda-p-storefront", Profile: "go"}
	key := soda.SSHDeviceKey{ID: "key-1", PersonID: alice.ID, Label: "Laptop", Fingerprint: "SHA256:test", IdentityFileHint: "~/.ssh/key with space"}
	workspace := soda.Worktree{ID: "workspace-1", ProjectID: project.ID, PersonID: alice.ID, Branch: "people/alice", Path: "/srv/soda/projects/storefront/worktrees/alice"}
	api := &fakeAPI{people: []soda.Person{alice}, projects: []soda.Project{project}, keys: []soda.SSHDeviceKey{key}, worktrees: []soda.Worktree{workspace}, jobs: []soda.ProvisioningJob{{ID: "job-1", ProjectID: project.ID, State: "ready"}}}
	app := testServer(t, api, fakeAuth{})
	token, err := app.sessions.create(alice)
	if err != nil {
		t.Fatal(err)
	}
	cookie := &http.Cookie{Name: sessionCookie, Value: token}
	fragment := request(app, http.MethodGet, "/projects/project-1/connect?key_id=key-1", "", cookie)
	for _, expected := range []string{`Host soda-storefront`, `User soda-p-storefront`, `IdentityFile &#34;~/.ssh/key with space&#34;`, workspace.Path, `ssh soda-storefront`} {
		if fragment.Code != http.StatusOK || !strings.Contains(fragment.Body.String(), expected) {
			t.Fatalf("connect fragment missing %q: %d %q", expected, fragment.Code, fragment.Body.String())
		}
	}
	download := request(app, http.MethodGet, "/projects/project-1/ssh-config?key_id=key-1", "", cookie)
	if download.Code != http.StatusOK || download.Header().Get("Content-Type") != "text/plain; charset=utf-8" || !strings.Contains(download.Body.String(), `IdentityFile "~/.ssh/key with space"`) {
		t.Fatalf("downloaded SSH config = %d %v %q", download.Code, download.Header(), download.Body.String())
	}
}

func TestDeveloperHomeShowsActiveProjectCollaborators(t *testing.T) {
	alice := soda.Person{ID: "person-1", Username: "alice", DisplayName: "Alice", Role: soda.RoleDeveloper}
	bob := soda.Person{ID: "person-2", Username: "bob", DisplayName: "Bob", Role: soda.RoleDeveloper}
	project := soda.Project{ID: "project-1", Slug: "storefront", Name: "Storefront", UnixUser: "soda-p-storefront", Profile: "go"}
	workspace := soda.Worktree{ID: "workspace-1", ProjectID: project.ID, PersonID: alice.ID, Branch: "people/alice", Path: "/srv/soda/projects/storefront/worktrees/alice"}
	api := &fakeAPI{
		people: []soda.Person{alice, bob}, projects: []soda.Project{project}, worktrees: []soda.Worktree{workspace},
		jobs: []soda.ProvisioningJob{{ID: "job-1", ProjectID: project.ID, State: "ready"}},
		active: []soda.ActiveSSHConnection{
			{ID: "session-bob-1", ProjectID: project.ID, PersonID: bob.ID},
			{ID: "session-bob-2", ProjectID: project.ID, PersonID: bob.ID},
		},
	}
	app := testServer(t, api, fakeAuth{})
	token, err := app.sessions.create(alice)
	if err != nil {
		t.Fatal(err)
	}
	home := request(app, http.MethodGet, "/", "", &http.Cookie{Name: sessionCookie, Value: token})
	if home.Code != http.StatusOK || !strings.Contains(home.Body.String(), "Active now:</strong> Bob") || strings.Count(home.Body.String(), "Bob") != 1 {
		t.Fatalf("developer home did not show deduplicated active collaborators: %d %q", home.Code, home.Body.String())
	}
}

func testServer(t *testing.T, api soda.API, authenticator auth.Authenticator) *Server {
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
