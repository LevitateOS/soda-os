package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/LevitateOS/soda-os/internal/domain"
	sodav2 "github.com/LevitateOS/soda-os/internal/gen/soda/v2"
	"github.com/LevitateOS/soda-os/internal/host"
	"github.com/LevitateOS/soda-os/internal/store"
	"github.com/LevitateOS/soda-os/internal/toolchain"
	"github.com/LevitateOS/soda-os/internal/version"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Telemetry interface {
	HostStatus(context.Context) (*sodav2.HostStatus, error)
}

type Service struct {
	sodav2.UnimplementedSodaServiceServer
	store               *store.Store
	host                host.Operations
	toolchains          toolchain.Installer
	telemetry           Telemetry
	projectsRoot        string
	logger              *slog.Logger
	background          context.Context
	cancel              context.CancelFunc
	wg                  sync.WaitGroup
	provisioningTimeout time.Duration
}

type Options struct {
	Store               *store.Store
	Host                host.Operations
	Toolchains          toolchain.Installer
	Telemetry           Telemetry
	ProjectsRoot        string
	Logger              *slog.Logger
	ProvisioningTimeout time.Duration
}

const defaultProvisioningTimeout = 30 * time.Minute

func New(options Options) *Service {
	background, cancel := context.WithCancel(context.Background())
	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}
	provisioningTimeout := options.ProvisioningTimeout
	if provisioningTimeout <= 0 {
		provisioningTimeout = defaultProvisioningTimeout
	}
	return &Service{store: options.Store, host: options.Host, toolchains: options.Toolchains, telemetry: options.Telemetry, projectsRoot: options.ProjectsRoot, logger: logger, background: background, cancel: cancel, provisioningTimeout: provisioningTimeout}
}
func (s *Service) Close() { s.cancel(); s.wg.Wait() }

func (s *Service) Health(_ context.Context, _ *sodav2.HealthRequest) (*sodav2.HealthResponse, error) {
	return &sodav2.HealthResponse{Status: "ok", Service: "sodad", Version: version.Version}, nil
}
func (s *Service) CreatePerson(ctx context.Context, request *sodav2.CreatePersonRequest) (*sodav2.CreatePersonResponse, error) {
	role, err := roleDomain(request.GetRole())
	if err != nil {
		return nil, rpcError(err)
	}
	if err = validatePerson(request.GetUsername(), request.GetDisplayName(), request.GetEmail()); err != nil {
		return nil, rpcError(err)
	}
	if request.GetPassword() == "" {
		return nil, rpcError(invalid("password is required"))
	}
	if utf8.RuneCountInString(request.GetPassword()) < 6 {
		return nil, rpcError(invalid("password must contain at least 6 characters"))
	}
	if strings.ContainsAny(request.GetPassword(), "\r\n\x00") {
		return nil, rpcError(invalid("password contains a line or NUL delimiter"))
	}
	if err = s.store.PreflightPerson(ctx, request.GetUsername()); err != nil {
		return nil, rpcError(err)
	}
	person := domain.Person{ID: uuid.NewString(), Username: request.GetUsername(), DisplayName: request.GetDisplayName(), Email: request.GetEmail(), Role: role}
	cleanup, err := s.host.CreatePerson(ctx, person, request.GetPassword())
	if err != nil {
		return nil, rpcError(err)
	}
	if err = s.store.CreatePerson(ctx, person); err != nil {
		return nil, rpcError(s.compensate(ctx, err, cleanup, "person", person.Username))
	}
	return &sodav2.CreatePersonResponse{Person: personProto(person)}, nil
}
func (s *Service) ImportPerson(ctx context.Context, request *sodav2.ImportPersonRequest) (*sodav2.ImportPersonResponse, error) {
	role, err := roleDomain(request.GetRole())
	if err != nil {
		return nil, rpcError(err)
	}
	if err = validatePerson(request.GetUsername(), request.GetDisplayName(), request.GetEmail()); err != nil {
		return nil, rpcError(err)
	}
	if err = s.store.PreflightPerson(ctx, request.GetUsername()); err != nil {
		return nil, rpcError(err)
	}
	person := domain.Person{ID: uuid.NewString(), Username: request.GetUsername(), DisplayName: request.GetDisplayName(), Email: request.GetEmail(), Role: role}
	cleanup, err := s.host.ImportPerson(ctx, person)
	if err != nil {
		return nil, rpcError(err)
	}
	if err = s.store.CreatePerson(ctx, person); err != nil {
		return nil, rpcError(s.compensate(ctx, err, cleanup, "imported person", person.Username))
	}
	return &sodav2.ImportPersonResponse{Person: personProto(person)}, nil
}
func (s *Service) ListPeople(ctx context.Context, _ *sodav2.ListPeopleRequest) (*sodav2.ListPeopleResponse, error) {
	values, err := s.store.People(ctx)
	if err != nil {
		return nil, rpcError(err)
	}
	response := &sodav2.ListPeopleResponse{People: make([]*sodav2.Person, 0, len(values))}
	for _, value := range values {
		response.People = append(response.People, personProto(value))
	}
	return response, nil
}

func (s *Service) CreateSshDeviceKey(ctx context.Context, request *sodav2.CreateSshDeviceKeyRequest) (*sodav2.CreateSshDeviceKeyResponse, error) {
	personID, err := parseID(request.GetPersonId(), "person")
	if err != nil {
		return nil, rpcError(err)
	}
	if _, err = s.store.Person(ctx, personID); err != nil {
		return nil, rpcError(err)
	}
	label := strings.TrimSpace(request.GetLabel())
	if label == "" || len(label) > 40 || strings.ContainsAny(label, "\r\n\x00") {
		return nil, rpcError(invalid("device label must contain 1 to 40 characters"))
	}
	hint := strings.TrimSpace(request.GetIdentityFileHint())
	if hint == "" || len(hint) > 255 || strings.ContainsAny(hint, "\r\n\x00") {
		return nil, rpcError(invalid("identity file path hint must be a single value of at most 255 characters"))
	}
	if err = host.ValidatePublicKey(request.GetPublicKey(), false); err != nil {
		return nil, rpcError(invalid("%v", err))
	}
	fingerprint, err := domain.SSHKeyFingerprint(request.GetPublicKey())
	if err != nil {
		return nil, rpcError(invalid("%v", err))
	}
	key := domain.SSHDeviceKey{ID: uuid.NewString(), PersonID: personID, Label: label, PublicKey: strings.TrimSpace(request.GetPublicKey()), Fingerprint: fingerprint, IdentityFileHint: hint, CreatedAt: time.Now().UTC()}
	if err = s.store.CreateSSHDeviceKey(ctx, key); err != nil {
		return nil, rpcError(err)
	}
	if err = s.reconcilePersonAccess(ctx, personID); err != nil {
		_, rollbackErr := s.store.DeleteSSHDeviceKey(context.WithoutCancel(ctx), personID, key.ID)
		if rollbackErr == nil {
			rollbackErr = s.reconcilePersonAccess(context.WithoutCancel(ctx), personID)
		}
		return nil, rpcError(errors.Join(err, rollbackErr))
	}
	return &sodav2.CreateSshDeviceKeyResponse{Key: sshDeviceKeyProto(key)}, nil
}

func (s *Service) ListSshDeviceKeys(ctx context.Context, request *sodav2.ListSshDeviceKeysRequest) (*sodav2.ListSshDeviceKeysResponse, error) {
	personID, err := parseID(request.GetPersonId(), "person")
	if err != nil {
		return nil, rpcError(err)
	}
	if _, err = s.store.Person(ctx, personID); err != nil {
		return nil, rpcError(err)
	}
	keys, err := s.store.SSHDeviceKeys(ctx, personID)
	if err != nil {
		return nil, rpcError(err)
	}
	response := &sodav2.ListSshDeviceKeysResponse{Keys: make([]*sodav2.SshDeviceKey, 0, len(keys))}
	for _, key := range keys {
		response.Keys = append(response.Keys, sshDeviceKeyProto(key))
	}
	return response, nil
}

func (s *Service) RevokeSshDeviceKey(ctx context.Context, request *sodav2.RevokeSshDeviceKeyRequest) (*sodav2.RevokeSshDeviceKeyResponse, error) {
	personID, err := parseID(request.GetPersonId(), "person")
	if err != nil {
		return nil, rpcError(err)
	}
	keyID, err := parseID(request.GetKeyId(), "SSH device key")
	if err != nil {
		return nil, rpcError(err)
	}
	key, err := s.store.DeleteSSHDeviceKey(ctx, personID, keyID)
	if err != nil {
		return nil, rpcError(err)
	}
	if err = s.reconcilePersonAccess(ctx, personID); err != nil {
		rollbackErr := s.store.CreateSSHDeviceKey(context.WithoutCancel(ctx), key)
		if rollbackErr == nil {
			rollbackErr = s.reconcilePersonAccess(context.WithoutCancel(ctx), personID)
		}
		return nil, rpcError(errors.Join(err, rollbackErr))
	}
	return &sodav2.RevokeSshDeviceKeyResponse{Key: sshDeviceKeyProto(key)}, nil
}

func (s *Service) CreateProject(ctx context.Context, request *sodav2.CreateProjectRequest) (*sodav2.CreateProjectResponse, error) {
	if err := validateSlug(request.GetSlug()); err != nil {
		return nil, rpcError(err)
	}
	if strings.TrimSpace(request.GetName()) == "" {
		return nil, rpcError(invalid("project name is required"))
	}
	profile, err := profileDomain(request.GetProfile())
	if err != nil {
		return nil, rpcError(err)
	}
	source, err := sourceDomain(request.GetSource())
	if err != nil {
		return nil, rpcError(err)
	}
	personIDs, err := s.initialPeople(ctx, request.GetInitialPersonIds())
	if err != nil {
		return nil, rpcError(err)
	}
	project := domain.Project{ID: uuid.NewString(), Slug: request.GetSlug(), Name: request.GetName(), UnixUser: "soda-p-" + request.GetSlug(), Profile: profile, Source: source}
	if err = s.store.PreflightProject(ctx, project.Slug, project.UnixUser); err != nil {
		return nil, rpcError(err)
	}
	cleanup, err := s.host.CreateProject(ctx, project)
	if err != nil {
		return nil, rpcError(err)
	}
	if err = s.store.CreateProjectWithMemberships(ctx, project, personIDs); err != nil {
		return nil, rpcError(s.compensate(ctx, err, cleanup, "project", project.Slug))
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
	projectID, personID, err := parsePair(request.GetProjectId(), request.GetPersonId())
	if err != nil {
		return nil, rpcError(err)
	}
	if _, err = s.store.Membership(ctx, projectID, personID); err == nil {
		return nil, rpcError(fmt.Errorf("%w: membership", store.ErrAlreadyExists))
	} else if !errors.Is(err, store.ErrNotFound) {
		return nil, rpcError(err)
	}
	project, err := s.store.Project(ctx, projectID)
	if err != nil {
		return nil, rpcError(err)
	}
	person, err := s.store.Person(ctx, personID)
	if err != nil {
		return nil, rpcError(err)
	}
	if err = s.requireProjectReady(ctx, projectID); err != nil {
		return nil, rpcError(err)
	}
	baseRef, err := s.host.DefaultBranch(ctx, project)
	if err != nil {
		return nil, rpcError(err)
	}
	tree := s.makeWorktree(project, person)
	if err = s.store.PreflightWorktree(ctx, tree); err != nil {
		return nil, rpcError(err)
	}
	cleanup, err := s.host.CreateWorktree(ctx, project, person, tree, baseRef)
	if err != nil {
		return nil, rpcError(err)
	}
	membership := domain.Membership{ProjectID: projectID, PersonID: personID}
	if err = s.store.AddMembershipAndWorktree(ctx, membership, tree); err != nil {
		return nil, rpcError(s.compensate(ctx, err, cleanup, "collaborator worktree", tree.Path))
	}
	if err = s.reconcileProjectAccess(ctx, projectID); err != nil {
		rollbackErr := s.store.DeleteMembershipAndWorktree(context.WithoutCancel(ctx), projectID, personID)
		if cleanupErr := s.runCleanup(ctx, cleanup); cleanupErr != nil {
			rollbackErr = errors.Join(rollbackErr, cleanupErr)
		}
		if reconcileErr := s.reconcileProjectAccess(context.WithoutCancel(ctx), projectID); reconcileErr != nil {
			rollbackErr = errors.Join(rollbackErr, reconcileErr)
		}
		return nil, rpcError(errors.Join(err, rollbackErr))
	}
	return &sodav2.AddCollaboratorResponse{Membership: membershipProto(membership), Worktree: worktreeProto(tree)}, nil
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
	if err := s.store.BeginProvisioning(s.background, job); err != nil {
		return domain.ProvisioningJob{}, err
	}
	s.wg.Add(1)
	go func() { defer s.wg.Done(); s.runProvisioning(projectID, job.ID) }()
	return job, nil
}
func (s *Service) runProvisioning(projectID, jobID string) {
	ctx, cancel := context.WithTimeout(s.background, s.provisioningTimeout)
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
	if err = s.host.EnsureRepository(ctx, project); err != nil {
		return err
	}
	baseRef, err := s.host.DefaultBranch(ctx, project)
	if err != nil {
		return err
	}
	installation, _, err := s.store.ProjectInstallation(ctx, projectID)
	if err == nil {
		if err = s.host.WriteProjectEnvironment(ctx, project, "source "+filepath.Join(installation.Path, "env")+"\n"); err != nil {
			return err
		}
	} else {
		if !errors.Is(err, store.ErrNotFound) {
			return err
		}
		installation, err = s.toolchains.Install(ctx, project.Profile)
		if err != nil {
			return err
		}
		installation.ID = uuid.NewString()
		if _, err = s.store.SaveInstallation(ctx, projectID, installation); err != nil {
			return err
		}
		if err = s.host.WriteProjectEnvironment(ctx, project, "source "+filepath.Join(installation.Path, "env")+"\n"); err != nil {
			return err
		}
	}
	members, err := s.store.Collaborators(ctx, projectID)
	if err != nil {
		return err
	}
	for _, person := range members {
		trees, treeErr := s.store.WorktreesForPerson(ctx, projectID, person.ID)
		if treeErr != nil {
			return treeErr
		}
		if len(trees) == 1 {
			continue
		}
		if len(trees) != 0 {
			return fmt.Errorf("person %s has multiple personal workspaces", person.Username)
		}
		tree := s.makeWorktree(project, person)
		if treeErr = s.store.PreflightWorktree(ctx, tree); treeErr != nil {
			return treeErr
		}
		cleanup, createErr := s.host.CreateWorktree(ctx, project, person, tree, baseRef)
		if createErr != nil {
			return createErr
		}
		if createErr = s.store.CreateWorktree(ctx, tree); createErr != nil {
			return s.compensate(ctx, createErr, cleanup, "personal workspace", tree.Path)
		}
	}
	if err = s.reconcileProjectAccess(ctx, projectID); err != nil {
		return err
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
		trees, treeErr := s.store.WorktreesForPerson(ctx, projectID, person.ID)
		if treeErr != nil {
			return treeErr
		}
		if len(trees) == 0 {
			continue
		}
		if len(trees) != 1 {
			return fmt.Errorf("person %s has multiple personal workspaces", person.Username)
		}
		keys, keyErr := s.store.SSHDeviceKeys(ctx, person.ID)
		if keyErr != nil {
			return keyErr
		}
		access = append(access, domain.ProjectAccess{Person: person, Worktree: trees[0], Keys: keys})
	}
	return s.host.ReconcileAuthorizedKeys(ctx, project, access)
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

func validateUsername(value string) error {
	if value == "" || len(value) > 24 || value[0] < 'a' || value[0] > 'z' {
		return invalid("username must start with a lowercase letter and contain at most 24 lowercase letters, digits, or hyphens")
	}
	for _, char := range []byte(value) {
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '-' {
			return invalid("username must start with a lowercase letter and contain at most 24 lowercase letters, digits, or hyphens")
		}
	}
	return nil
}
func validateSlug(value string) error { return validateUsername(value) }
func validatePerson(username, displayName, email string) error {
	if err := validateUsername(username); err != nil {
		return err
	}
	if strings.TrimSpace(displayName) == "" {
		return invalid("display name is required")
	}
	if !strings.Contains(email, "@") {
		return invalid("email address is invalid")
	}
	return nil
}
func parseID(value, kind string) (string, error) {
	if _, err := uuid.Parse(value); err != nil {
		return "", invalid("invalid %s ID", kind)
	}
	return value, nil
}
func parsePair(project, person string) (string, string, error) {
	projectID, err := parseID(project, "project")
	if err != nil {
		return "", "", err
	}
	personID, err := parseID(person, "person")
	return projectID, personID, err
}
