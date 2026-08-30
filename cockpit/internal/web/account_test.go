package web

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/cockpit/internal/auth"
	"github.com/LevitateOS/soda-os/cockpit/internal/daemonclient"
	"github.com/stretchr/testify/require"
)

func TestHealthAndLoginPageArePublic(t *testing.T) {
	app := testServer(t, &fakePorts{}, &changingAuth{})
	for path, expected := range map[string]string{"/healthz": "ok\n", "/login": "Sign in to Soda OS"} {
		response := request(app, http.MethodGet, path, "", nil)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), expected) {
			t.Fatalf("unexpected %s response: %d %q", path, response.Code, response.Body.String())
		}
	}
	login := request(app, http.MethodGet, "/login", "", nil)
	require.NotContains(t, login.Body.String(), `name="password"`)
}

func TestProtectedPageRedirectsToLogin(t *testing.T) {
	app := testServer(t, &fakePorts{}, &changingAuth{})
	response := request(app, http.MethodGet, "/", "", nil)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/login" {
		t.Fatalf("unexpected response: %d %q", response.Code, response.Header().Get("Location"))
	}
}

func TestProjectCardsLeaveConnectionGuidanceToProjectDetail(t *testing.T) {
	admin := daemonclient.Person{ID: "admin-1", Username: "admin", DisplayName: "Admin", Role: daemonclient.RoleAdmin}
	project := daemonclient.Project{ID: "project-1", Slug: "demo", Name: "Demo", UnixUser: "soda-p-demo", Profile: "go"}
	app := testServer(t, &fakePorts{accounts: fakeAccounts{people: []daemonclient.Person{admin}}, projects: fakeProjects{projects: []daemonclient.Project{project}, jobs: []daemonclient.ProvisioningJob{{ProjectID: project.ID, State: "ready"}}}}, &changingAuth{})
	token, err := app.sessions.create(admin)
	if err != nil {
		t.Fatal(err)
	}
	response := request(app, http.MethodGet, "/projects", "", &http.Cookie{Name: sessionCookie, Value: token})
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Demo") {
		t.Fatalf("project cards = %d %q", response.Code, response.Body.String())
	}
	for _, removed := range []string{"Add an SSH device", "personal workspace is being prepared", "Connect:"} {
		if strings.Contains(response.Body.String(), removed) {
			t.Fatalf("project card retained connection guidance %q", removed)
		}
	}
}

func TestOSUpdateControlsAreAdministratorOnlyAndUseVerifiedExactRelease(t *testing.T) {
	digest := "sha256:" + strings.Repeat("b", 64)
	exact := "ghcr.io/levitateos/soda-os@" + digest
	status := daemonclient.OSUpdateStatus{Booted: &daemonclient.OSDeployment{Version: "0.2.0", Digest: "sha256:" + strings.Repeat("a", 64), Architecture: "arm64"}}
	release := daemonclient.OSRelease{ImageReference: exact, Version: "0.3.0", Digest: digest, StateSchema: 3, Available: true}

	developer := daemonclient.Person{ID: "dev-1", Username: "dev", DisplayName: "Developer", Role: daemonclient.RoleDeveloper}
	developerServer := testServer(t, &fakePorts{accounts: fakeAccounts{people: []daemonclient.Person{developer}}}, &changingAuth{})
	developerToken, err := developerServer.sessions.create(developer)
	if err != nil {
		t.Fatal(err)
	}
	response := request(developerServer, http.MethodGet, "/os-update", "", &http.Cookie{Name: sessionCookie, Value: developerToken})
	if response.Code != http.StatusForbidden {
		t.Fatalf("developer accessed OS updates: %d", response.Code)
	}

	admin := daemonclient.Person{ID: "admin-1", Username: "admin", DisplayName: "Admin", Role: daemonclient.RoleAdmin}
	api := &fakePorts{accounts: fakeAccounts{people: []daemonclient.Person{admin}}, updates: fakeUpdates{status: status, release: release}}
	app := testServer(t, api, &changingAuth{})
	token, err := app.sessions.create(admin)
	if err != nil {
		t.Fatal(err)
	}
	cookie := &http.Cookie{Name: sessionCookie, Value: token}
	response = request(app, http.MethodPost, "/os-update/stage", "", cookie)
	if response.Code != http.StatusOK || api.updates.stagedImage != exact {
		t.Fatalf("unexpected stage result: %d %q exact=%q", response.Code, response.Body.String(), api.updates.stagedImage)
	}

	response = request(app, http.MethodPost, "/os-update/activate", "", cookie)
	if response.Code != http.StatusUnprocessableEntity || api.updates.activateCalls != 0 {
		t.Fatalf("activation lacked confirmation gate: %d calls=%d", response.Code, api.updates.activateCalls)
	}
	form := url.Values{"confirm_reboot": {"yes"}}.Encode()
	response = request(app, http.MethodPost, "/os-update/activate", form, cookie)
	if response.Code != http.StatusOK || api.updates.activateCalls != 1 {
		t.Fatalf("confirmed activation failed: %d calls=%d", response.Code, api.updates.activateCalls)
	}
}

func TestPasswordLoginProgressesToPasswordPageAndCreatesSession(t *testing.T) {
	api := &fakePorts{accounts: fakeAccounts{people: []daemonclient.Person{{ID: "person-1", Username: "alice", DisplayName: "Alice", Role: daemonclient.RoleDeveloper}}}}
	authenticator := &changingAuth{}
	app := testServer(t, api, authenticator)
	username := request(app, http.MethodPost, "/login", url.Values{"username": {"alice"}}.Encode(), nil)
	require.Equal(t, http.StatusOK, username.Code)
	require.Contains(t, username.Body.String(), `name="password"`)
	require.Empty(t, username.Result().Cookies())
	form := url.Values{"username": {"alice"}, "password": {"correct"}}.Encode()
	login := request(app, http.MethodPost, "/login/password", form, nil)
	require.Equal(t, http.StatusSeeOther, login.Code, login.Body.String())
	require.Equal(t, []string{"alice"}, authenticator.passwordlessCalls)
	require.Equal(t, [][2]string{{"alice", "correct"}}, authenticator.authenticateCalls)
	cookies := login.Result().Cookies()
	require.Len(t, cookies, 1)
	require.True(t, cookies[0].Secure)
	require.True(t, cookies[0].HttpOnly)
	home := request(app, http.MethodGet, "/", "", cookies[0])
	require.Equal(t, http.StatusOK, home.Code)
	require.Contains(t, home.Body.String(), "Your Soda projects")
	people := request(app, http.MethodGet, "/team", "", cookies[0])
	require.Equal(t, http.StatusForbidden, people.Code)
}

func TestPasswordlessLoginCreatesNormalSession(t *testing.T) {
	alice := daemonclient.Person{ID: "person-1", Username: "alice", DisplayName: "Alice", Role: daemonclient.RoleDeveloper}
	authenticator := &changingAuth{passwordless: true}
	app := testServer(t, &fakePorts{accounts: fakeAccounts{people: []daemonclient.Person{alice}}}, authenticator)
	login := request(app, http.MethodPost, "/login", url.Values{"username": {"alice"}}.Encode(), nil)
	require.Equal(t, http.StatusSeeOther, login.Code)
	require.Equal(t, "/", login.Header().Get("Location"))
	cookies := login.Result().Cookies()
	require.Len(t, cookies, 1)
	require.True(t, cookies[0].Secure)
	require.True(t, cookies[0].HttpOnly)
	require.Empty(t, authenticator.authenticateCalls)
	home := request(app, http.MethodGet, "/", "", cookies[0])
	require.Equal(t, http.StatusOK, home.Code)
	require.Contains(t, home.Body.String(), "Your Soda projects")
}

func TestInvalidAndUnregisteredLoginsDoNotCreateSessionOrExposeRegistration(t *testing.T) {
	alice := daemonclient.Person{ID: "person-1", Username: "alice", DisplayName: "Alice", Role: daemonclient.RoleDeveloper}
	tests := []struct {
		name          string
		username      string
		authenticator *changingAuth
		passwordless  []string
		password      [][2]string
	}{
		{name: "invalid password", username: "alice", authenticator: &changingAuth{authErr: errors.New("denied")}, passwordless: []string{"alice"}, password: [][2]string{{"alice", "wrong"}}},
		{name: "unregistered account", username: "mallory", authenticator: &changingAuth{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := testServer(t, &fakePorts{accounts: fakeAccounts{people: []daemonclient.Person{alice}}}, test.authenticator)
			username := request(app, http.MethodPost, "/login", url.Values{"username": {test.username}}.Encode(), nil)
			require.Equal(t, http.StatusOK, username.Code)
			require.Contains(t, username.Body.String(), `name="password"`)
			response := request(app, http.MethodPost, "/login/password", url.Values{"username": {test.username}, "password": {"wrong"}}.Encode(), nil)
			require.Equal(t, http.StatusUnauthorized, response.Code)
			require.Contains(t, response.Body.String(), "Invalid username or password")
			require.NotContains(t, response.Body.String(), "not registered")
			require.Empty(t, response.Result().Cookies())
			require.Equal(t, test.passwordless, test.authenticator.passwordlessCalls)
			require.Equal(t, test.password, test.authenticator.authenticateCalls)
		})
	}
}

type changingAuth struct {
	result            auth.Result
	authErr           error
	passwordless      bool
	passwordlessCalls []string
	authenticateCalls [][2]string
	changes           [][3]string
	changeErr         error
}

func (a *changingAuth) AuthenticatePasswordless(username string) (auth.Result, error) {
	a.passwordlessCalls = append(a.passwordlessCalls, username)
	if a.passwordless {
		return auth.Authenticated, nil
	}
	return "", errors.New("password required")
}

func (a *changingAuth) Authenticate(username, password string) (auth.Result, error) {
	a.authenticateCalls = append(a.authenticateCalls, [2]string{username, password})
	if a.authErr != nil {
		return "", a.authErr
	}
	if a.result == "" {
		return auth.Authenticated, nil
	}
	return a.result, nil
}
func (a *changingAuth) ChangePassword(username, current, replacement string) error {
	a.changes = append(a.changes, [3]string{username, current, replacement})
	return a.changeErr
}

func TestFirstLoginRequiresPasswordReplacementThenSignsIn(t *testing.T) {
	alice := daemonclient.Person{ID: "person-1", Username: "alice", DisplayName: "Alice", Role: daemonclient.RoleDeveloper}
	authenticator := &changingAuth{result: auth.PasswordChangeRequired}
	app := testServer(t, &fakePorts{accounts: fakeAccounts{people: []daemonclient.Person{alice}}}, authenticator)
	username := request(app, http.MethodPost, "/login", url.Values{"username": {"alice"}}.Encode(), nil)
	require.Equal(t, http.StatusOK, username.Code)
	require.Contains(t, username.Body.String(), `name="password"`)
	login := request(app, http.MethodPost, "/login/password", url.Values{"username": {"alice"}, "password": {"temporary"}}.Encode(), nil)
	if login.Code != http.StatusOK || len(login.Result().Cookies()) != 0 {
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
