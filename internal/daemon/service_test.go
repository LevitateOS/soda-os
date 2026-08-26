package daemon

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/LevitateOS/soda-os/internal/domain"
	sodav2 "github.com/LevitateOS/soda-os/internal/gen/soda/v2"
	"github.com/LevitateOS/soda-os/internal/grpcclient"
	"github.com/LevitateOS/soda-os/internal/observe"
	"github.com/LevitateOS/soda-os/internal/store"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type fakeHost struct {
	mu           sync.Mutex
	projects     []domain.Project
	worktrees    []domain.Worktree
	environments int
}

func (*fakeHost) InstallerAdministrator(context.Context) (*domain.Person, error) { return nil, nil }
func (*fakeHost) CreatePerson(context.Context, domain.Person, string) error      { return nil }
func (*fakeHost) ImportPerson(context.Context, domain.Person) error              { return nil }
func (h *fakeHost) CreateProject(_ context.Context, value domain.Project) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.projects = append(h.projects, value)
	return nil
}
func (*fakeHost) EnsureRepository(context.Context, domain.Project) error { return nil }
func (h *fakeHost) CreateWorktree(_ context.Context, _ domain.Project, _ domain.Person, value domain.Worktree, _ string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.worktrees = append(h.worktrees, value)
	return nil
}
func (h *fakeHost) WriteProjectEnvironment(context.Context, domain.Project, string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.environments++
	return nil
}
func (*fakeHost) DeployPublicKey(context.Context, domain.Project) (string, error) {
	return "ssh-ed25519 AAAA project", nil
}

type fakeInstaller struct{}

func (fakeInstaller) Install(_ context.Context, profile domain.ToolchainProfile) (domain.ToolchainInstallation, error) {
	return domain.ToolchainInstallation{Profile: profile, Version: string(profile) + "=test", Path: "/toolchains/" + string(profile), Checksum: "verified", State: domain.JobReady}, nil
}

type observeHost struct{}

func (observeHost) SampleHost(context.Context) (domain.HostStatus, error) {
	return domain.HostStatus{SampledAt: time.Now(), Overall: domain.RuntimeReady}, nil
}

type observeGit struct{}

func (observeGit) Inspect(_ context.Context, _ domain.Project, tree domain.Worktree) domain.WorktreeStatus {
	return domain.WorktreeStatus{WorktreeID: tree.ID, State: domain.WorktreeClean}
}

type observeSessions struct{}

func (observeSessions) Inspect(context.Context, []domain.Project, []domain.Person, []domain.Worktree) (observe.SessionObservation, error) {
	return observe.SessionObservation{Connections: []domain.ActiveSSHConnection{{ID: "connection", ConnectedAt: time.Now()}}}, nil
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	repository, err := store.Open(filepath.Join(t.TempDir(), "soda.db"))
	if err != nil {
		t.Fatal(err)
	}
	service := New(Options{Store: repository, Host: &fakeHost{}, Toolchains: fakeInstaller{}, ProjectsRoot: filepath.Join(t.TempDir(), "projects")})
	t.Cleanup(service.Close)
	return service
}

func TestUnaryWorkflowOverBufconn(t *testing.T) {
	service := newTestService(t)
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer()
	sodav2.RegisterSodaServiceServer(server, service)
	go server.Serve(listener)
	t.Cleanup(server.Stop)
	connection, err := grpc.NewClient("passthrough:///bufnet", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }))
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	client := sodav2.NewSodaServiceClient(connection)
	ctx := context.Background()
	personResponse, err := client.CreatePerson(ctx, &sodav2.CreatePersonRequest{Username: "alice", DisplayName: "Alice", Email: "alice@example.test", Role: sodav2.Role_ROLE_DEVELOPER, SshPublicKey: "ssh-ed25519 AAAA alice", Password: "local"})
	if err != nil {
		t.Fatal(err)
	}
	projectResponse, err := client.CreateProject(ctx, &sodav2.CreateProjectRequest{Slug: "demo", Name: "Demo", Profile: sodav2.ToolchainProfile_TOOLCHAIN_PROFILE_GO, Source: &sodav2.ProjectSource{Source: &sodav2.ProjectSource_Empty{Empty: &sodav2.EmptyProjectSource{}}}})
	if err != nil {
		t.Fatal(err)
	}
	projectID := projectResponse.Project.Id
	personID := personResponse.Person.Id
	collaborator, err := client.AddCollaborator(ctx, &sodav2.AddCollaboratorRequest{ProjectId: projectID, PersonId: personID})
	if err != nil {
		t.Fatal(err)
	}
	if collaborator.Worktree.Branch != "people/alice" {
		t.Fatalf("branch = %q", collaborator.Worktree.Branch)
	}
	created, err := client.CreateWorktree(ctx, &sodav2.CreateWorktreeRequest{ProjectId: projectID, PersonId: personID, Name: "feature", BaseRef: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if created.Worktree.Branch != "work/alice/feature" {
		t.Fatalf("branch = %q", created.Worktree.Branch)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		jobs, jobErr := client.ListProvisioningJobs(ctx, &sodav2.ListProvisioningJobsRequest{ProjectId: projectID})
		if jobErr != nil {
			t.Fatal(jobErr)
		}
		if len(jobs.Jobs) > 0 && jobs.Jobs[0].State == sodav2.JobState_JOB_STATE_READY {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("jobs did not become ready: %v", jobs.Jobs)
		}
		time.Sleep(10 * time.Millisecond)
	}
	toolchainResponse, err := client.GetProjectToolchain(ctx, &sodav2.GetProjectToolchainRequest{ProjectId: projectID})
	if err != nil {
		t.Fatal(err)
	}
	if toolchainResponse.Installation.Profile != sodav2.ToolchainProfile_TOOLCHAIN_PROFILE_GO {
		t.Fatalf("toolchain = %#v", toolchainResponse.Installation)
	}
	collaborators, err := client.ListCollaborators(ctx, &sodav2.ListCollaboratorsRequest{ProjectId: projectID})
	if err != nil || len(collaborators.Collaborators) != 1 || len(collaborators.Collaborators[0].Worktrees) != 2 {
		t.Fatalf("collaborators = %#v, %v", collaborators, err)
	}
}

func TestRealUnixSocketPermissionsAndHealth(t *testing.T) {
	service := newTestService(t)
	socketDir, err := os.MkdirTemp("/tmp", "soda-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(socketDir) })
	socket := filepath.Join(socketDir, "s.sock")
	server, err := ListenUnix(socket, "", service, nil)
	if err != nil {
		t.Fatal(err)
	}
	go server.Serve()
	t.Cleanup(server.Stop)
	connection, err := grpcclient.Dial(context.Background(), socket)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	health, err := sodav2.NewSodaServiceClient(connection).Health(context.Background(), &sodav2.HealthRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if health.Service != "sodad" {
		t.Fatalf("health = %#v", health)
	}
	info, err := os.Stat(socket)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o660 {
		t.Fatalf("socket mode = %o", info.Mode().Perm())
	}
}

func TestValidationUsesCanonicalStatus(t *testing.T) {
	service := newTestService(t)
	_, err := service.CreatePerson(context.Background(), &sodav2.CreatePersonRequest{Username: "Invalid", DisplayName: "Invalid", Email: "invalid", Role: sodav2.Role_ROLE_DEVELOPER})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("status = %s: %v", status.Code(err), err)
	}
	_, err = service.ListProjectsForPerson(context.Background(), &sodav2.ListProjectsForPersonRequest{PersonId: uuid.NewString()})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("status = %s: %v", status.Code(err), err)
	}
}

func TestCommittedObservabilityBacksTelemetryAndEventStream(t *testing.T) {
	repository, err := store.Open(filepath.Join(t.TempDir(), "soda.db"))
	if err != nil {
		t.Fatal(err)
	}
	manager, err := observe.NewManager(observe.Dependencies{Store: repository, Host: observeHost{}, Git: observeGit{}, Sessions: observeSessions{}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer manager.Broker().Close()
	manager.Run(ctx)
	adapter := NewObservability(manager)
	service := New(Options{Store: repository, Host: &fakeHost{}, Toolchains: fakeInstaller{}, Telemetry: adapter, Events: adapter, EventSource: adapter, ProjectsRoot: t.TempDir()})
	defer service.Close()
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer()
	sodav2.RegisterSodaServiceServer(server, service)
	go server.Serve(listener)
	defer server.Stop()
	connection, err := grpc.NewClient("passthrough:///bufnet", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }))
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	client := sodav2.NewSodaServiceClient(connection)
	hostResponse, err := client.GetHostStatus(ctx, &sodav2.GetHostStatusRequest{})
	if err != nil || hostResponse.Host == nil {
		t.Fatalf("host = %#v, %v", hostResponse, err)
	}
	sessions, err := client.ListActiveSshConnections(ctx, &sodav2.ListActiveSshConnectionsRequest{})
	if err != nil || len(sessions.Connections) != 1 {
		t.Fatalf("sessions = %#v, %v", sessions, err)
	}
	streamCtx, streamCancel := context.WithTimeout(ctx, 2*time.Second)
	defer streamCancel()
	stream, err := client.SubscribeEvents(streamCtx, &sodav2.SubscribeEventsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	first, err := stream.Recv()
	if err != nil {
		t.Fatal(err)
	}
	if first.GetControl() != sodav2.StreamControl_STREAM_CONTROL_REFRESH {
		t.Fatalf("initial stream message = %#v", first)
	}
	manager.Broker().Publish(domain.EventPeopleChanged, "")
	for {
		message, receiveErr := stream.Recv()
		if receiveErr != nil {
			t.Fatal(receiveErr)
		}
		if message.GetEvent().GetKind() == sodav2.EventKind_EVENT_KIND_PEOPLE_CHANGED {
			break
		}
	}
}
