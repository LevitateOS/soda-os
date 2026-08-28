package web

import (
	"context"

	"github.com/LevitateOS/soda-os/cockpit/internal/daemonclient"
)

type projectListView struct {
	ProjectCards  []projectCardView
	ProjectsError string
}

type projectsView struct {
	pageIdentity
	projectListView
	People []daemonclient.Person
	Error  string
}

type projectStateView struct {
	Label  string
	Class  string
	Ready  bool
	Active bool
}

type provisioningView struct {
	Project   daemonclient.Project
	Admin     bool
	State     projectStateView
	Jobs      []daemonclient.ProvisioningJob
	Toolchain *daemonclient.ToolchainInstallation
	Error     string
}

type collaborationView struct {
	Project   daemonclient.Project
	User      daemonclient.Person
	Admin     bool
	Members   []memberWorkspaceView
	Available []daemonclient.Person
	Ready     bool
	Message   string
	Error     string
}

type connectView struct {
	Project     daemonclient.Project
	User        daemonclient.Person
	State       projectStateView
	DeviceKeys  []daemonclient.SSHDeviceKey
	SelectedKey *daemonclient.SSHDeviceKey
	Workspace   *daemonclient.Worktree
	SSHConfig   string
	SSHCommand  string
	Error       string
}

type projectView struct {
	pageIdentity
	Project       daemonclient.Project
	State         projectStateView
	Connection    connectView
	Provisioning  provisioningView
	Collaboration collaborationView
	DeployKey     string
}

type profilesView struct {
	pageIdentity
	Projects []daemonclient.Project
}

type projectCardView struct {
	Project    daemonclient.Project
	State      string
	StateClass string
}

type memberWorkspaceView struct {
	Person    daemonclient.Person
	Workspace *daemonclient.Worktree
}

func (s *Server) visibleProjects(ctx context.Context, user daemonclient.Person) ([]daemonclient.Project, error) {
	if user.Role == daemonclient.RoleAdmin {
		return s.projectAPI.Projects(ctx)
	}
	return s.projectAPI.ProjectsForPerson(ctx, user.ID)
}

func (s *Server) provisioningState(ctx context.Context, projectID string) ([]daemonclient.ProvisioningJob, *daemonclient.ToolchainInstallation, error) {
	jobs, err := s.projectAPI.Jobs(ctx, projectID)
	if err != nil {
		return nil, nil, err
	}
	installation, err := s.projectAPI.Toolchain(ctx, projectID)
	if err != nil {
		return nil, nil, err
	}
	return jobs, installation, nil
}

func provisioningActive(jobs []daemonclient.ProvisioningJob) bool {
	for _, job := range jobs {
		if job.State == "installing" {
			return true
		}
	}
	return false
}

func projectState(jobs []daemonclient.ProvisioningJob) (string, string) {
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

func (s *Server) projectCards(ctx context.Context, projects []daemonclient.Project) ([]projectCardView, error) {
	cards := make([]projectCardView, 0, len(projects))
	for _, project := range projects {
		jobs, loadErr := s.projectAPI.Jobs(ctx, project.ID)
		if loadErr != nil {
			return nil, loadErr
		}
		state, stateClass := projectState(jobs)
		cards = append(cards, projectCardView{Project: project, State: state, StateClass: stateClass})
	}
	return cards, nil
}

func peopleWithoutMembers(people, projectMembers []daemonclient.Person) []daemonclient.Person {
	members := make(map[string]struct{}, len(projectMembers))
	for _, person := range projectMembers {
		members[person.ID] = struct{}{}
	}
	available := make([]daemonclient.Person, 0, len(people))
	for _, person := range people {
		if _, exists := members[person.ID]; !exists {
			available = append(available, person)
		}
	}
	return available
}

func memberWorkspaceViews(members []daemonclient.Person, worktrees []daemonclient.Worktree) []memberWorkspaceView {
	workspaceByPerson := make(map[string]daemonclient.Worktree, len(worktrees))
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

func (s *Server) visibleProject(ctx context.Context, user daemonclient.Person, projectID string) (daemonclient.Project, bool, error) {
	projects, err := s.visibleProjects(ctx, user)
	if err != nil {
		return daemonclient.Project{}, false, err
	}
	for _, project := range projects {
		if project.ID == projectID {
			return project, true, nil
		}
	}
	return daemonclient.Project{}, false, nil
}
