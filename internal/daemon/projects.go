package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/LevitateOS/soda-os/internal/domain"
	sodav2 "github.com/LevitateOS/soda-os/internal/gen/soda/v2"
	"github.com/LevitateOS/soda-os/internal/host"
	"github.com/LevitateOS/soda-os/internal/store"
	"github.com/google/uuid"
)

func (s *Service) CreateProject(ctx context.Context, request *sodav2.CreateProjectRequest) (*sodav2.CreateProjectResponse, error) {
	project, personIDs, err := s.projectRequest(ctx, request)
	if err != nil {
		return nil, rpcError(err)
	}
	if err := s.store.PreflightProject(ctx, project.Slug, project.UnixUser); err != nil {
		return nil, rpcError(err)
	}
	cleanup, err := s.host.CreateProject(ctx, project)
	if err != nil {
		return nil, rpcError(err)
	}
	if err = s.store.CreateProjectWithMemberships(ctx, project, personIDs); err != nil {
		return nil, rpcError(s.compensate(ctx, err, cleanup, "project", project.Slug))
	}
	if _, external := project.Source.(domain.GitProjectSource); external {
		return &sodav2.CreateProjectResponse{Project: projectProto(project)}, nil
	}
	if _, err = s.startProvisioning(project.ID); err != nil {
		cleanupErr := s.store.DeleteFreshProject(context.WithoutCancel(ctx), project.ID)
		if cleanupErr != nil {
			// The project is still durable, so its matching host account and
			// filesystem must remain intact. Removing them here would leave a
			// persisted project that can no longer be provisioned or accessed.
			s.logger.Error("clean up failed project database row", slog.String("project", project.Slug), slog.Any("error", cleanupErr))
			return nil, rpcError(errors.Join(err, fmt.Errorf("database cleanup failed; host resources preserved: %w", cleanupErr)))
		}
		if hostCleanupErr := s.runCleanup(ctx, cleanup); hostCleanupErr != nil {
			s.logger.Error("clean up failed project host resources", slog.String("project", project.Slug), slog.Any("error", hostCleanupErr))
			err = errors.Join(err, fmt.Errorf("host cleanup failed: %w", hostCleanupErr))
		}
		return nil, rpcError(err)
	}
	return &sodav2.CreateProjectResponse{Project: projectProto(project)}, nil
}

func (s *Service) projectRequest(ctx context.Context, request *sodav2.CreateProjectRequest) (domain.Project, []string, error) {
	if err := validateUsername(request.GetSlug()); err != nil {
		return domain.Project{}, nil, err
	}
	if strings.TrimSpace(request.GetName()) == "" {
		return domain.Project{}, nil, invalid("project name is required")
	}
	profile, err := profileDomain(request.GetProfile())
	if err != nil {
		return domain.Project{}, nil, err
	}
	source, err := sourceDomain(request.GetSource())
	if err != nil {
		return domain.Project{}, nil, err
	}
	personIDs, err := s.initialPeople(ctx, request.GetInitialPersonIds())
	if err != nil {
		return domain.Project{}, nil, err
	}
	bootstrap, err := s.bootstrapPerson(ctx, source, request.GetBootstrapPersonId(), personIDs)
	if err != nil {
		return domain.Project{}, nil, err
	}
	return domain.Project{ID: uuid.NewString(), Slug: request.GetSlug(), Name: request.GetName(), UnixUser: "soda-p-" + request.GetSlug(), Profile: profile, Source: source, BootstrapPersonID: bootstrap}, personIDs, nil
}

func (s *Service) bootstrapPerson(ctx context.Context, source domain.ProjectSource, requested string, personIDs []string) (string, error) {
	if _, external := source.(domain.GitProjectSource); !external {
		if requested != "" {
			return "", invalid("bootstrap person is only valid for an external repository")
		}
		return "", nil
	}
	bootstrap, err := parseID(requested, "bootstrap person")
	if err != nil {
		return "", err
	}
	for _, personID := range personIDs {
		if personID == bootstrap {
			_, err = s.store.GitIdentity(ctx, bootstrap)
			return bootstrap, err
		}
	}
	return "", invalid("bootstrap person must be an initial project member")
}

func (s *Service) ListProjects(ctx context.Context, _ *sodav2.ListProjectsRequest) (*sodav2.ListProjectsResponse, error) {
	values, err := s.store.Projects(ctx)
	if err != nil {
		return nil, rpcError(err)
	}
	response := &sodav2.ListProjectsResponse{Projects: make([]*sodav2.Project, 0, len(values))}
	for _, value := range values {
		response.Projects = append(response.Projects, projectProto(value))
	}
	return response, nil
}
func (s *Service) ListProjectsForPerson(ctx context.Context, request *sodav2.ListProjectsForPersonRequest) (*sodav2.ListProjectsForPersonResponse, error) {
	id, err := parseID(request.GetPersonId(), "person")
	if err != nil {
		return nil, rpcError(err)
	}
	if _, err = s.store.Person(ctx, id); err != nil {
		return nil, rpcError(err)
	}
	values, err := s.store.ProjectsForPerson(ctx, id)
	if err != nil {
		return nil, rpcError(err)
	}
	response := &sodav2.ListProjectsForPersonResponse{Projects: make([]*sodav2.Project, 0, len(values))}
	for _, value := range values {
		response.Projects = append(response.Projects, projectProto(value))
	}
	return response, nil
}

func (s *Service) AddCollaborator(ctx context.Context, request *sodav2.AddCollaboratorRequest) (*sodav2.AddCollaboratorResponse, error) {
	project, person, err := s.collaboratorAdmission(ctx, request)
	if err != nil {
		return nil, rpcError(err)
	}
	workspace, err := s.createCollaboratorWorkspace(ctx, project, person)
	if err != nil {
		return nil, rpcError(err)
	}
	if err = s.ensureBuiltInGitProject(ctx, project); err != nil {
		rollbackErr := s.store.DeleteMembershipAndWorktree(context.WithoutCancel(ctx), workspace.membership.ProjectID, workspace.membership.PersonID)
		if cleanupErr := s.runCleanup(ctx, workspace.cleanup); cleanupErr != nil {
			rollbackErr = errors.Join(rollbackErr, cleanupErr)
		}
		return nil, rpcError(errors.Join(err, rollbackErr))
	}
	if err = s.reconcileProjectAccess(ctx, workspace.membership.ProjectID); err != nil {
		rollbackErr := s.store.DeleteMembershipAndWorktree(context.WithoutCancel(ctx), workspace.membership.ProjectID, workspace.membership.PersonID)
		if cleanupErr := s.runCleanup(ctx, workspace.cleanup); cleanupErr != nil {
			rollbackErr = errors.Join(rollbackErr, cleanupErr)
		}
		if reconcileErr := s.reconcileProjectAccess(context.WithoutCancel(ctx), workspace.membership.ProjectID); reconcileErr != nil {
			rollbackErr = errors.Join(rollbackErr, reconcileErr)
		}
		err = errors.Join(err, rollbackErr)
		return nil, rpcError(err)
	}
	return &sodav2.AddCollaboratorResponse{Membership: membershipProto(workspace.membership), Worktree: worktreeProto(workspace.worktree)}, nil
}

func (s *Service) collaboratorAdmission(ctx context.Context, request *sodav2.AddCollaboratorRequest) (domain.Project, domain.Person, error) {
	projectID, err := parseID(request.GetProjectId(), "project")
	if err != nil {
		return domain.Project{}, domain.Person{}, err
	}
	personID, err := parseID(request.GetPersonId(), "person")
	if err != nil {
		return domain.Project{}, domain.Person{}, err
	}
	if _, err = s.store.Membership(ctx, projectID, personID); err == nil {
		return domain.Project{}, domain.Person{}, fmt.Errorf("%w: membership", store.ErrAlreadyExists)
	} else if !errors.Is(err, store.ErrNotFound) {
		return domain.Project{}, domain.Person{}, err
	}
	project, err := s.store.Project(ctx, projectID)
	if err != nil {
		return domain.Project{}, domain.Person{}, err
	}
	person, err := s.store.Person(ctx, personID)
	if err != nil {
		return domain.Project{}, domain.Person{}, err
	}
	if err = s.requireProjectReady(ctx, projectID); err != nil {
		return domain.Project{}, domain.Person{}, err
	}
	return project, person, nil
}

type collaboratorWorkspace struct {
	membership domain.Membership
	worktree   domain.Worktree
	cleanup    host.Cleanup
}

func (s *Service) createCollaboratorWorkspace(ctx context.Context, project domain.Project, person domain.Person) (collaboratorWorkspace, error) {
	baseRef, err := s.host.DefaultBranch(ctx, project)
	if err != nil {
		return collaboratorWorkspace{}, err
	}
	tree := s.makeWorktree(project, person)
	if err = s.store.PreflightWorktree(ctx, tree); err != nil {
		return collaboratorWorkspace{}, err
	}
	cleanup, err := s.host.CreateWorktree(ctx, project, person, tree, baseRef)
	if err != nil {
		return collaboratorWorkspace{}, err
	}
	membership := domain.Membership{ProjectID: project.ID, PersonID: person.ID}
	if err = s.store.AddMembershipAndWorktree(ctx, membership, tree); err != nil {
		return collaboratorWorkspace{}, s.compensate(ctx, err, cleanup, "collaborator worktree", tree.Path)
	}
	return collaboratorWorkspace{membership: membership, worktree: tree, cleanup: cleanup}, nil
}

func (s *Service) ListCollaborators(ctx context.Context, request *sodav2.ListCollaboratorsRequest) (*sodav2.ListCollaboratorsResponse, error) {
	projectID, err := parseID(request.GetProjectId(), "project")
	if err != nil {
		return nil, rpcError(err)
	}
	if _, err = s.store.Project(ctx, projectID); err != nil {
		return nil, rpcError(err)
	}
	people, err := s.store.Collaborators(ctx, projectID)
	if err != nil {
		return nil, rpcError(err)
	}
	response := &sodav2.ListCollaboratorsResponse{Collaborators: make([]*sodav2.Collaborator, 0, len(people))}
	for _, person := range people {
		trees, treeErr := s.store.WorktreesForPerson(ctx, projectID, person.ID)
		if treeErr != nil {
			return nil, rpcError(treeErr)
		}
		protoTrees := make([]*sodav2.Worktree, 0, len(trees))
		for _, tree := range trees {
			protoTrees = append(protoTrees, worktreeProto(tree))
		}
		membership := domain.Membership{ProjectID: projectID, PersonID: person.ID}
		response.Collaborators = append(response.Collaborators, &sodav2.Collaborator{Person: personProto(person), Membership: membershipProto(membership), Worktrees: protoTrees})
	}
	return response, nil
}
func (s *Service) ListWorktrees(ctx context.Context, request *sodav2.ListWorktreesRequest) (*sodav2.ListWorktreesResponse, error) {
	projectID, err := parseID(request.GetProjectId(), "project")
	if err != nil {
		return nil, rpcError(err)
	}
	if _, err = s.store.Project(ctx, projectID); err != nil {
		return nil, rpcError(err)
	}
	values, err := s.store.Worktrees(ctx, projectID)
	if err != nil {
		return nil, rpcError(err)
	}
	response := &sodav2.ListWorktreesResponse{Worktrees: make([]*sodav2.Worktree, 0, len(values))}
	for _, value := range values {
		response.Worktrees = append(response.Worktrees, worktreeProto(value))
	}
	return response, nil
}

func (s *Service) GetDeployKey(ctx context.Context, request *sodav2.GetDeployKeyRequest) (*sodav2.GetDeployKeyResponse, error) {
	id, err := parseID(request.GetProjectId(), "project")
	if err != nil {
		return nil, rpcError(err)
	}
	project, err := s.store.Project(ctx, id)
	if err != nil {
		return nil, rpcError(err)
	}
	if _, builtIn := project.Source.(domain.EmptyProjectSource); !builtIn {
		return nil, rpcError(store.ErrNotFound)
	}
	key, err := s.host.DeployPublicKey(ctx, project)
	if err != nil {
		return nil, rpcError(err)
	}
	return &sodav2.GetDeployKeyResponse{DeployKey: &sodav2.DeployKey{ProjectId: id, PublicKey: key}}, nil
}
func (s *Service) GetProjectToolchain(ctx context.Context, request *sodav2.GetProjectToolchainRequest) (*sodav2.GetProjectToolchainResponse, error) {
	id, err := parseID(request.GetProjectId(), "project")
	if err != nil {
		return nil, rpcError(err)
	}
	if _, err = s.store.Project(ctx, id); err != nil {
		return nil, rpcError(err)
	}
	installation, resolution, err := s.store.ProjectInstallation(ctx, id)
	if err != nil {
		return nil, rpcError(err)
	}
	return &sodav2.GetProjectToolchainResponse{Installation: installationProto(installation), Resolution: resolutionProto(resolution)}, nil
}
