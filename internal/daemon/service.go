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

	"github.com/LevitateOS/soda-os/internal/domain"
	sodav2 "github.com/LevitateOS/soda-os/internal/gen/soda/v2"
	"github.com/LevitateOS/soda-os/internal/host"
	"github.com/LevitateOS/soda-os/internal/store"
	"github.com/LevitateOS/soda-os/internal/toolchain"
	"github.com/LevitateOS/soda-os/internal/version"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type EventPublisher interface {
	Publish(domain.EventKind, *string)
}
type EventMessage struct {
	Event   *domain.Event
	Refresh bool
}
type EventSubscription interface {
	Messages() <-chan EventMessage
	Close()
}
type EventSource interface {
	Subscribe(context.Context, *string) (EventSubscription, error)
}
type Telemetry interface {
	HostStatus(context.Context) (*sodav2.HostStatus, error)
	WorktreeStatuses(context.Context, string) ([]*sodav2.WorktreeStatus, error)
	ActiveSSHConnections(context.Context) ([]*sodav2.ActiveSshConnection, error)
}

type noopEvents struct{}

func (noopEvents) Publish(domain.EventKind, *string) {}

type Service struct {
	sodav2.UnimplementedSodaServiceServer
	store               *store.Store
	host                host.Operations
	toolchains          toolchain.Installer
	telemetry           Telemetry
	events              EventPublisher
	eventSource         EventSource
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
	Events              EventPublisher
	EventSource         EventSource
	ProjectsRoot        string
	Logger              *slog.Logger
	ProvisioningTimeout time.Duration
}

const defaultProvisioningTimeout = 30 * time.Minute

func New(options Options) *Service {
	background, cancel := context.WithCancel(context.Background())
	events := options.Events
	if events == nil {
		events = noopEvents{}
	}
	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}
	provisioningTimeout := options.ProvisioningTimeout
	if provisioningTimeout <= 0 {
		provisioningTimeout = defaultProvisioningTimeout
	}
	return &Service{store: options.Store, host: options.Host, toolchains: options.Toolchains, telemetry: options.Telemetry, events: events, eventSource: options.EventSource, projectsRoot: options.ProjectsRoot, logger: logger, background: background, cancel: cancel, provisioningTimeout: provisioningTimeout}
}
func (s *Service) Close() { s.cancel(); s.wg.Wait() }

func (s *Service) BootstrapInstallerAdministrator(ctx context.Context) error {
	people, err := s.store.People(ctx)
	if err != nil {
		return err
	}
	if len(people) != 0 {
		return nil
	}
	candidate, err := s.host.InstallerAdministrator(ctx)
	if err != nil {
		return err
	}
	if candidate == nil {
		return nil
	}
	candidate.ID = uuid.NewString()
	cleanup, err := s.host.ImportPerson(ctx, *candidate)
	if err != nil {
		return err
	}
	if err = s.store.CreatePerson(ctx, *candidate); err != nil {
		return s.compensate(ctx, err, cleanup, "installer administrator", candidate.Username)
	}
	s.events.Publish(domain.EventPeopleChanged, nil)
	return nil
}

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
	if strings.ContainsAny(request.GetPassword(), "\r\n\x00") {
		return nil, rpcError(invalid("password contains a line or NUL delimiter"))
	}
	if err = host.ValidatePublicKey(request.GetSshPublicKey(), false); err != nil {
		return nil, rpcError(invalid("%v", err))
	}
	if err = s.store.PreflightPerson(ctx, request.GetUsername(), request.GetSshPublicKey()); err != nil {
		return nil, rpcError(err)
	}
	person := domain.Person{ID: uuid.NewString(), Username: request.GetUsername(), DisplayName: request.GetDisplayName(), Email: request.GetEmail(), Role: role, SSHPublicKey: request.GetSshPublicKey()}
	cleanup, err := s.host.CreatePerson(ctx, person, request.GetPassword())
	if err != nil {
		return nil, rpcError(err)
	}
	if err = s.store.CreatePerson(ctx, person); err != nil {
		return nil, rpcError(s.compensate(ctx, err, cleanup, "person", person.Username))
	}
	s.events.Publish(domain.EventPeopleChanged, nil)
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
	if err = host.ValidatePublicKey(request.GetSshPublicKey(), true); err != nil {
		return nil, rpcError(invalid("%v", err))
	}
	if err = s.store.PreflightPerson(ctx, request.GetUsername(), request.GetSshPublicKey()); err != nil {
		return nil, rpcError(err)
	}
	person := domain.Person{ID: uuid.NewString(), Username: request.GetUsername(), DisplayName: request.GetDisplayName(), Email: request.GetEmail(), Role: role, SSHPublicKey: request.GetSshPublicKey()}
	cleanup, err := s.host.ImportPerson(ctx, person)
	if err != nil {
		return nil, rpcError(err)
	}
	if err = s.store.CreatePerson(ctx, person); err != nil {
		return nil, rpcError(s.compensate(ctx, err, cleanup, "imported person", person.Username))
	}
	s.events.Publish(domain.EventPeopleChanged, nil)
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
	project := domain.Project{ID: uuid.NewString(), Slug: request.GetSlug(), Name: request.GetName(), UnixUser: "soda-p-" + request.GetSlug(), Profile: profile, Source: source}
	if err = s.store.PreflightProject(ctx, project.Slug, project.UnixUser); err != nil {
		return nil, rpcError(err)
	}
	cleanup, err := s.host.CreateProject(ctx, project)
	if err != nil {
		return nil, rpcError(err)
	}
	if err = s.store.CreateProject(ctx, project); err != nil {
		return nil, rpcError(s.compensate(ctx, err, cleanup, "project", project.Slug))
	}
	if _, err = s.startProvisioning(project.ID); err != nil {
		cleanupErr := s.store.DeleteFreshProject(context.WithoutCancel(ctx), project.ID)
		if hostCleanupErr := s.runCleanup(ctx, cleanup); hostCleanupErr != nil {
			cleanupErr = errors.Join(cleanupErr, hostCleanupErr)
		}
		if cleanupErr != nil {
			s.logger.Error("clean up failed project creation", slog.String("project", project.Slug), slog.Any("error", cleanupErr))
			err = errors.Join(err, fmt.Errorf("cleanup failed: %w", cleanupErr))
		}
		return nil, rpcError(err)
	}
	s.events.Publish(domain.EventProjectsChanged, nil)
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
	tree := s.makeWorktree(project, person, "default", "people", "")
	if err = s.store.PreflightWorktree(ctx, tree); err != nil {
		return nil, rpcError(err)
	}
	cleanup, err := s.host.CreateWorktree(ctx, project, person, tree, "main")
	if err != nil {
		return nil, rpcError(err)
	}
	membership := domain.Membership{ProjectID: projectID, PersonID: personID}
	if err = s.store.AddMembershipAndWorktree(ctx, membership, tree); err != nil {
		return nil, rpcError(s.compensate(ctx, err, cleanup, "collaborator worktree", tree.Path))
	}
	s.events.Publish(domain.EventWorktreesChanged, &projectID)
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
func (s *Service) CreateWorktree(ctx context.Context, request *sodav2.CreateWorktreeRequest) (*sodav2.CreateWorktreeResponse, error) {
	projectID, personID, err := parsePair(request.GetProjectId(), request.GetPersonId())
	if err != nil {
		return nil, rpcError(err)
	}
	if err = validateSlug(request.GetName()); err != nil {
		return nil, rpcError(err)
	}
	if strings.TrimSpace(request.GetBaseRef()) == "" {
		return nil, rpcError(invalid("base ref is required"))
	}
	project, err := s.store.Project(ctx, projectID)
	if err != nil {
		return nil, rpcError(err)
	}
	person, err := s.store.Person(ctx, personID)
	if err != nil {
		return nil, rpcError(err)
	}
	if _, err = s.store.Membership(ctx, projectID, personID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, rpcError(precondition("person is not a project collaborator"))
		}
		return nil, rpcError(err)
	}
	tree := s.makeWorktree(project, person, request.GetName(), "work", request.GetName())
	if err = s.store.PreflightWorktree(ctx, tree); err != nil {
		return nil, rpcError(err)
	}
	cleanup, err := s.host.CreateWorktree(ctx, project, person, tree, request.GetBaseRef())
	if err != nil {
		return nil, rpcError(err)
	}
	if err = s.store.CreateWorktree(ctx, tree); err != nil {
		return nil, rpcError(s.compensate(ctx, err, cleanup, "worktree", tree.Path))
	}
	s.events.Publish(domain.EventWorktreesChanged, &projectID)
	return &sodav2.CreateWorktreeResponse{Worktree: worktreeProto(tree)}, nil
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
func (s *Service) ListWorktreeStatuses(ctx context.Context, request *sodav2.ListWorktreeStatusesRequest) (*sodav2.ListWorktreeStatusesResponse, error) {
	id, err := parseID(request.GetProjectId(), "project")
	if err != nil {
		return nil, rpcError(err)
	}
	if _, err = s.store.Project(ctx, id); err != nil {
		return nil, rpcError(err)
	}
	if s.telemetry == nil {
		return nil, status.Error(codes.Unavailable, "Git telemetry is unavailable")
	}
	values, err := s.telemetry.WorktreeStatuses(ctx, id)
	if err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}
	return &sodav2.ListWorktreeStatusesResponse{Worktrees: values}, nil
}
func (s *Service) ListActiveSshConnections(ctx context.Context, _ *sodav2.ListActiveSshConnectionsRequest) (*sodav2.ListActiveSshConnectionsResponse, error) {
	if s.telemetry == nil {
		return nil, status.Error(codes.Unavailable, "SSH telemetry is unavailable")
	}
	values, err := s.telemetry.ActiveSSHConnections(ctx)
	if err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}
	return &sodav2.ListActiveSshConnectionsResponse{Connections: values}, nil
}
func (s *Service) SubscribeEvents(request *sodav2.SubscribeEventsRequest, stream grpc.ServerStreamingServer[sodav2.SubscribeEventsResponse]) error {
	if s.eventSource == nil {
		return status.Error(codes.Unavailable, "event stream is unavailable")
	}
	var projectID *string
	if request.ProjectId != nil {
		id, err := parseID(request.GetProjectId(), "project")
		if err != nil {
			return rpcError(err)
		}
		projectID = &id
	}
	subscription, err := s.eventSource.Subscribe(stream.Context(), projectID)
	if err != nil {
		return status.Error(codes.Unavailable, err.Error())
	}
	defer subscription.Close()
	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case message, ok := <-subscription.Messages():
			if !ok {
				return status.Error(codes.Unavailable, "event stream closed")
			}
			if message.Refresh {
				if err = stream.Send(&sodav2.SubscribeEventsResponse{Payload: &sodav2.SubscribeEventsResponse_Control{Control: sodav2.StreamControl_STREAM_CONTROL_REFRESH}}); err != nil {
					return err
				}
				continue
			}
			if message.Event == nil {
				continue
			}
			event := *message.Event
			protoEvent := &sodav2.SodaEvent{Kind: eventKindProto(event.Kind), Sequence: event.Sequence}
			protoEvent.ProjectId = event.ProjectID
			if err = stream.Send(&sodav2.SubscribeEventsResponse{Payload: &sodav2.SubscribeEventsResponse_Event{Event: protoEvent}}); err != nil {
				return err
			}
		}
	}
}

func (s *Service) startProvisioning(projectID string) (domain.ProvisioningJob, error) {
	job := domain.ProvisioningJob{ID: uuid.NewString(), ProjectID: projectID, State: domain.JobInstalling}
	if err := s.store.BeginProvisioning(s.background, job); err != nil {
		return domain.ProvisioningJob{}, err
	}
	s.events.Publish(domain.EventProvisioningChanged, &projectID)
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
	s.events.Publish(domain.EventProvisioningChanged, &projectID)
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
	installation, _, err := s.store.ProjectInstallation(ctx, projectID)
	if err == nil {
		return s.host.WriteProjectEnvironment(ctx, project, "source "+filepath.Join(installation.Path, "env")+"\n")
	}
	if !errors.Is(err, store.ErrNotFound) {
		return err
	}
	installation, err = s.toolchains.Install(ctx, project.Profile)
	if err != nil {
		return err
	}
	installation.ID = uuid.NewString()
	_, err = s.store.SaveInstallation(ctx, projectID, installation)
	if err != nil {
		return err
	}
	return s.host.WriteProjectEnvironment(ctx, project, "source "+filepath.Join(installation.Path, "env")+"\n")
}
func (s *Service) makeWorktree(project domain.Project, person domain.Person, name, prefix, suffix string) domain.Worktree {
	branch := prefix + "/" + person.Username
	if suffix != "" {
		branch += "/" + suffix
	}
	path := filepath.Join(s.projectsRoot, project.Slug, "worktrees", person.Username)
	if name != "default" {
		path = filepath.Join(path, name)
	}
	return domain.Worktree{ID: uuid.NewString(), ProjectID: project.ID, PersonID: person.ID, Name: name, Branch: branch, Path: path}
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
func eventKindProto(kind domain.EventKind) sodav2.EventKind {
	switch kind {
	case domain.EventHostChanged:
		return sodav2.EventKind_EVENT_KIND_HOST_CHANGED
	case domain.EventPeopleChanged:
		return sodav2.EventKind_EVENT_KIND_PEOPLE_CHANGED
	case domain.EventProjectsChanged:
		return sodav2.EventKind_EVENT_KIND_PROJECTS_CHANGED
	case domain.EventWorktreesChanged:
		return sodav2.EventKind_EVENT_KIND_WORKTREES_CHANGED
	case domain.EventProvisioningChanged:
		return sodav2.EventKind_EVENT_KIND_PROVISIONING_CHANGED
	case domain.EventGitChanged:
		return sodav2.EventKind_EVENT_KIND_GIT_CHANGED
	case domain.EventSessionsChanged:
		return sodav2.EventKind_EVENT_KIND_SESSIONS_CHANGED
	default:
		return sodav2.EventKind_EVENT_KIND_UNSPECIFIED
	}
}
