package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/LevitateOS/soda-os/internal/domain"
	sodav2 "github.com/LevitateOS/soda-os/internal/gen/soda/v2"
	"github.com/LevitateOS/soda-os/internal/host"
	"github.com/LevitateOS/soda-os/internal/store"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *Service) StartProvisioning(ctx context.Context, request *sodav2.StartProvisioningRequest) (*sodav2.StartProvisioningResponse, error) {
	id, err := parseID(request.GetProjectId(), "project")
	if err != nil {
		return nil, rpcError(err)
	}
	if _, err = s.store.Project(ctx, id); err != nil {
		return nil, rpcError(err)
	}
	job, err := s.startProvisioning(id)
	if err != nil {
		return nil, rpcError(err)
	}
	return &sodav2.StartProvisioningResponse{Job: jobProto(job)}, nil
}
func (s *Service) ListProvisioningJobs(ctx context.Context, request *sodav2.ListProvisioningJobsRequest) (*sodav2.ListProvisioningJobsResponse, error) {
	id, err := parseID(request.GetProjectId(), "project")
	if err != nil {
		return nil, rpcError(err)
	}
	if _, err = s.store.Project(ctx, id); err != nil {
		return nil, rpcError(err)
	}
	jobs, err := s.store.Jobs(ctx, id)
	if err != nil {
		return nil, rpcError(err)
	}
	response := &sodav2.ListProvisioningJobsResponse{Jobs: make([]*sodav2.ProvisioningJob, 0, len(jobs))}
	for _, job := range jobs {
		response.Jobs = append(response.Jobs, jobProto(job))
	}
	return response, nil
}

func (s *Service) GetHostStatus(ctx context.Context, _ *sodav2.GetHostStatusRequest) (*sodav2.GetHostStatusResponse, error) {
	if s.telemetry == nil {
		return nil, status.Error(codes.Unavailable, "host telemetry is unavailable")
	}
	hostStatus, err := s.telemetry.HostStatus(ctx)
	if err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}
	return &sodav2.GetHostStatusResponse{Host: hostStatus}, nil
}
func (s *Service) startProvisioning(projectID string) (domain.ProvisioningJob, error) {
	job := domain.ProvisioningJob{ID: uuid.NewString(), ProjectID: projectID, State: domain.JobInstalling}
	if err := s.store.BeginProvisioning(s.provisioning.background, job); err != nil {
		return domain.ProvisioningJob{}, err
	}
	s.provisioning.wg.Add(1)
	go func() { defer s.provisioning.wg.Done(); s.runProvisioning(projectID, job.ID) }()
	return job, nil
}
func (s *Service) runProvisioning(projectID, jobID string) {
	ctx, cancel := context.WithTimeout(s.provisioning.background, s.provisioning.timeout)
	defer cancel()
	job := domain.ProvisioningJob{ID: jobID, ProjectID: projectID, State: domain.JobReady}
	if err := s.installProject(ctx, projectID); err != nil {
		message := err.Error()
		job.State = domain.JobFailed
		job.Error = &message
	}
	updateContext, updateCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer updateCancel()
	if err := s.store.UpdateJob(updateContext, job); err != nil {
		s.logger.Error("update provisioning job", slog.String("job_id", jobID), slog.Any("error", err))
	}
}

func (s *Service) compensate(ctx context.Context, operationErr error, cleanup host.Cleanup, resource, identity string) error {
	if cleanupErr := s.runCleanup(ctx, cleanup); cleanupErr != nil {
		s.logger.Error("creation cleanup failed", slog.String("resource", resource), slog.String("identity", identity), slog.Any("error", cleanupErr))
		return errors.Join(operationErr, fmt.Errorf("cleanup failed: %w", cleanupErr))
	}
	return operationErr
}

func (s *Service) runCleanup(ctx context.Context, cleanup host.Cleanup) error {
	if cleanup == nil {
		return nil
	}
	cleanupContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()
	return cleanup(cleanupContext)
}
func (s *Service) installProject(ctx context.Context, projectID string) error {
	project, err := s.store.Project(ctx, projectID)
	if err != nil {
		return err
	}
	baseRef, err := s.ensureProjectPrerequisites(ctx, project)
	if err != nil {
		return err
	}
	if err = s.provisionProjectWorkspaces(ctx, project, baseRef); err != nil {
		return err
	}
	return s.reconcileProjectAccess(ctx, projectID)
}

func (s *Service) ensureProjectPrerequisites(ctx context.Context, project domain.Project) (string, error) {
	if err := s.ensureProjectRepository(ctx, project); err != nil {
		return "", err
	}
	baseRef, err := s.host.DefaultBranch(ctx, project)
	if err != nil {
		return "", err
	}
	installation, _, err := s.store.ProjectInstallation(ctx, project.ID)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			return "", err
		}
		installation, err = s.toolchains.Install(ctx, project.Profile)
		if err != nil {
			return "", err
		}
		installation.ID = uuid.NewString()
		if _, err = s.store.SaveInstallation(ctx, project.ID, installation); err != nil {
			return "", err
		}
	}
	if err = s.host.WriteProjectEnvironment(ctx, project, filepath.Join(installation.Path, "environment.json")); err != nil {
		return "", err
	}
	return baseRef, nil
}

func (s *Service) ensureProjectRepository(ctx context.Context, project domain.Project) error {
	if err := s.host.EnsureRepository(ctx, project); err != nil {
		return err
	}
	return s.ensureBuiltInGitProject(ctx, project)
}

func (s *Service) provisionProjectWorkspaces(ctx context.Context, project domain.Project, baseRef string) error {
	members, err := s.store.Collaborators(ctx, project.ID)
	if err != nil {
		return err
	}
	for _, person := range members {
		if err := s.ensurePersonalWorkspace(ctx, project, person, baseRef); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ensurePersonalWorkspace(ctx context.Context, project domain.Project, person domain.Person, baseRef string) error {
	trees, err := s.store.WorktreesForPerson(ctx, project.ID, person.ID)
	if err != nil || len(trees) == 1 {
		return err
	}
	if len(trees) != 0 {
		return fmt.Errorf("person %s has multiple personal workspaces", person.Username)
	}
	tree := s.makeWorktree(project, person)
	if err = s.store.PreflightWorktree(ctx, tree); err != nil {
		return err
	}
	cleanup, err := s.host.CreateWorktree(ctx, project, person, tree, baseRef)
	if err != nil {
		return err
	}
	if err = s.store.CreateWorktree(ctx, tree); err != nil {
		return s.compensate(ctx, err, cleanup, "personal workspace", tree.Path)
	}
	return nil
}
func (s *Service) makeWorktree(project domain.Project, person domain.Person) domain.Worktree {
	branch := "people/" + person.Username
	path := filepath.Join(s.projectsRoot, project.Slug, "worktrees", person.Username)
	return domain.Worktree{ID: uuid.NewString(), ProjectID: project.ID, PersonID: person.ID, Name: "default", Branch: branch, Path: path}
}

func (s *Service) initialPeople(ctx context.Context, requested []string) ([]string, error) {
	if len(requested) == 0 {
		return nil, invalid("at least one initial project member is required")
	}
	seen := make(map[string]struct{}, len(requested))
	people := make([]string, 0, len(requested))
	for _, value := range requested {
		personID, err := parseID(value, "person")
		if err != nil {
			return nil, err
		}
		if _, exists := seen[personID]; exists {
			return nil, invalid("initial project members must be unique")
		}
		if _, err = s.store.Person(ctx, personID); err != nil {
			return nil, err
		}
		seen[personID] = struct{}{}
		people = append(people, personID)
	}
	return people, nil
}

func (s *Service) requireProjectReady(ctx context.Context, projectID string) error {
	jobs, err := s.store.Jobs(ctx, projectID)
	if err != nil {
		return err
	}
	if len(jobs) == 0 || jobs[0].State != domain.JobReady {
		return precondition("project setup must be ready before adding a team member")
	}
	return nil
}

func (s *Service) reconcilePersonAccess(ctx context.Context, personID string) error {
	projects, err := s.store.ProjectsForPerson(ctx, personID)
	if err != nil {
		return err
	}
	for _, project := range projects {
		if err = s.reconcileProjectAccess(ctx, project.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) reconcileProjectAccess(ctx context.Context, projectID string) error {
	project, err := s.store.Project(ctx, projectID)
	if err != nil {
		return err
	}
	people, err := s.store.Collaborators(ctx, projectID)
	if err != nil {
		return err
	}
	access := make([]domain.ProjectAccess, 0, len(people))
	for _, person := range people {
		value, included, accessErr := s.projectAccessForPerson(ctx, projectID, person)
		if accessErr != nil {
			return accessErr
		}
		if included {
			access = append(access, value)
		}
	}
	return s.host.ReconcileAuthorizedKeys(ctx, project, access)
}

func (s *Service) projectAccessForPerson(ctx context.Context, projectID string, person domain.Person) (domain.ProjectAccess, bool, error) {
	trees, err := s.store.WorktreesForPerson(ctx, projectID, person.ID)
	if err != nil {
		return domain.ProjectAccess{}, false, err
	}
	if len(trees) == 0 {
		return domain.ProjectAccess{}, false, nil
	}
	if len(trees) != 1 {
		return domain.ProjectAccess{}, false, fmt.Errorf("person %s has multiple personal workspaces", person.Username)
	}
	keys, err := s.store.SSHDeviceKeys(ctx, person.ID)
	if err != nil {
		return domain.ProjectAccess{}, false, err
	}
	return domain.ProjectAccess{Person: person, Worktree: trees[0], Keys: keys}, true, nil
}

func (s *Service) ReconcileAllAuthorizedKeys(ctx context.Context) error {
	projects, err := s.store.Projects(ctx)
	if err != nil {
		return err
	}
	for _, project := range projects {
		if err = s.reconcileProjectAccess(ctx, project.ID); err != nil {
			return err
		}
	}
	return nil
}
