package server

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/LevitateOS/soda-os/cockpit/internal/auth"
	"github.com/LevitateOS/soda-os/cockpit/internal/soda"
	"github.com/LevitateOS/soda-os/internal/version"
)

//go:embed templates/*.html static/*
var content embed.FS

const sessionCookie = "soda_session"

type Server struct {
	templates *template.Template
	assets    http.Handler
	api       soda.API
	auth      auth.Authenticator
	sessions  *sessionStore
}

type sessionStore struct {
	mu      sync.RWMutex
	byToken map[string]soda.Person
}

type pageData struct {
	Title                  string
	Version                string
	User                   soda.Person
	People                 []soda.Person
	AvailablePeople        []soda.Person
	Projects               []soda.Project
	ProjectCards           []projectCardView
	Project                *soda.Project
	Worktrees              []soda.Worktree
	MemberWorkspaces       []memberWorkspaceView
	Jobs                   []soda.ProvisioningJob
	Toolchain              *soda.ToolchainInstallation
	DeployKey              string
	PersonNames            map[string]string
	Error                  string
	HostError              string
	ProjectsError          string
	Admin                  bool
	ProvisioningActive     bool
	Host                   *soda.HostStatus
	OSUpdate               *soda.OSUpdateStatus
	OSRelease              *soda.OSRelease
	Message                string
	DeviceKeys             []soda.SSHDeviceKey
	SelectedDeviceKey      *soda.SSHDeviceKey
	PersonalWorkspace      *soda.Worktree
	SSHConfig              string
	SSHCommand             string
	ConnectError           string
	ProjectState           string
	ProjectStateClass      string
	ProjectReady           bool
	PasswordChangeRequired bool
	Username               string
}

type projectCardView struct {
	Project    soda.Project
	State      string
	StateClass string
}

type connectResources struct {
	Worktrees  []soda.Worktree
	DeviceKeys []soda.SSHDeviceKey
	Jobs       []soda.ProvisioningJob
}

type memberWorkspaceView struct {
	Person    soda.Person
	Workspace *soda.Worktree
}

type userContextKey struct{}

func New(api soda.API, authenticator auth.Authenticator) (*Server, error) {
	templates, err := template.New("root").Funcs(template.FuncMap{
		"bytes":    humanBytes,
		"duration": humanDuration,
		"time":     humanTime,
	}).ParseFS(content, "templates/*.html")
	if err != nil {
		return nil, err
	}
	assetsFS, err := fs.Sub(content, "static")
	if err != nil {
		return nil, err
	}
	return &Server{
		templates: templates,
		assets:    http.FileServer(http.FS(assetsFS)),
		api:       api,
		auth:      authenticator,
		sessions:  &sessionStore{byToken: make(map[string]soda.Person)},
	}, nil
}

func (s *Server) Handler() http.Handler {
	public := http.NewServeMux()
	public.Handle("GET /static/", http.StripPrefix("/static/", s.assets))
	public.HandleFunc("GET /healthz", health)
	public.HandleFunc("GET /login", s.loginPage)
	public.HandleFunc("POST /login", s.login)
	public.HandleFunc("POST /activate-password", s.activatePassword)

	protected := http.NewServeMux()
	protected.HandleFunc("GET /", s.home)
	protected.HandleFunc("POST /logout", s.logout)
	protected.HandleFunc("GET /account", s.account)
	protected.HandleFunc("POST /account/password", s.changePassword)
	protected.HandleFunc("POST /account/ssh-keys", s.createSSHDeviceKey)
	protected.HandleFunc("POST /account/ssh-keys/{key_id}/revoke", s.revokeSSHDeviceKey)
	protected.HandleFunc("GET /team", s.people)
	protected.HandleFunc("POST /team", s.createPerson)
	protected.HandleFunc("GET /projects", s.projects)
	protected.HandleFunc("POST /projects", s.createProject)
	protected.HandleFunc("GET /projects/{project_id}", s.project)
	protected.HandleFunc("GET /projects/{project_id}/connect", s.connectFragment)
	protected.HandleFunc("GET /projects/{project_id}/ssh-config", s.sshConfig)
	protected.HandleFunc("GET /projects/{project_id}/provisioning", s.provisioning)
	protected.HandleFunc("POST /projects/{project_id}/members", s.addCollaborator)
	protected.HandleFunc("POST /projects/{project_id}/provisioning", s.retryProvisioning)
	protected.HandleFunc("GET /profiles", s.profiles)
	protected.HandleFunc("GET /os-update", s.osUpdate)
	protected.HandleFunc("POST /os-update/check", s.checkOSUpdate)
	protected.HandleFunc("POST /os-update/stage", s.stageOSUpdate)
	protected.HandleFunc("POST /os-update/activate", s.activateOSUpdate)

	public.Handle("/", s.requireSession(protected))
	return securityHeaders(public)
}

func (s *Server) ListenAndServeTLS(address, certFile, keyFile string) error {
	return http.ListenAndServeTLS(address, certFile, keyFile, s.Handler())
}

func health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok\n"))
}

func (s *Server) loginPage(w http.ResponseWriter, _ *http.Request) {
	s.render(w, http.StatusOK, "login.html", pageData{Title: "Sign in · Soda OS", Version: version.Version})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid login", http.StatusBadRequest)
		return
	}
	username := r.FormValue("username")
	result, err := s.auth.Authenticate(username, r.FormValue("password"))
	if err != nil {
		s.render(w, http.StatusUnauthorized, "login.html", pageData{Title: "Sign in · Soda OS", Version: version.Version, Error: "Invalid username or password."})
		return
	}
	if result == auth.PasswordChangeRequired {
		s.render(w, http.StatusOK, "login.html", pageData{Title: "Activate account · Soda OS", Version: version.Version, PasswordChangeRequired: true, Username: username})
		return
	}
	s.finishLogin(w, r, username, "/")
}

func (s *Server) activatePassword(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid password change", http.StatusBadRequest)
		return
	}
	username := r.FormValue("username")
	current := r.FormValue("current_password")
	newPassword := r.FormValue("new_password")
	if newPassword != r.FormValue("confirm_password") {
		s.render(w, http.StatusUnprocessableEntity, "login.html", pageData{Title: "Activate account · Soda OS", Version: version.Version, PasswordChangeRequired: true, Username: username, Error: "New passwords do not match."})
		return
	}
	if err := validatePassword(newPassword); err != nil {
		s.render(w, http.StatusUnprocessableEntity, "login.html", pageData{Title: "Activate account · Soda OS", Version: version.Version, PasswordChangeRequired: true, Username: username, Error: err.Error()})
		return
	}
	if err := s.auth.ChangePassword(username, current, newPassword); err != nil {
		s.render(w, http.StatusUnauthorized, "login.html", pageData{Title: "Activate account · Soda OS", Version: version.Version, PasswordChangeRequired: true, Username: username, Error: "The current password was invalid or the password could not be changed."})
		return
	}
	s.finishLogin(w, r, username, "/account")
}

func (s *Server) finishLogin(w http.ResponseWriter, r *http.Request, username, destination string) {
	people, err := s.api.People(r.Context())
	if err != nil {
		http.Error(w, "Soda service unavailable", http.StatusBadGateway)
		return
	}
	var person *soda.Person
	for i := range people {
		if people[i].Username == username {
			person = &people[i]
			break
		}
	}
	if person == nil {
		s.render(w, http.StatusForbidden, "login.html", pageData{Title: "Sign in · Soda OS", Version: version.Version, Error: "This Linux account is not registered with Soda OS."})
		return
	}
	token, err := s.sessions.create(*person)
	if err != nil {
		http.Error(w, "create session", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
	http.Redirect(w, r, destination, http.StatusSeeOther)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		s.sessions.remove(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/", MaxAge: -1, HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	projects, err := s.visibleProjects(r.Context(), user)
	if err != nil {
		http.Error(w, "load projects", http.StatusBadGateway)
		return
	}
	data := pageData{Title: "Soda OS", Version: version.Version, User: user, Projects: projects}
	data.ProjectCards, err = s.projectCards(r.Context(), projects)
	if err != nil {
		data.ProjectsError = "Projects are temporarily unavailable."
	}
	if user.Role == soda.RoleAdmin {
		host, hostErr := s.api.HostStatus(r.Context())
		if hostErr != nil {
			data.HostError = "Host status is temporarily unavailable."
		} else {
			data.Host = &host
		}
	}
	s.render(w, http.StatusOK, "index.html", data)
}

func (s *Server) account(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	keys, err := s.api.SSHDeviceKeys(r.Context(), user.ID)
	if err != nil {
		http.Error(w, "load SSH devices", http.StatusBadGateway)
		return
	}
	s.render(w, http.StatusOK, "account.html", pageData{Title: "My account · Soda OS", Version: version.Version, User: user, DeviceKeys: keys, Admin: user.Role == soda.RoleAdmin})
}

func (s *Server) createSSHDeviceKey(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid SSH device", http.StatusBadRequest)
		return
	}
	user := currentUser(r)
	_, err := s.api.CreateSSHDeviceKey(r.Context(), user.ID, r.FormValue("label"), r.FormValue("public_key"), r.FormValue("identity_file_hint"))
	if err != nil {
		s.renderSSHKeysResult(w, r, http.StatusUnprocessableEntity, "", err.Error())
		return
	}
	if isHTMX(r) {
		s.renderSSHKeysResult(w, r, http.StatusOK, "SSH device added.", "")
		return
	}
	redirect(w, r, "/account")
}

func (s *Server) revokeSSHDeviceKey(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if _, err := s.api.RevokeSSHDeviceKey(r.Context(), user.ID, r.PathValue("key_id")); err != nil {
		s.renderSSHKeysResult(w, r, http.StatusUnprocessableEntity, "", err.Error())
		return
	}
	if isHTMX(r) {
		s.renderSSHKeysResult(w, r, http.StatusOK, "SSH device revoked. Existing sessions remain connected.", "")
		return
	}
	redirect(w, r, "/account")
}

func (s *Server) renderSSHKeysResult(w http.ResponseWriter, r *http.Request, status int, message, errorMessage string) {
	user := currentUser(r)
	keys, _ := s.api.SSHDeviceKeys(r.Context(), user.ID)
	s.render(w, status, "ssh-keys", pageData{User: user, DeviceKeys: keys, Message: message, Error: errorMessage, Admin: user.Role == soda.RoleAdmin})
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid password change", http.StatusBadRequest)
		return
	}
	user := currentUser(r)
	newPassword := r.FormValue("new_password")
	if newPassword != r.FormValue("confirm_password") {
		s.render(w, http.StatusUnprocessableEntity, "password-change", pageData{User: user, Error: "New passwords do not match."})
		return
	}
	if err := validatePassword(newPassword); err != nil {
		s.render(w, http.StatusUnprocessableEntity, "password-change", pageData{User: user, Error: err.Error()})
		return
	}
	if err := s.auth.ChangePassword(user.Username, r.FormValue("current_password"), newPassword); err != nil {
		s.render(w, http.StatusUnprocessableEntity, "password-change", pageData{User: user, Error: "The current password was invalid or the password could not be changed."})
		return
	}
	s.render(w, http.StatusOK, "password-change", pageData{User: user, Message: "Password changed."})
}

func (s *Server) people(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	people, err := s.api.People(r.Context())
	if err != nil {
		http.Error(w, "load people", http.StatusBadGateway)
		return
	}
	s.render(w, http.StatusOK, "people.html", pageData{Title: "Team · Soda OS", Version: version.Version, User: currentUser(r), People: people})
}

func (s *Server) createPerson(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid person", http.StatusBadRequest)
		return
	}
	_, err := s.api.CreatePerson(r.Context(), soda.CreatePersonRequest{
		Username: r.FormValue("username"), DisplayName: r.FormValue("display_name"),
		Email: r.FormValue("email"), Role: soda.Role(r.FormValue("role")),
		Password: r.FormValue("password"),
	})
	if err != nil {
		if isHTMX(r) {
			people, _ := s.api.People(r.Context())
			s.render(w, http.StatusUnprocessableEntity, "people-management", pageData{
				User: currentUser(r), People: people, Error: err.Error(),
			})
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if isHTMX(r) {
		people, loadErr := s.api.People(r.Context())
		if loadErr != nil {
			http.Error(w, "load people", http.StatusBadGateway)
			return
		}
		s.render(w, http.StatusOK, "people-management", pageData{
			User: currentUser(r), People: people, Message: "Team member added.",
		})
		return
	}
	redirect(w, r, "/team")
}

func (s *Server) projects(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	projects, err := s.visibleProjects(r.Context(), user)
	if err != nil {
		http.Error(w, "load projects", http.StatusBadGateway)
		return
	}
	data := pageData{Title: "Projects · Soda OS", Version: version.Version, User: user, Projects: projects}
	data.ProjectCards, err = s.projectCards(r.Context(), projects)
	if err != nil {
		data.ProjectsError = "Projects are temporarily unavailable."
	}
	if user.Role == soda.RoleAdmin {
		data.People, err = s.api.People(r.Context())
		if err != nil {
			http.Error(w, "load team", http.StatusBadGateway)
			return
		}
	}
	s.render(w, http.StatusOK, "projects.html", data)
}

func (s *Server) createProject(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid project", http.StatusBadRequest)
		return
	}
	source := soda.ProjectSource{Kind: "empty"}
	if remote := strings.TrimSpace(r.FormValue("remote_url")); remote != "" {
		source = soda.ProjectSource{Kind: "git", RemoteURL: remote}
	}
	members := append([]string(nil), r.Form["member_ids"]...)
	project, err := s.api.CreateProject(r.Context(), soda.CreateProjectRequest{
		Slug: r.FormValue("slug"), Name: r.FormValue("name"), Profile: r.FormValue("profile"), Source: source, InitialPersonIDs: members,
	})
	if err != nil {
		if isHTMX(r) {
			people, _ := s.api.People(r.Context())
			s.render(w, http.StatusUnprocessableEntity, "project-create", pageData{
				User: currentUser(r), People: people, Error: err.Error(),
			})
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	redirect(w, r, "/projects/"+project.ID)
}

func (s *Server) project(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	project, allowed, err := s.visibleProject(r.Context(), user, r.PathValue("project_id"))
	if err != nil {
		http.Error(w, "load project", http.StatusBadGateway)
		return
	}
	if !allowed {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	worktrees, err := s.api.Worktrees(r.Context(), project.ID)
	if err != nil {
		http.Error(w, "load worktrees", http.StatusBadGateway)
		return
	}
	jobs, installation, err := s.provisioningState(r.Context(), project.ID)
	if err != nil {
		http.Error(w, "load provisioning", http.StatusBadGateway)
		return
	}
	people, err := s.api.People(r.Context())
	if err != nil {
		http.Error(w, "load people", http.StatusBadGateway)
		return
	}
	members, err := s.api.Members(r.Context(), project.ID)
	if err != nil {
		http.Error(w, "load project members", http.StatusBadGateway)
		return
	}
	data := pageData{Title: "Project · Soda OS", Version: version.Version, User: user, Project: &project,
		People: people, Worktrees: worktrees, Jobs: jobs, Toolchain: installation,
		MemberWorkspaces: memberWorkspaceViews(members, worktrees), PersonNames: personNames(people),
		ProvisioningActive: provisioningActive(jobs)}
	data.ProjectState, data.ProjectStateClass = projectState(jobs)
	data.ProjectReady = data.ProjectStateClass == "ready"
	data.AvailablePeople = peopleWithoutMembers(people, members)
	s.addConnectData(r.Context(), &data)
	if user.Role == soda.RoleAdmin {
		key, err := s.api.DeployKey(r.Context(), project.ID)
		if err != nil {
			http.Error(w, "load deploy key", http.StatusBadGateway)
			return
		}
		data.DeployKey = key.PublicKey
	}
	s.render(w, http.StatusOK, "project.html", data)
}

func (s *Server) connectFragment(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	project, allowed, err := s.visibleProject(r.Context(), user, r.PathValue("project_id"))
	if err != nil {
		http.Error(w, "load project", http.StatusBadGateway)
		return
	}
	if !allowed {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	data := pageData{User: user, Project: &project}
	if err = s.loadConnectData(r.Context(), &data, r.URL.Query().Get("key_id")); err != nil {
		data.ConnectError = "Connection details are temporarily unavailable."
	}
	s.render(w, http.StatusOK, "connect", data)
}

func (s *Server) sshConfig(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	project, allowed, err := s.visibleProject(r.Context(), user, r.PathValue("project_id"))
	if err != nil {
		http.Error(w, "load project", http.StatusBadGateway)
		return
	}
	if !allowed {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	data := pageData{User: user, Project: &project}
	if err = s.loadConnectData(r.Context(), &data, r.URL.Query().Get("key_id")); err != nil {
		http.Error(w, "load connection details", http.StatusBadGateway)
		return
	}
	if data.SSHConfig == "" {
		http.Error(w, "a ready personal workspace and SSH device are required", http.StatusPreconditionFailed)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="soda-%s.sshconfig"`, project.Slug))
	_, _ = fmt.Fprint(w, data.SSHConfig)
}

func (s *Server) addConnectData(ctx context.Context, data *pageData) {
	keys, err := s.api.SSHDeviceKeys(ctx, data.User.ID)
	if err != nil {
		data.ConnectError = "Connection details are temporarily unavailable."
		return
	}
	if err := populateConnectData(data, connectResources{Worktrees: data.Worktrees, DeviceKeys: keys, Jobs: data.Jobs}, ""); err != nil {
		data.ConnectError = "Connection details are temporarily unavailable."
	}
}

func (s *Server) loadConnectData(ctx context.Context, data *pageData, selectedKeyID string) error {
	worktrees, err := s.api.Worktrees(ctx, data.Project.ID)
	if err != nil {
		return err
	}
	keys, err := s.api.SSHDeviceKeys(ctx, data.User.ID)
	if err != nil {
		return err
	}
	jobs, _, err := s.provisioningState(ctx, data.Project.ID)
	if err != nil {
		return err
	}
	return populateConnectData(data, connectResources{Worktrees: worktrees, DeviceKeys: keys, Jobs: jobs}, selectedKeyID)
}

func populateConnectData(data *pageData, resources connectResources, selectedKeyID string) error {
	data.Worktrees = resources.Worktrees
	data.DeviceKeys = resources.DeviceKeys
	data.PersonalWorkspace = nil
	for i := range resources.Worktrees {
		if resources.Worktrees[i].PersonID == data.User.ID {
			workspace := resources.Worktrees[i]
			data.PersonalWorkspace = &workspace
			break
		}
	}
	key, err := selectDeviceKey(resources.DeviceKeys, selectedKeyID)
	if err != nil {
		return err
	}
	data.SelectedDeviceKey = key
	data.ProjectState, data.ProjectStateClass = projectState(resources.Jobs)
	data.ProjectReady = data.ProjectStateClass == "ready"
	if data.ProjectReady && data.PersonalWorkspace != nil && data.SelectedDeviceKey != nil {
		data.SSHConfig = personalizedSSHConfig(*data.Project, *data.SelectedDeviceKey)
		data.SSHCommand = personalizedSSHCommand(*data.Project, *data.SelectedDeviceKey)
	}
	return nil
}

func selectDeviceKey(keys []soda.SSHDeviceKey, selectedKeyID string) (*soda.SSHDeviceKey, error) {
	if selectedKeyID == "" {
		if len(keys) == 0 {
			return nil, nil
		}
		selected := keys[0]
		return &selected, nil
	}
	for i := range keys {
		if keys[i].ID == selectedKeyID {
			selected := keys[i]
			return &selected, nil
		}
	}
	return nil, fmt.Errorf("SSH device is not registered to this account")
}

func (s *Server) collaborationData(w http.ResponseWriter, r *http.Request) (pageData, bool) {
	user := currentUser(r)
	project, allowed, err := s.visibleProject(r.Context(), user, r.PathValue("project_id"))
	if err != nil {
		http.Error(w, "load project", http.StatusBadGateway)
		return pageData{}, false
	}
	if !allowed {
		http.Error(w, "forbidden", http.StatusForbidden)
		return pageData{}, false
	}
	worktrees, err := s.api.Worktrees(r.Context(), project.ID)
	if err != nil {
		http.Error(w, "load worktrees", http.StatusBadGateway)
		return pageData{}, false
	}
	people, err := s.api.People(r.Context())
	if err != nil {
		http.Error(w, "load people", http.StatusBadGateway)
		return pageData{}, false
	}
	members, err := s.api.Members(r.Context(), project.ID)
	if err != nil {
		http.Error(w, "load project members", http.StatusBadGateway)
		return pageData{}, false
	}
	data := pageData{
		User: user, Project: &project, People: people, Worktrees: worktrees,
		MemberWorkspaces: memberWorkspaceViews(members, worktrees), AvailablePeople: peopleWithoutMembers(people, members),
		PersonNames: personNames(people),
	}
	jobs, _, setupErr := s.provisioningState(r.Context(), project.ID)
	if setupErr == nil {
		_, stateClass := projectState(jobs)
		data.ProjectReady = stateClass == "ready"
	}
	return data, true
}

func (s *Server) provisioning(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	project, allowed, err := s.visibleProject(r.Context(), user, r.PathValue("project_id"))
	if err != nil {
		http.Error(w, "load project", http.StatusBadGateway)
		return
	}
	if !allowed {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	jobs, installation, err := s.provisioningState(r.Context(), project.ID)
	if err != nil {
		http.Error(w, "load provisioning", http.StatusBadGateway)
		return
	}
	state, stateClass := projectState(jobs)
	s.render(w, http.StatusOK, "provisioning", pageData{
		User: user, Project: &project, Jobs: jobs, Toolchain: installation,
		ProvisioningActive: provisioningActive(jobs), ProjectState: state, ProjectStateClass: stateClass,
	})
}

func (s *Server) addCollaborator(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid team member", http.StatusBadRequest)
		return
	}
	projectID := r.PathValue("project_id")
	if _, err := s.api.AddCollaborator(r.Context(), projectID, r.FormValue("person_id")); err != nil {
		if isHTMX(r) {
			data, ok := s.collaborationData(w, r)
			if ok {
				data.Error = err.Error()
				s.render(w, http.StatusUnprocessableEntity, "collaboration", data)
			}
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if isHTMX(r) {
		data, ok := s.collaborationData(w, r)
		if ok {
			data.Message = "Team member and personal workspace added."
			s.render(w, http.StatusOK, "collaboration", data)
		}
		return
	}
	redirect(w, r, "/projects/"+projectID)
}

func (s *Server) retryProvisioning(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	projectID := r.PathValue("project_id")
	if _, err := s.api.RetryProvisioning(r.Context(), projectID); err != nil {
		if isHTMX(r) {
			user := currentUser(r)
			project, allowed, loadErr := s.visibleProject(r.Context(), user, projectID)
			if loadErr != nil || !allowed {
				http.Error(w, "load project", http.StatusBadGateway)
				return
			}
			jobs, installation, _ := s.provisioningState(r.Context(), projectID)
			state, stateClass := projectState(jobs)
			s.render(w, http.StatusUnprocessableEntity, "provisioning", pageData{
				User: user, Project: &project, Jobs: jobs, Toolchain: installation,
				ProvisioningActive: provisioningActive(jobs), ProjectState: state, ProjectStateClass: stateClass, Error: err.Error(),
			})
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if r.Header.Get("HX-Request") == "true" {
		s.provisioning(w, r)
		return
	}
	redirect(w, r, "/projects/"+projectID)
}

func (s *Server) profiles(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	projects, err := s.visibleProjects(r.Context(), user)
	if err != nil {
		http.Error(w, "load profiles", http.StatusBadGateway)
		return
	}
	s.render(w, http.StatusOK, "profiles.html", pageData{Title: "Development environments · Soda OS", Version: version.Version, User: user, Projects: projects})
}

func (s *Server) osUpdate(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	s.renderOSUpdate(w, r, http.StatusOK, "", "", nil)
}

func (s *Server) checkOSUpdate(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	release, err := s.api.CheckOSUpdate(r.Context())
	if err != nil {
		s.renderOSUpdate(w, r, http.StatusUnprocessableEntity, "", err.Error(), nil)
		return
	}
	message := "This host already runs the current signed Soda OS release."
	if release.Available {
		message = "A signed Soda OS release is available to stage."
	}
	s.renderOSUpdate(w, r, http.StatusOK, message, "", &release)
}

func (s *Server) stageOSUpdate(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	release, err := s.api.CheckOSUpdate(r.Context())
	if err != nil {
		s.renderOSUpdate(w, r, http.StatusUnprocessableEntity, "", err.Error(), nil)
		return
	}
	if !release.Available {
		s.renderOSUpdate(w, r, http.StatusUnprocessableEntity, "", "No newer signed Soda OS release is available.", &release)
		return
	}
	status, err := s.api.StageOSUpdate(r.Context(), release.ImageReference)
	if err != nil {
		s.renderOSUpdate(w, r, http.StatusUnprocessableEntity, "", err.Error(), &release)
		return
	}
	s.renderOSUpdateWithStatus(w, r, http.StatusOK, "Update downloaded and locked. Running work is unchanged; activation still requires confirmation.", "", &release, status)
}

func (s *Server) activateOSUpdate(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid activation confirmation", http.StatusBadRequest)
		return
	}
	confirmed := r.FormValue("confirm_reboot") == "yes"
	if err := s.api.ActivateOSUpdate(r.Context(), confirmed); err != nil {
		s.renderOSUpdate(w, r, http.StatusUnprocessableEntity, "", err.Error(), nil)
		return
	}
	s.renderOSUpdate(w, r, http.StatusOK, "Maintenance reboot requested.", "", nil)
}

func (s *Server) renderOSUpdate(w http.ResponseWriter, r *http.Request, status int, message, errorMessage string, release *soda.OSRelease) {
	value, err := s.api.OSUpdateStatus(r.Context())
	if err != nil {
		s.render(w, http.StatusBadGateway, "os_update.html", pageData{Title: "OS update · Soda OS", Version: version.Version, User: currentUser(r), Admin: true, Error: "OS update status is unavailable."})
		return
	}
	s.renderOSUpdateWithStatus(w, r, status, message, errorMessage, release, value)
}

func (s *Server) renderOSUpdateWithStatus(w http.ResponseWriter, r *http.Request, status int, message, errorMessage string, release *soda.OSRelease, value soda.OSUpdateStatus) {
	s.render(w, status, "os_update.html", pageData{
		Title: "OS update · Soda OS", Version: version.Version, User: currentUser(r), Admin: true,
		OSUpdate: &value, OSRelease: release, Message: message, Error: errorMessage,
	})
}

func (s *Server) visibleProjects(ctx context.Context, user soda.Person) ([]soda.Project, error) {
	if user.Role == soda.RoleAdmin {
		return s.api.Projects(ctx)
	}
	return s.api.ProjectsForPerson(ctx, user.ID)
}

func (s *Server) provisioningState(ctx context.Context, projectID string) ([]soda.ProvisioningJob, *soda.ToolchainInstallation, error) {
	jobs, err := s.api.Jobs(ctx, projectID)
	if err != nil {
		return nil, nil, err
	}
	installation, err := s.api.Toolchain(ctx, projectID)
	if err != nil {
		return nil, nil, err
	}
	return jobs, installation, nil
}

func provisioningActive(jobs []soda.ProvisioningJob) bool {
	for _, job := range jobs {
		if job.State == "installing" {
			return true
		}
	}
	return false
}

func projectState(jobs []soda.ProvisioningJob) (string, string) {
	if len(jobs) == 0 {
		return "Preparing", "preparing"
	}
	switch jobs[0].State {
	case "ready":
		return "Ready", "ready"
	case "failed":
		return "Needs attention", "failed"
	default:
		return "Preparing", "preparing"
	}
}

func (s *Server) projectCards(ctx context.Context, projects []soda.Project) ([]projectCardView, error) {
	cards := make([]projectCardView, 0, len(projects))
	for _, project := range projects {
		jobs, loadErr := s.api.Jobs(ctx, project.ID)
		if loadErr != nil {
			return nil, loadErr
		}
		state, stateClass := projectState(jobs)
		cards = append(cards, projectCardView{
			Project: project, State: state, StateClass: stateClass,
		})
	}
	return cards, nil
}

func peopleWithoutMembers(people, projectMembers []soda.Person) []soda.Person {
	members := make(map[string]struct{}, len(projectMembers))
	for _, person := range projectMembers {
		members[person.ID] = struct{}{}
	}
	available := make([]soda.Person, 0, len(people))
	for _, person := range people {
		if _, exists := members[person.ID]; !exists {
			available = append(available, person)
		}
	}
	return available
}

func memberWorkspaceViews(members []soda.Person, worktrees []soda.Worktree) []memberWorkspaceView {
	workspaceByPerson := make(map[string]soda.Worktree, len(worktrees))
	for _, worktree := range worktrees {
		workspaceByPerson[worktree.PersonID] = worktree
	}
	views := make([]memberWorkspaceView, 0, len(members))
	for _, person := range members {
		view := memberWorkspaceView{Person: person}
		if workspace, exists := workspaceByPerson[person.ID]; exists {
			workspaceCopy := workspace
			view.Workspace = &workspaceCopy
		}
		views = append(views, view)
	}
	return views
}

func validatePassword(password string) error {
	if strings.ContainsAny(password, "\r\n\x00") {
		return fmt.Errorf("password cannot contain line breaks")
	}
	if utf8.RuneCountInString(password) < 6 {
		return fmt.Errorf("password must be at least six characters")
	}
	return nil
}

func personalizedSSHConfig(project soda.Project, key soda.SSHDeviceKey) string {
	return fmt.Sprintf("Host soda-%s\n    HostName soda.local\n    User %s\n    IdentityFile %s\n    IdentitiesOnly yes\n", project.Slug, project.UnixUser, sshConfigValue(key.IdentityFileHint))
}

func personalizedSSHCommand(project soda.Project, key soda.SSHDeviceKey) string {
	return fmt.Sprintf("ssh -i %s %s@soda.local", shellPath(key.IdentityFileHint), project.UnixUser)
}

func sshConfigValue(value string) string {
	escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(value)
	return `"` + escaped + `"`
}

func shellPath(value string) string {
	if value == "~" {
		return `"$HOME"`
	}
	if strings.HasPrefix(value, "~/") {
		return `"$HOME"/` + shellQuote(strings.TrimPrefix(value, "~/"))
	}
	return shellQuote(value)
}

func shellQuote(value string) string {
	return `'` + strings.ReplaceAll(value, `'`, `'"'"'`) + `'`
}

func (s *Server) visibleProject(ctx context.Context, user soda.Person, projectID string) (soda.Project, bool, error) {
	projects, err := s.visibleProjects(ctx, user)
	if err != nil {
		return soda.Project{}, false, err
	}
	for _, project := range projects {
		if project.ID == projectID {
			return project, true, nil
		}
	}
	return soda.Project{}, false, nil
}

func (s *Server) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookie)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		person, ok := s.sessions.get(cookie.Value)
		if !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userContextKey{}, person)))
	})
}

func currentUser(r *http.Request) soda.Person {
	return r.Context().Value(userContextKey{}).(soda.Person)
}

func requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if currentUser(r).Role != soda.RoleAdmin {
		http.Error(w, "forbidden", http.StatusForbidden)
		return false
	}
	return true
}

func personNames(people []soda.Person) map[string]string {
	names := make(map[string]string, len(people))
	for _, person := range people {
		names[person.ID] = person.DisplayName
	}
	return names
}

func (s *Server) render(w http.ResponseWriter, status int, name string, data pageData) {
	data.Admin = data.User.Role == soda.RoleAdmin
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := s.templates.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "render page", http.StatusInternalServerError)
	}
}

func redirect(w http.ResponseWriter, r *http.Request, location string) {
	if isHTMX(r) {
		w.Header().Set("HX-Redirect", location)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, location, http.StatusSeeOther)
}

func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

func humanBytes(value uint64) string {
	const unit = uint64(1024)
	if value < unit {
		return fmt.Sprintf("%d B", value)
	}
	divisor := unit
	exponent := 0
	for remaining := value / unit; remaining >= unit && exponent < 5; remaining /= unit {
		divisor *= unit
		exponent++
	}
	return fmt.Sprintf("%.1f %ciB", float64(value)/float64(divisor), "KMGTPE"[exponent])
}

func humanDuration(seconds uint64) string {
	duration := time.Duration(seconds) * time.Second
	if duration >= 24*time.Hour {
		return fmt.Sprintf("%dd %dh", int(duration/(24*time.Hour)), int(duration/time.Hour)%24)
	}
	return duration.Round(time.Minute).String()
}

func humanTime(seconds uint64) string {
	if seconds == 0 {
		return "unknown"
	}
	return time.Unix(int64(seconds), 0).Format("2006-01-02 15:04:05")
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

func (s *sessionStore) create(person soda.Person) (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(bytes)
	s.mu.Lock()
	s.byToken[token] = person
	s.mu.Unlock()
	return token, nil
}

func (s *sessionStore) get(token string) (soda.Person, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	person, ok := s.byToken[token]
	return person, ok
}

func (s *sessionStore) remove(token string) {
	s.mu.Lock()
	delete(s.byToken, token)
	s.mu.Unlock()
}
