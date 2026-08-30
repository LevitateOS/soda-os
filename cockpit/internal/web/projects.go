package web

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/LevitateOS/soda-os/cockpit/internal/daemonclient"
)

func (s *Server) projects(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	projects, err := s.visibleProjects(r.Context(), user)
	if err != nil {
		http.Error(w, "load projects", http.StatusBadGateway)
		return
	}
	data := projectsView{pageIdentity: pageIdentity{Title: "Projects · Soda OS", User: user}}
	data.ProjectCards, err = s.projectCards(r.Context(), projects)
	if err != nil {
		data.ProjectsError = "Projects are temporarily unavailable."
	}
	if user.Role == daemonclient.RoleAdmin {
		data.People, err = s.accounts.People(r.Context())
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
	source, message := projectSource(r)
	if message != "" {
		s.renderProjectCreateError(w, r, message)
		return
	}
	user := currentUser(r)
	members, bootstrap := initialProjectPeople(source, user.ID, r.Form["member_ids"])
	project, err := s.projectAPI.CreateProject(r.Context(), daemonclient.CreateProjectRequest{
		Slug: r.FormValue("slug"), Name: r.FormValue("name"), Profile: r.FormValue("profile"), Source: source, InitialPersonIDs: members, BootstrapPersonID: bootstrap,
	})
	if err != nil {
		if isHTMX(r) {
			people, _ := s.accounts.People(r.Context())
			s.render(w, http.StatusUnprocessableEntity, "project-create", projectsView{pageIdentity: pageIdentity{User: currentUser(r)}, People: people, Error: err.Error()})
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	redirect(w, r, "/projects/"+project.ID)
}

func projectSource(r *http.Request) (daemonclient.ProjectSource, string) {
	switch r.FormValue("repository_source") {
	case "built_in":
		return daemonclient.EmptyProjectSource{}, ""
	case "external":
		remote := strings.TrimSpace(r.FormValue("remote_url"))
		if remote == "" {
			return nil, "Existing repository address is required."
		}
		return daemonclient.GitProjectSource{RemoteURL: remote}, ""
	default:
		return nil, "Choose a repository source."
	}
}

func initialProjectPeople(source daemonclient.ProjectSource, creatorID string, requested []string) ([]string, string) {
	members := append([]string(nil), requested...)
	if _, external := source.(daemonclient.GitProjectSource); !external {
		return members, ""
	}
	for _, personID := range members {
		if personID == creatorID {
			return members, creatorID
		}
	}
	return append(members, creatorID), creatorID
}

func (s *Server) renderProjectCreateError(w http.ResponseWriter, r *http.Request, message string) {
	if isHTMX(r) {
		people, _ := s.accounts.People(r.Context())
		s.render(w, http.StatusUnprocessableEntity, "project-create", projectsView{pageIdentity: pageIdentity{User: currentUser(r)}, People: people, Error: message})
		return
	}
	http.Error(w, message, http.StatusBadRequest)
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
	data, worktrees, err := s.projectPageData(r.Context(), user, project)
	if err != nil {
		http.Error(w, "load project details", http.StatusBadGateway)
		return
	}
	s.addConnectionKeys(r.Context(), &data.Connection, worktrees)
	if err = s.addProjectRepositoryData(r.Context(), user, &data); err != nil {
		http.Error(w, "load repository identity", http.StatusBadGateway)
		return
	}
	s.render(w, http.StatusOK, "project.html", data)
}

func (s *Server) projectPageData(ctx context.Context, user daemonclient.Person, project daemonclient.Project) (projectView, []daemonclient.Worktree, error) {
	worktrees, err := s.projectAPI.Worktrees(ctx, project.ID)
	if err != nil {
		return projectView{}, nil, err
	}
	jobs, installation, err := s.provisioningState(ctx, project.ID)
	if err != nil {
		return projectView{}, nil, err
	}
	people, err := s.accounts.People(ctx)
	if err != nil {
		return projectView{}, nil, err
	}
	members, err := s.projectAPI.Members(ctx, project.ID)
	if err != nil {
		return projectView{}, nil, err
	}
	state, class := projectState(project, jobs)
	viewState := projectStateView{Label: state, Class: class, Ready: class == "ready", Active: provisioningActive(jobs)}
	data := projectView{
		pageIdentity:  pageIdentity{Title: "Project · Soda OS", User: user},
		Project:       project,
		State:         viewState,
		Provisioning:  provisioningView{Project: project, Admin: user.Role == daemonclient.RoleAdmin, State: viewState, Jobs: jobs, Toolchain: installation},
		Collaboration: collaborationView{Project: project, User: user, Admin: user.Role == daemonclient.RoleAdmin, Members: memberWorkspaceViews(members, worktrees), Available: peopleWithoutMembers(people, members), Ready: viewState.Ready},
		Connection:    connectView{Project: project, User: user, State: viewState},
	}
	return data, worktrees, nil
}

func (s *Server) addProjectRepositoryData(ctx context.Context, user daemonclient.Person, data *projectView) error {
	if data.State.Class == "waiting" {
		identity, err := s.accounts.GitIdentity(ctx, data.Project.BootstrapPersonID)
		if err != nil {
			return err
		}
		data.Provisioning.RepositoryIdentity = &identity
	}
	if user.Role != daemonclient.RoleAdmin {
		return nil
	}
	if _, builtIn := data.Project.Source.(daemonclient.EmptyProjectSource); !builtIn {
		return nil
	}
	key, err := s.projectAPI.DeployKey(ctx, data.Project.ID)
	data.DeployKey = key
	return err
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
	data := connectView{Project: project, User: user}
	if err = s.loadConnectData(r.Context(), &data, r.URL.Query().Get("key_id")); err != nil {
		data.Error = "Connection details are temporarily unavailable."
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
	data := connectView{Project: project, User: user}
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

func (s *Server) addConnectionKeys(ctx context.Context, data *connectView, worktrees []daemonclient.Worktree) {
	keys, err := s.accounts.SSHDeviceKeys(ctx, data.User.ID)
	if err != nil {
		data.Error = "Connection details are temporarily unavailable."
		return
	}
	if err := populateConnectData(data, worktrees, keys, "", s.address); err != nil {
		data.Error = "Connection details are temporarily unavailable."
	}
}

func (s *Server) loadConnectData(ctx context.Context, data *connectView, selectedKeyID string) error {
	worktrees, err := s.projectAPI.Worktrees(ctx, data.Project.ID)
	if err != nil {
		return err
	}
	keys, err := s.accounts.SSHDeviceKeys(ctx, data.User.ID)
	if err != nil {
		return err
	}
	jobs, _, err := s.provisioningState(ctx, data.Project.ID)
	if err != nil {
		return err
	}
	state, class := projectState(data.Project, jobs)
	data.State = projectStateView{Label: state, Class: class, Ready: class == "ready"}
	return populateConnectData(data, worktrees, keys, selectedKeyID, s.address)
}

func populateConnectData(data *connectView, worktrees []daemonclient.Worktree, keys []daemonclient.SSHDeviceKey, selectedKeyID, address string) error {
	data.DeviceKeys = keys
	data.Workspace = nil
	for i := range worktrees {
		if worktrees[i].PersonID == data.User.ID {
			workspace := worktrees[i]
			data.Workspace = &workspace
			break
		}
	}
	key, err := selectDeviceKey(keys, selectedKeyID)
	if err != nil {
		return err
	}
	data.SelectedKey = key
	if data.State.Ready && data.Workspace != nil && data.SelectedKey != nil {
		data.SSHConfig = personalizedSSHConfig(data.Project, data.User, *data.SelectedKey, address)
		data.SSHCommand = personalizedSSHCommand(data.Project, data.User, *data.SelectedKey, address)
	}
	return nil
}

func selectDeviceKey(keys []daemonclient.SSHDeviceKey, selectedKeyID string) (*daemonclient.SSHDeviceKey, error) {
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

func (s *Server) collaborationData(w http.ResponseWriter, r *http.Request) (collaborationView, bool) {
	user := currentUser(r)
	project, allowed, err := s.visibleProject(r.Context(), user, r.PathValue("project_id"))
	if err != nil {
		http.Error(w, "load project", http.StatusBadGateway)
		return collaborationView{}, false
	}
	if !allowed {
		http.Error(w, "forbidden", http.StatusForbidden)
		return collaborationView{}, false
	}
	worktrees, err := s.projectAPI.Worktrees(r.Context(), project.ID)
	if err != nil {
		http.Error(w, "load worktrees", http.StatusBadGateway)
		return collaborationView{}, false
	}
	people, err := s.accounts.People(r.Context())
	if err != nil {
		http.Error(w, "load people", http.StatusBadGateway)
		return collaborationView{}, false
	}
	members, err := s.projectAPI.Members(r.Context(), project.ID)
	if err != nil {
		http.Error(w, "load project members", http.StatusBadGateway)
		return collaborationView{}, false
	}
	data := collaborationView{Project: project, User: user, Admin: user.Role == daemonclient.RoleAdmin, Members: memberWorkspaceViews(members, worktrees), Available: peopleWithoutMembers(people, members)}
	jobs, _, setupErr := s.provisioningState(r.Context(), project.ID)
	if setupErr == nil {
		_, stateClass := projectState(project, jobs)
		data.Ready = stateClass == "ready"
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
	state, class := projectState(project, jobs)
	data := provisioningView{Project: project, Admin: user.Role == daemonclient.RoleAdmin, State: projectStateView{Label: state, Class: class, Active: provisioningActive(jobs)}, Jobs: jobs, Toolchain: installation}
	if class == "waiting" {
		identity, identityErr := s.accounts.GitIdentity(r.Context(), project.BootstrapPersonID)
		if identityErr != nil {
			http.Error(w, "load repository identity", http.StatusBadGateway)
			return
		}
		data.RepositoryIdentity = &identity
	}
	s.render(w, http.StatusOK, "provisioning", data)
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
	command := daemonclient.AddCollaboratorCommand{ProjectID: projectID, PersonID: r.FormValue("person_id")}
	commandErr := s.projectAPI.AddCollaborator(r.Context(), command)
	s.respondToCollaboratorCommand(w, r, projectID, commandErr)
}

func (s *Server) respondToCollaboratorCommand(w http.ResponseWriter, r *http.Request, projectID string, commandErr error) {
	if !isHTMX(r) {
		if commandErr != nil {
			http.Error(w, commandErr.Error(), http.StatusBadRequest)
			return
		}
		redirect(w, r, "/projects/"+projectID)
		return
	}
	data, ok := s.collaborationData(w, r)
	if !ok {
		return
	}
	if commandErr != nil {
		data.Error = commandErr.Error()
		s.render(w, http.StatusUnprocessableEntity, "collaboration", data)
		return
	}
	data.Message = "Team member and personal workspace added."
	s.render(w, http.StatusOK, "collaboration", data)
}

func (s *Server) retryProvisioning(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	projectID := r.PathValue("project_id")
	if err := s.projectAPI.RetryProvisioning(r.Context(), projectID); err != nil {
		if isHTMX(r) {
			user := currentUser(r)
			project, allowed, loadErr := s.visibleProject(r.Context(), user, projectID)
			if loadErr != nil || !allowed {
				http.Error(w, "load project", http.StatusBadGateway)
				return
			}
			jobs, installation, _ := s.provisioningState(r.Context(), projectID)
			state, class := projectState(project, jobs)
			s.render(w, http.StatusUnprocessableEntity, "provisioning", provisioningView{Project: project, Admin: user.Role == daemonclient.RoleAdmin, State: projectStateView{Label: state, Class: class, Active: provisioningActive(jobs)}, Jobs: jobs, Toolchain: installation, Error: err.Error()})
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
	s.render(w, http.StatusOK, "profiles.html", profilesView{pageIdentity: pageIdentity{Title: "Development environments · Soda OS", User: user}, Projects: projects})
}
