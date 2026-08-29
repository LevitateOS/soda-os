package daemon

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/domain"
	sodav2 "github.com/LevitateOS/soda-os/internal/gen/soda/v2"
	"github.com/LevitateOS/soda-os/internal/grpcclient"
	"github.com/LevitateOS/soda-os/internal/osupdate"
	"github.com/LevitateOS/soda-os/internal/store"
	"github.com/LevitateOS/soda-os/internal/telemetry"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type fakeOSUpdater struct {
	status          osupdate.Status
	candidate       osupdate.Candidate
	stagedReference string
	activated       bool
	err             error
}

func (u *fakeOSUpdater) Status(context.Context) (osupdate.Status, error)   { return u.status, u.err }
func (u *fakeOSUpdater) Check(context.Context) (osupdate.Candidate, error) { return u.candidate, u.err }
func (u *fakeOSUpdater) Stage(_ context.Context, reference string) (osupdate.Status, error) {
	u.stagedReference = reference
	return u.status, u.err
}
func (u *fakeOSUpdater) Activate(context.Context) error {
	u.activated = true
	return u.err
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
	person, err := client.CreatePerson(ctx, &sodav2.CreatePersonRequest{Username: "alice", DisplayName: "Alice", Email: "alice@example.test", Role: sodav2.Role_ROLE_DEVELOPER, Password: "simple"})
	if err != nil {
		t.Fatal(err)
	}
	project, err := client.CreateProject(ctx, &sodav2.CreateProjectRequest{Slug: "demo", Name: "Demo", Profile: sodav2.ToolchainProfile_TOOLCHAIN_PROFILE_GO, Source: &sodav2.ProjectSource{Source: &sodav2.ProjectSource_Empty{Empty: &sodav2.EmptyProjectSource{}}}, InitialPersonIds: []string{person.Person.Id}})
	if err != nil {
		t.Fatal(err)
	}
	projectID := project.Project.Id
	waitForJobState(t, service.store, projectID, domain.JobReady)
	toolchainResponse, err := client.GetProjectToolchain(ctx, &sodav2.GetProjectToolchainRequest{ProjectId: projectID})
	if err != nil {
		t.Fatal(err)
	}
	if toolchainResponse.Installation.Profile != sodav2.ToolchainProfile_TOOLCHAIN_PROFILE_GO {
		t.Fatalf("toolchain = %#v", toolchainResponse.Installation)
	}
	collaborators, err := client.ListCollaborators(ctx, &sodav2.ListCollaboratorsRequest{ProjectId: projectID})
	if err != nil || len(collaborators.Collaborators) != 1 || len(collaborators.Collaborators[0].Worktrees) != 1 || collaborators.Collaborators[0].Worktrees[0].Branch != "people/alice" {
		t.Fatalf("collaborators = %#v, %v", collaborators, err)
	}
}

func TestProjectRequiresAtLeastOneInitialMember(t *testing.T) {
	repository, err := store.Open(filepath.Join(t.TempDir(), "soda.db"))
	if err != nil {
		t.Fatal(err)
	}
	hostSystem := &fakeHost{}
	service := New(Options{Store: repository, Host: hostSystem, Toolchains: fakeInstaller{}, ProjectsRoot: t.TempDir()})
	defer service.Close()
	request := &sodav2.CreateProjectRequest{
		Slug: "demo", Name: "Demo", Profile: sodav2.ToolchainProfile_TOOLCHAIN_PROFILE_GO,
		Source: &sodav2.ProjectSource{Source: &sodav2.ProjectSource_Empty{Empty: &sodav2.EmptyProjectSource{}}},
	}
	if _, err = service.CreateProject(context.Background(), request); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("project without an initial member status = %s: %v", status.Code(err), err)
	}
	if len(hostSystem.workspaces.projects) != 0 {
		t.Fatalf("invalid project changed host state: %#v", hostSystem.workspaces.projects)
	}
}

func TestRealUnixSocketPermissionsAndHealth(t *testing.T) {
	if _, err := user.LookupGroup(apiGroup); err != nil {
		t.Skipf("%s group is unavailable: %v", apiGroup, err)
	}
	service := newTestService(t)
	socketDir, err := os.MkdirTemp("/tmp", "soda-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(socketDir) })
	socket := filepath.Join(socketDir, "s.sock")
	server, err := ListenUnix(socket, service, slog.Default())
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

func TestCommittedHostObservabilityBacksTelemetry(t *testing.T) {
	repository, err := store.Open(filepath.Join(t.TempDir(), "soda.db"))
	if err != nil {
		t.Fatal(err)
	}
	manager, err := telemetry.NewManager(observeHost{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	manager.Run(ctx)
	adapter := NewTelemetryAdapter(manager)
	service := New(Options{Store: repository, Host: &fakeHost{}, Toolchains: fakeInstaller{}, Telemetry: adapter, ProjectsRoot: t.TempDir()})
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
	if hostResponse.Host.GetOverall() != sodav2.RuntimeState_RUNTIME_STATE_READY {
		t.Fatalf("host status = %#v", hostResponse.Host)
	}
}

func TestOSUpdateRPCsPreserveExactIdentityAndRebootConfirmation(t *testing.T) {
	digest := "sha256:" + strings.Repeat("b", 64)
	exact := osupdate.Repository + "@" + digest
	updates := &fakeOSUpdater{
		status:    osupdate.Status{Booted: &osupdate.Deployment{ImageReference: osupdate.Repository + "@sha256:" + strings.Repeat("a", 64), Architecture: "arm64", Signature: "containerPolicy"}},
		candidate: osupdate.Candidate{ImageReference: exact, Digest: digest, Version: "0.3.0", StateSchema: 3, Available: true},
	}
	service := New(Options{OSUpdates: updates})
	defer service.Close()
	ctx := context.Background()

	checked, err := service.CheckOSUpdate(ctx, &sodav2.CheckOSUpdateRequest{})
	if err != nil || checked.GetRelease().GetImageReference() != exact || checked.GetRelease().GetStateSchema() != 3 {
		t.Fatalf("checked release = %#v, %v", checked, err)
	}
	if _, err = service.StageOSUpdate(ctx, &sodav2.StageOSUpdateRequest{ImageReference: exact}); err != nil || updates.stagedReference != exact {
		t.Fatalf("staged reference = %q, %v", updates.stagedReference, err)
	}
	if _, err = service.ActivateOSUpdate(ctx, &sodav2.ActivateOSUpdateRequest{ConfirmReboot: true}); err != nil || !updates.activated {
		t.Fatalf("activation confirmation = %v, %v", updates.activated, err)
	}

	updates.err = osupdate.ErrRejected
	_, err = service.CheckOSUpdate(ctx, &sodav2.CheckOSUpdateRequest{})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("rejected release status = %v", status.Code(err))
	}
}
