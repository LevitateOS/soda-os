package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/cockpit/internal/auth"
	"github.com/LevitateOS/soda-os/cockpit/internal/daemonclient"
)

func TestProvisioningFragmentShowsCurrentState(t *testing.T) {
	admin := daemonclient.Person{ID: "admin-1", Username: "admin", DisplayName: "Admin", Role: daemonclient.RoleAdmin}
	project := daemonclient.Project{ID: "project-1", Name: "Live project"}
	api := &fakePorts{
		accounts: fakeAccounts{people: []daemonclient.Person{admin}},
		projects: fakeProjects{projects: []daemonclient.Project{project}, jobs: []daemonclient.ProvisioningJob{{ID: "job-1", ProjectID: project.ID, State: "installing"}}},
	}
	app := testServer(t, api, &changingAuth{})
	token, err := app.sessions.create(admin)
	if err != nil {
		t.Fatal(err)
	}
	cookie := &http.Cookie{Name: sessionCookie, Value: token}

	installing := request(app, http.MethodGet, "/projects/project-1/provisioning", "", cookie)
	if installing.Code != http.StatusOK || strings.Contains(installing.Body.String(), `sse:`) ||
		!strings.Contains(installing.Body.String(), `aria-busy="true"`) ||
		strings.Contains(installing.Body.String(), `Retry project setup`) {
		t.Fatalf("expected installing fragment, got %d %q", installing.Code, installing.Body.String())
	}

	api.projects.jobs[0].State = "ready"
	ready := request(app, http.MethodGet, "/projects/project-1/provisioning", "", cookie)
	if ready.Code != http.StatusOK || strings.Contains(ready.Body.String(), `sse:`) ||
		strings.Contains(ready.Body.String(), `Retry project setup`) || !strings.Contains(ready.Body.String(), `>Ready<`) {
		t.Fatalf("expected ready fragment, got %d %q", ready.Code, ready.Body.String())
	}
}

func TestHTMXRetryReturnsInstallingFragment(t *testing.T) {
	admin := daemonclient.Person{ID: "admin-1", Username: "admin", DisplayName: "Admin", Role: daemonclient.RoleAdmin}
	project := daemonclient.Project{ID: "project-1", Name: "Live project"}
	api := &fakePorts{accounts: fakeAccounts{people: []daemonclient.Person{admin}}, projects: fakeProjects{projects: []daemonclient.Project{project}}}
	app := testServer(t, api, &changingAuth{})
	token, err := app.sessions.create(admin)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/projects/project-1/provisioning", nil)
	req.Header.Set("HX-Request", "true")
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
	response := httptest.NewRecorder()
	app.Handler().ServeHTTP(response, req)

	if !api.projects.retried || response.Code != http.StatusOK || response.Header().Get("HX-Redirect") != "" ||
		!strings.Contains(response.Body.String(), `aria-busy="true"`) || strings.Contains(response.Body.String(), `sse:`) {
		t.Fatalf("expected HTMX retry fragment, got retried=%t status=%d headers=%v body=%q",
			api.projects.retried, response.Code, response.Header(), response.Body.String())
	}
}

func TestAdminHTMXPersonFlow(t *testing.T) {
	admin := daemonclient.Person{ID: "admin-1", Username: "admin", DisplayName: "Admin", Role: daemonclient.RoleAdmin}
	api := &fakePorts{accounts: fakeAccounts{people: []daemonclient.Person{admin}}}
	app := testServer(t, api, &changingAuth{})
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
	if api.accounts.created == nil || api.accounts.created.Username != "bob" {
		t.Fatalf("person request was not forwarded: %#v", api.accounts.created)
	}
}

func TestMyAccountManagesOnlyCurrentUsersSSHDevices(t *testing.T) {
	alice := daemonclient.Person{ID: "person-1", Username: "alice", DisplayName: "Alice", Role: daemonclient.RoleAdmin}
	bob := daemonclient.Person{ID: "person-2", Username: "bob", DisplayName: "Bob", Role: daemonclient.RoleDeveloper}
	api := &fakePorts{accounts: fakeAccounts{people: []daemonclient.Person{alice, bob}, keys: []daemonclient.SSHDeviceKey{{ID: "bob-key", PersonID: bob.ID, Label: "Bob laptop"}}}}
	app := testServer(t, api, &changingAuth{})
	token, err := app.sessions.create(alice)
	if err != nil {
		t.Fatal(err)
	}
	cookie := &http.Cookie{Name: sessionCookie, Value: token}
	form := url.Values{"label": {"Alice laptop"}, "public_key": {"ssh-ed25519 AAAA alice"}, "identity_file_hint": {"~/.ssh/id_ed25519"}}.Encode()
	created := request(app, http.MethodPost, "/account/ssh-keys", form, cookie)
	if created.Code != http.StatusSeeOther || len(api.accounts.keys) != 2 || api.accounts.keys[1].PersonID != alice.ID {
		t.Fatalf("device creation = %d %#v", created.Code, api.accounts.keys)
	}
	revoked := request(app, http.MethodPost, "/account/ssh-keys/bob-key/revoke", "", cookie)
	if revoked.Code != http.StatusUnprocessableEntity || len(api.accounts.keys) != 2 {
		t.Fatalf("administrator revoked another account's key: %d %#v", revoked.Code, api.accounts.keys)
	}
}

func TestProjectCreationForwardsInitialTeamAndHasNoWorktreeCreationRoute(t *testing.T) {
	admin := daemonclient.Person{ID: "admin-1", Username: "admin", DisplayName: "Admin", Role: daemonclient.RoleAdmin}
	bob := daemonclient.Person{ID: "person-2", Username: "bob", DisplayName: "Bob", Role: daemonclient.RoleDeveloper}
	api := &fakePorts{accounts: fakeAccounts{people: []daemonclient.Person{admin, bob}}}
	app := testServer(t, api, &changingAuth{})
	token, err := app.sessions.create(admin)
	if err != nil {
		t.Fatal(err)
	}
	cookie := &http.Cookie{Name: sessionCookie, Value: token}
	form := url.Values{"slug": {"demo"}, "name": {"Demo"}, "profile": {"go"}, "repository_source": {"built_in"}, "member_ids": {admin.ID, bob.ID}}.Encode()
	response := request(app, http.MethodPost, "/projects", form, cookie)
	if response.Code != http.StatusSeeOther || api.projects.created == nil || len(api.projects.created.InitialPersonIDs) != 2 {
		t.Fatalf("project creation = %d %#v", response.Code, api.projects.created)
	}
	removed := request(app, http.MethodPost, "/projects/project-1/worktrees", "", cookie)
	if removed.Code != http.StatusMethodNotAllowed {
		t.Fatalf("removed worktree creation route returned %d", removed.Code)
	}
}

func TestProjectCreationOffersBuiltInAndExternalGit(t *testing.T) {
	admin := daemonclient.Person{ID: "admin-1", Username: "admin", DisplayName: "Admin", Role: daemonclient.RoleAdmin}
	api := &fakePorts{accounts: fakeAccounts{people: []daemonclient.Person{admin}}}
	app := testServer(t, api, &changingAuth{})
	token, err := app.sessions.create(admin)
	if err != nil {
		t.Fatal(err)
	}
	cookie := &http.Cookie{Name: sessionCookie, Value: token}
	page := request(app, http.MethodGet, "/projects", "", cookie)
	for _, text := range []string{"Create a new repository on this Soda server", "Connect an existing Git repository"} {
		if !strings.Contains(page.Body.String(), text) {
			t.Fatalf("project source choice %q missing from %q", text, page.Body.String())
		}
	}

	external := url.Values{"slug": {"external"}, "name": {"External"}, "profile": {"go"}, "repository_source": {"external"}, "remote_url": {"ssh://git@example.test/team/external.git"}, "member_ids": {admin.ID}}.Encode()
	response := request(app, http.MethodPost, "/projects", external, cookie)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("external project creation status = %d", response.Code)
	}
	source, ok := api.projects.created.Source.(daemonclient.GitProjectSource)
	if !ok || source.RemoteURL != "ssh://git@example.test/team/external.git" {
		t.Fatalf("external source = %#v", api.projects.created.Source)
	}

	missing := url.Values{"slug": {"missing"}, "name": {"Missing"}, "profile": {"go"}, "repository_source": {"external"}, "member_ids": {admin.ID}}.Encode()
	response = request(app, http.MethodPost, "/projects", missing, cookie)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("missing external address status = %d", response.Code)
	}
}

func TestProjectShowsMembershipWhilePersonalWorkspaceIsPreparing(t *testing.T) {
	admin := daemonclient.Person{ID: "admin-1", Username: "admin", DisplayName: "Admin", Role: daemonclient.RoleAdmin}
	bob := daemonclient.Person{ID: "person-2", Username: "bob", DisplayName: "Bob", Role: daemonclient.RoleDeveloper}
	project := daemonclient.Project{ID: "project-1", Slug: "demo", Name: "Demo", UnixUser: "soda-p-demo", Profile: "go"}
	api := &fakePorts{
		accounts: fakeAccounts{people: []daemonclient.Person{admin, bob}},
		projects: fakeProjects{members: []daemonclient.Person{bob}, projects: []daemonclient.Project{project}, jobs: []daemonclient.ProvisioningJob{{ID: "job-1", ProjectID: project.ID, State: "installing"}}},
	}
	app := testServer(t, api, &changingAuth{})
	token, err := app.sessions.create(admin)
	if err != nil {
		t.Fatal(err)
	}
	page := request(app, http.MethodGet, "/projects/project-1", "", &http.Cookie{Name: sessionCookie, Value: token})
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "Bob <code>bob</code>") || !strings.Contains(page.Body.String(), ">Preparing<") {
		t.Fatalf("pending member workspace was not visible: %d %q", page.Code, page.Body.String())
	}
	for _, removed := range []string{"sse-connect", "sse:", "Personal workspace Git status", "Active development sessions"} {
		if strings.Contains(page.Body.String(), removed) {
			t.Fatalf("project page retained removed live-status behavior %q: %q", removed, page.Body.String())
		}
	}
}

func TestConnectFragmentRendersPersonalizedSSHConfiguration(t *testing.T) {
	alice := daemonclient.Person{ID: "person-1", Username: "alice", DisplayName: "Alice", Role: daemonclient.RoleDeveloper}
	project := daemonclient.Project{ID: "project-1", Slug: "storefront", Name: "Storefront", UnixUser: "soda-p-storefront", Profile: "go"}
	key := daemonclient.SSHDeviceKey{ID: "key-1", PersonID: alice.ID, Label: "Laptop", Fingerprint: "SHA256:test", IdentityFileHint: "~/.ssh/key with space"}
	workspace := daemonclient.Worktree{ID: "workspace-1", ProjectID: project.ID, PersonID: alice.ID, Branch: "people/alice", Path: "/srv/soda/projects/storefront/worktrees/alice"}
	api := &fakePorts{
		accounts: fakeAccounts{people: []daemonclient.Person{alice}, keys: []daemonclient.SSHDeviceKey{key}},
		projects: fakeProjects{members: []daemonclient.Person{alice}, projects: []daemonclient.Project{project}, worktrees: []daemonclient.Worktree{workspace}, jobs: []daemonclient.ProvisioningJob{{ID: "job-1", ProjectID: project.ID, State: "ready"}}},
	}
	app := testServerWithURLs(t, api, &changingAuth{}, "atlas.example.ts.net", "")
	token, err := app.sessions.create(alice)
	if err != nil {
		t.Fatal(err)
	}
	cookie := &http.Cookie{Name: sessionCookie, Value: token}
	fragment := request(app, http.MethodGet, "/projects/project-1/connect?key_id=key-1", "", cookie)
	for _, expected := range []string{`Host soda-storefront`, `HostName atlas.example.ts.net`, `User soda-p-storefront`, `IdentityFile &#34;~/.ssh/key with space&#34;`, workspace.Path, `ssh soda-storefront`, `soda-p-storefront@atlas.example.ts.net`} {
		if fragment.Code != http.StatusOK || !strings.Contains(fragment.Body.String(), expected) {
			t.Fatalf("connect fragment missing %q: %d %q", expected, fragment.Code, fragment.Body.String())
		}
	}
	download := request(app, http.MethodGet, "/projects/project-1/ssh-config?key_id=key-1", "", cookie)
	if download.Code != http.StatusOK || download.Header().Get("Content-Type") != "text/plain; charset=utf-8" || !strings.Contains(download.Body.String(), `HostName atlas.example.ts.net`) || !strings.Contains(download.Body.String(), `IdentityFile "~/.ssh/key with space"`) {
		t.Fatalf("downloaded SSH config = %d %v %q", download.Code, download.Header(), download.Body.String())
	}
}

func TestAdminHomeRendersApplianceAddress(t *testing.T) {
	admin := daemonclient.Person{ID: "admin-1", Username: "admin", DisplayName: "Admin", Role: daemonclient.RoleAdmin}
	api := &fakePorts{accounts: fakeAccounts{people: []daemonclient.Person{admin}}}
	app := testServerWithURLs(t, api, &changingAuth{}, "atlas.example.ts.net", "http://atlas.example.ts.net:3000")
	token, err := app.sessions.create(admin)
	if err != nil {
		t.Fatal(err)
	}
	page := request(app, http.MethodGet, "/", "", &http.Cookie{Name: sessionCookie, Value: token})
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), `https://atlas.example.ts.net:9090`) || !strings.Contains(page.Body.String(), `href="http://atlas.example.ts.net:3000"`) || strings.Contains(page.Body.String(), `atlas.local`) {
		t.Fatalf("home page = %d %q", page.Code, page.Body.String())
	}
}

func TestDashboardHidesBuiltInGitWhenForgejoURLIsUnavailable(t *testing.T) {
	admin := daemonclient.Person{ID: "admin-1", Username: "admin", DisplayName: "Admin", Role: daemonclient.RoleAdmin}
	api := &fakePorts{accounts: fakeAccounts{people: []daemonclient.Person{admin}}}
	app := testServerWithURLs(t, api, &changingAuth{}, "atlas.local", "")
	token, err := app.sessions.create(admin)
	if err != nil {
		t.Fatal(err)
	}
	page := request(app, http.MethodGet, "/", "", &http.Cookie{Name: sessionCookie, Value: token})
	if page.Code != http.StatusOK || strings.Contains(page.Body.String(), `>Built-in Git<`) {
		t.Fatalf("home page = %d %q", page.Code, page.Body.String())
	}
}

func testServer(t *testing.T, ports *fakePorts, authenticator auth.Authenticator) *Server {
	return testServerWithURLs(t, ports, authenticator, "soda.example.ts.net", "")
}

func testServerWithAddress(t *testing.T, ports *fakePorts, authenticator auth.Authenticator, address string) *Server {
	return testServerWithURLs(t, ports, authenticator, address, "")
}

func testServerWithURLs(t *testing.T, ports *fakePorts, authenticator auth.Authenticator, address, forgejoURL string) *Server {
	t.Helper()
	if ports.projects.members == nil {
		ports.projects.members = ports.accounts.people
	}
	app, err := New(Ports{Accounts: &ports.accounts, Projects: &ports.projects, Host: &ports.host, Updates: &ports.updates}, authenticator, address, forgejoURL)
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
