package daemon

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LevitateOS/soda-os/internal/domain"
	sodav2 "github.com/LevitateOS/soda-os/internal/gen/soda/v2"
	"github.com/LevitateOS/soda-os/internal/grpcclient"
	"github.com/LevitateOS/soda-os/internal/host"
	"github.com/LevitateOS/soda-os/internal/observe"
	"github.com/LevitateOS/soda-os/internal/osupdate"
	"github.com/LevitateOS/soda-os/internal/store"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"gorm.io/gorm"
)

type fakeOSUpdater struct {
	status          osupdate.Status
	candidate       osupdate.Candidate
	stagedReference string
	confirmed       bool
	err             error
}

func (u *fakeOSUpdater) Status(context.Context) (osupdate.Status, error)   { return u.status, u.err }
func (u *fakeOSUpdater) Check(context.Context) (osupdate.Candidate, error) { return u.candidate, u.err }
func (u *fakeOSUpdater) Stage(_ context.Context, reference string) (osupdate.Status, error) {
	u.stagedReference = reference
	return u.status, u.err
}
func (u *fakeOSUpdater) Activate(_ context.Context, confirmed bool) error {
	u.confirmed = confirmed
	return u.err
}

type fakeHost struct {
	mu               sync.Mutex
	people           int
	imports          int
	projects         []domain.Project
	worktrees        []domain.Worktree
	environments     int
	personCleanups   int
	projectCleanups  int
	worktreeCleanups int
	worktreeAttempts int
	failWorktreeAt   int
	baseRefs         []string
	reconciliations  [][]domain.ProjectAccess
	reconcileErr     error
}

func (h *fakeHost) CreatePerson(context.Context, domain.Person, string) (host.Cleanup, error) {
	h.mu.Lock()
	h.people++
	h.mu.Unlock()
	return func(context.Context) error {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.personCleanups++
		return nil
	}, nil
}
func (h *fakeHost) ImportPerson(context.Context, domain.Person) (host.Cleanup, error) {
	h.mu.Lock()
	h.imports++
	h.mu.Unlock()
	return func(context.Context) error {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.personCleanups++
		return nil
	}, nil
}
func (h *fakeHost) CreateProject(_ context.Context, value domain.Project) (host.Cleanup, error) {
	h.mu.Lock()
	h.projects = append(h.projects, value)
	h.mu.Unlock()
	return func(context.Context) error {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.projectCleanups++
		return nil
	}, nil
}
func (*fakeHost) EnsureRepository(context.Context, domain.Project) error        { return nil }
func (*fakeHost) DefaultBranch(context.Context, domain.Project) (string, error) { return "trunk", nil }
func (h *fakeHost) CreateWorktree(_ context.Context, _ domain.Project, _ domain.Person, value domain.Worktree, baseRef string) (host.Cleanup, error) {
	h.mu.Lock()
	h.worktreeAttempts++
	if h.failWorktreeAt == h.worktreeAttempts {
		h.mu.Unlock()
		return nil, errors.New("injected personal workspace failure")
	}
	h.worktrees = append(h.worktrees, value)
	h.baseRefs = append(h.baseRefs, baseRef)
	h.mu.Unlock()
	return func(context.Context) error {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.worktreeCleanups++
		return nil
	}, nil
}
func (h *fakeHost) ReconcileAuthorizedKeys(_ context.Context, _ domain.Project, access []domain.ProjectAccess) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	copyOfAccess := append([]domain.ProjectAccess(nil), access...)
	h.reconciliations = append(h.reconciliations, copyOfAccess)
	return h.reconcileErr
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

type blockingInstaller struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

type timeoutInstaller struct{}

func (timeoutInstaller) Install(ctx context.Context, _ domain.ToolchainProfile) (domain.ToolchainInstallation, error) {
	<-ctx.Done()
	return domain.ToolchainInstallation{}, ctx.Err()
}

func (b *blockingInstaller) Install(_ context.Context, profile domain.ToolchainProfile) (domain.ToolchainInstallation, error) {
	b.once.Do(func() { close(b.started) })
	<-b.release
	return domain.ToolchainInstallation{Profile: profile, Version: string(profile) + "=blocked", Path: "/toolchains/" + string(profile), Checksum: "verified", State: domain.JobReady}, nil
}

type observeHost struct{}

func (observeHost) SampleHost(context.Context) (domain.HostStatus, error) {
	return domain.HostStatus{SampledAt: time.Now(), Overall: domain.RuntimeReady}, nil
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

func testStore(t *testing.T) *store.Store {
	t.Helper()
	repository, err := store.Open(filepath.Join(t.TempDir(), "soda.db"))
	if err != nil {
		t.Fatal(err)
	}
	return repository
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
	device, err := client.CreateSshDeviceKey(ctx, &sodav2.CreateSshDeviceKeyRequest{PersonId: person.Person.Id, Label: "Laptop", PublicKey: "ssh-ed25519 AAAA alice", IdentityFileHint: "~/.ssh/id_ed25519"})
	if err != nil || device.Key.GetFingerprint() == "" {
		t.Fatalf("device key = %#v, %v", device, err)
	}
	project, err := client.CreateProject(ctx, &sodav2.CreateProjectRequest{Slug: "demo", Name: "Demo", Profile: sodav2.ToolchainProfile_TOOLCHAIN_PROFILE_GO, Source: &sodav2.ProjectSource{Source: &sodav2.ProjectSource_Empty{Empty: &sodav2.EmptyProjectSource{}}}, InitialPersonIds: []string{person.Person.Id}})
	if err != nil {
		t.Fatal(err)
	}
	projectID := project.Project.Id
	waitForJobState(t, service.store, projectID, domain.JobReady)
	assertUnaryProjectViews(t, ctx, client, projectID)
}

func assertUnaryProjectViews(t *testing.T, ctx context.Context, client sodav2.SodaServiceClient, projectID string) {
	t.Helper()
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
	if len(hostSystem.projects) != 0 {
		t.Fatalf("invalid project changed host state: %#v", hostSystem.projects)
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
	manager, err := observe.NewManager(observeHost{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	manager.Run(ctx)
	adapter := NewObservability(manager)
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
		candidate: osupdate.Candidate{ImageReference: exact, Digest: digest, Version: "0.3.0", StateSchema: 2, Available: true},
	}
	service := New(Options{OSUpdates: updates})
	defer service.Close()
	ctx := context.Background()

	checked, err := service.CheckOSUpdate(ctx, &sodav2.CheckOSUpdateRequest{})
	if err != nil || checked.GetRelease().GetImageReference() != exact || checked.GetRelease().GetStateSchema() != 2 {
		t.Fatalf("checked release = %#v, %v", checked, err)
	}
	if _, err = service.StageOSUpdate(ctx, &sodav2.StageOSUpdateRequest{ImageReference: exact}); err != nil || updates.stagedReference != exact {
		t.Fatalf("staged reference = %q, %v", updates.stagedReference, err)
	}
	if _, err = service.ActivateOSUpdate(ctx, &sodav2.ActivateOSUpdateRequest{ConfirmReboot: true}); err != nil || !updates.confirmed {
		t.Fatalf("activation confirmation = %v, %v", updates.confirmed, err)
	}

	updates.err = osupdate.ErrRejected
	_, err = service.CheckOSUpdate(ctx, &sodav2.CheckOSUpdateRequest{})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("rejected release status = %v", status.Code(err))
	}
}

func TestDuplicatePreflightsDoNotExecuteHostMutations(t *testing.T) {
	repository := testStore(t)
	hostSystem := &fakeHost{}
	service := New(Options{Store: repository, Host: hostSystem, Toolchains: fakeInstaller{}, ProjectsRoot: filepath.Join(t.TempDir(), "projects")})
	defer service.Close()
	ctx := context.Background()
	personRequest := &sodav2.CreatePersonRequest{Username: "alice", DisplayName: "Alice", Email: "alice@example.test", Role: sodav2.Role_ROLE_DEVELOPER, Password: "simple"}
	alice, err := service.CreatePerson(ctx, personRequest)
	if err != nil {
		t.Fatal(err)
	}
	bob, err := service.CreatePerson(ctx, &sodav2.CreatePersonRequest{Username: "bob", DisplayName: "Bob", Email: "bob@example.test", Role: sodav2.Role_ROLE_DEVELOPER, Password: "simple"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.CreatePerson(ctx, personRequest); status.Code(err) != codes.AlreadyExists {
		t.Fatalf("duplicate person status = %s: %v", status.Code(err), err)
	}
	if _, err = service.ImportPerson(ctx, &sodav2.ImportPersonRequest{Username: "alice", DisplayName: "Alice", Email: "alice@example.test", Role: sodav2.Role_ROLE_DEVELOPER}); status.Code(err) != codes.AlreadyExists {
		t.Fatalf("duplicate import status = %s: %v", status.Code(err), err)
	}
	projectRequest := &sodav2.CreateProjectRequest{Slug: "demo", Name: "Demo", Profile: sodav2.ToolchainProfile_TOOLCHAIN_PROFILE_GO, Source: &sodav2.ProjectSource{Source: &sodav2.ProjectSource_Empty{Empty: &sodav2.EmptyProjectSource{}}}, InitialPersonIds: []string{alice.Person.Id}}
	project, err := service.CreateProject(ctx, projectRequest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.CreateProject(ctx, projectRequest); status.Code(err) != codes.AlreadyExists {
		t.Fatalf("duplicate project status = %s: %v", status.Code(err), err)
	}
	waitForJobState(t, repository, project.Project.Id, domain.JobReady)
	if _, err := service.AddCollaborator(ctx, &sodav2.AddCollaboratorRequest{ProjectId: project.Project.Id, PersonId: alice.Person.Id}); status.Code(err) != codes.AlreadyExists {
		t.Fatalf("duplicate membership status = %s: %v", status.Code(err), err)
	}
	collaborator := &sodav2.AddCollaboratorRequest{ProjectId: project.Project.Id, PersonId: bob.Person.Id}
	if _, err := service.AddCollaborator(ctx, collaborator); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AddCollaborator(ctx, collaborator); status.Code(err) != codes.AlreadyExists {
		t.Fatalf("duplicate later membership status = %s: %v", status.Code(err), err)
	}
	assertDuplicatePreflightsHostState(t, hostSystem)
}

func assertDuplicatePreflightsHostState(t *testing.T, hostSystem *fakeHost) {
	t.Helper()
	hostSystem.mu.Lock()
	defer hostSystem.mu.Unlock()
	if hostSystem.people != 2 || hostSystem.imports != 0 || len(hostSystem.projects) != 1 || len(hostSystem.worktrees) != 2 {
		t.Fatalf("host mutations: people=%d imports=%d projects=%d worktrees=%d", hostSystem.people, hostSystem.imports, len(hostSystem.projects), len(hostSystem.worktrees))
	}
	for _, baseRef := range hostSystem.baseRefs {
		if baseRef != "trunk" {
			t.Fatalf("workspace base ref = %q, want symbolic default branch trunk", baseRef)
		}
	}
}

func TestProvisioningAdmissionIsAtomicAndFailedJobsCanRetry(t *testing.T) {
	repository := testStore(t)
	project := domain.Project{ID: uuid.NewString(), Slug: "demo", Name: "Demo", UnixUser: "soda-p-demo", Profile: domain.ToolchainGo, Source: domain.EmptyProjectSource{}}
	if err := repository.CreateProject(context.Background(), project); err != nil {
		t.Fatal(err)
	}
	installer := &blockingInstaller{started: make(chan struct{}), release: make(chan struct{})}
	service := New(Options{Store: repository, Host: &fakeHost{}, Toolchains: installer, ProjectsRoot: t.TempDir()})
	defer service.Close()
	assertConcurrentAdmission(t, service, project.ID)
	<-installer.started
	close(installer.release)
	waitForJobState(t, repository, project.ID, domain.JobReady)
	if _, err := service.StartProvisioning(context.Background(), &sodav2.StartProvisioningRequest{ProjectId: project.ID}); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("ready retry status = %s: %v", status.Code(err), err)
	}
	jobs, err := repository.Jobs(context.Background(), project.ID)
	if err != nil {
		t.Fatal(err)
	}
	latest := jobs[0]
	failure := "retry requested after failure"
	latest.State, latest.Error = domain.JobFailed, &failure
	if err = repository.UpdateJob(context.Background(), latest); err != nil {
		t.Fatal(err)
	}
	service.Close()
	retryService := New(Options{Store: repository, Host: &fakeHost{}, Toolchains: fakeInstaller{}, ProjectsRoot: t.TempDir()})
	defer retryService.Close()
	if _, err = retryService.StartProvisioning(context.Background(), &sodav2.StartProvisioningRequest{ProjectId: project.ID}); err != nil {
		t.Fatalf("failed job retry: %v", err)
	}
	waitForJobState(t, repository, project.ID, domain.JobReady)
}

func assertConcurrentAdmission(t *testing.T, service *Service, projectID string) {
	t.Helper()
	const attempts = 8
	results := make(chan error, attempts)
	start := make(chan struct{})
	for range attempts {
		go func() {
			<-start
			_, callErr := service.StartProvisioning(context.Background(), &sodav2.StartProvisioningRequest{ProjectId: projectID})
			results <- callErr
		}()
	}
	close(start)
	successes, preconditions := 0, 0
	for range attempts {
		callErr := <-results
		switch status.Code(callErr) {
		case codes.OK:
			successes++
		case codes.FailedPrecondition:
			preconditions++
		default:
			t.Fatalf("unexpected status %s: %v", status.Code(callErr), callErr)
		}
	}
	if successes != 1 || preconditions != attempts-1 {
		t.Fatalf("successes=%d preconditions=%d", successes, preconditions)
	}
}

func TestProvisioningRetryKeepsSuccessfulPersonalWorkspaces(t *testing.T) {
	repository := testStore(t)
	ctx := context.Background()
	alice := persistedPerson(t, repository, "alice")
	bob := persistedPerson(t, repository, "bob")
	project := domain.Project{ID: uuid.NewString(), Slug: "demo", Name: "Demo", UnixUser: "soda-p-demo", Profile: domain.ToolchainGo, Source: domain.EmptyProjectSource{}}
	if err := repository.CreateProjectWithMemberships(ctx, project, []string{alice.ID, bob.ID}); err != nil {
		t.Fatal(err)
	}
	hostSystem := &fakeHost{failWorktreeAt: 2}
	service := New(Options{Store: repository, Host: hostSystem, Toolchains: fakeInstaller{}, ProjectsRoot: t.TempDir()})
	defer service.Close()
	if _, err := service.StartProvisioning(ctx, &sodav2.StartProvisioningRequest{ProjectId: project.ID}); err != nil {
		t.Fatal(err)
	}
	waitForJobState(t, repository, project.ID, domain.JobFailed)
	workspaces, err := repository.Worktrees(ctx, project.ID)
	if err != nil || len(workspaces) != 1 || workspaces[0].PersonID != alice.ID {
		t.Fatalf("workspaces after partial setup = %#v, %v", workspaces, err)
	}
	hostSystem.mu.Lock()
	hostSystem.failWorktreeAt = 0
	hostSystem.mu.Unlock()
	assertWorkspaceRetryCompletes(t, ctx, service, repository, project.ID, hostSystem)
}

func assertWorkspaceRetryCompletes(t *testing.T, ctx context.Context, service *Service, repository *store.Store, projectID string, hostSystem *fakeHost) {
	t.Helper()
	if _, err := service.StartProvisioning(ctx, &sodav2.StartProvisioningRequest{ProjectId: projectID}); err != nil {
		t.Fatal(err)
	}
	waitForJobState(t, repository, projectID, domain.JobReady)
	workspaces, err := repository.Worktrees(ctx, projectID)
	if err != nil || len(workspaces) != 2 {
		t.Fatalf("workspaces after retry = %#v, %v", workspaces, err)
	}
	hostSystem.mu.Lock()
	defer hostSystem.mu.Unlock()
	if len(hostSystem.worktrees) != 2 || hostSystem.worktrees[0].PersonID == hostSystem.worktrees[1].PersonID {
		t.Fatalf("retry replaced or duplicated a successful workspace: %#v", hostSystem.worktrees)
	}
}

func TestCreatePersonUsesRelaxedSixCharacterPasswordPolicy(t *testing.T) {
	repository, err := store.Open(filepath.Join(t.TempDir(), "soda.db"))
	if err != nil {
		t.Fatal(err)
	}
	hostSystem := &fakeHost{}
	service := New(Options{Store: repository, Host: hostSystem, Toolchains: fakeInstaller{}, ProjectsRoot: t.TempDir()})
	defer service.Close()
	request := &sodav2.CreatePersonRequest{Username: "alice", DisplayName: "Alice", Email: "alice@example.test", Role: sodav2.Role_ROLE_DEVELOPER}
	for index, password := range []string{"simple", "with spaces", "colon:allowed"} {
		username := request.Username + string(rune('a'+index))
		candidate := &sodav2.CreatePersonRequest{Username: username, DisplayName: request.DisplayName, Email: username + "@example.test", Role: request.Role, Password: password}
		if _, err = service.CreatePerson(context.Background(), candidate); err != nil {
			t.Fatalf("password %q: %v", password, err)
		}
	}
	before := hostSystem.people
	for _, password := range []string{"short", "bad\x00password"} {
		request.Password = password
		if _, err = service.CreatePerson(context.Background(), request); status.Code(err) != codes.InvalidArgument {
			t.Fatalf("invalid password %q status = %s: %v", password, status.Code(err), err)
		}
	}
	if hostSystem.people != before {
		t.Fatalf("delimiter reached host: people=%d before=%d", hostSystem.people, before)
	}
}

func TestDuplicateSSHDeviceFingerprintIsRejectedGlobally(t *testing.T) {
	repository, err := store.Open(filepath.Join(t.TempDir(), "soda.db"))
	if err != nil {
		t.Fatal(err)
	}
	hostSystem := &fakeHost{}
	service := New(Options{Store: repository, Host: hostSystem, Toolchains: fakeInstaller{}, ProjectsRoot: t.TempDir()})
	defer service.Close()
	key := "ssh-ed25519 AAAA"
	alice, err := service.CreatePerson(context.Background(), &sodav2.CreatePersonRequest{Username: "alice", DisplayName: "Alice", Email: "alice@example.test", Role: sodav2.Role_ROLE_DEVELOPER, Password: "simple"})
	if err != nil {
		t.Fatal(err)
	}
	bob, err := service.CreatePerson(context.Background(), &sodav2.CreatePersonRequest{Username: "bob", DisplayName: "Bob", Email: "bob@example.test", Role: sodav2.Role_ROLE_DEVELOPER, Password: "simple"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.CreateSshDeviceKey(context.Background(), &sodav2.CreateSshDeviceKeyRequest{PersonId: alice.Person.Id, Label: "Laptop", PublicKey: key + " first", IdentityFileHint: "~/.ssh/id_ed25519"}); err != nil {
		t.Fatal(err)
	}
	_, err = service.CreateSshDeviceKey(context.Background(), &sodav2.CreateSshDeviceKeyRequest{PersonId: bob.Person.Id, Label: "Workstation", PublicKey: key + " second", IdentityFileHint: "~/.ssh/work"})
	if status.Code(err) != codes.AlreadyExists {
		t.Fatalf("duplicate fingerprint status = %s: %v", status.Code(err), err)
	}
	if hostSystem.people != 2 {
		t.Fatalf("device-key conflict changed person provisioning: people=%d", hostSystem.people)
	}
}

func TestSSHDeviceChangesReconcileAccessAndRollbackOnFilesystemFailure(t *testing.T) {
	repository := testStore(t)
	ctx := context.Background()
	person := persistedPerson(t, repository, "alice")
	project := domain.Project{ID: uuid.NewString(), Slug: "demo", Name: "Demo", UnixUser: "soda-p-demo", Profile: domain.ToolchainGo, Source: domain.EmptyProjectSource{}}
	if err := repository.CreateProjectWithMemberships(ctx, project, []string{person.ID}); err != nil {
		t.Fatal(err)
	}
	workspace := domain.Worktree{ID: uuid.NewString(), ProjectID: project.ID, PersonID: person.ID, Name: "default", Branch: "people/alice", Path: filepath.Join(t.TempDir(), "alice")}
	if err := repository.CreateWorktree(ctx, workspace); err != nil {
		t.Fatal(err)
	}
	hostSystem := &fakeHost{}
	service := New(Options{Store: repository, Host: hostSystem, Toolchains: fakeInstaller{}, ProjectsRoot: t.TempDir()})
	defer service.Close()
	created, err := service.CreateSshDeviceKey(ctx, &sodav2.CreateSshDeviceKeyRequest{PersonId: person.ID, Label: "Laptop", PublicKey: "ssh-ed25519 AAAA alice", IdentityFileHint: "~/.ssh/id_ed25519"})
	if err != nil {
		t.Fatal(err)
	}
	if len(hostSystem.reconciliations) != 1 || len(hostSystem.reconciliations[0]) != 1 || len(hostSystem.reconciliations[0][0].Keys) != 1 {
		t.Fatalf("create reconciliation = %#v", hostSystem.reconciliations)
	}
	assertFailedDeviceKeyRevocationRestoresKey(t, ctx, service, repository, hostSystem, person.ID, created.Key.Id)
}

func assertFailedDeviceKeyRevocationRestoresKey(t *testing.T, ctx context.Context, service *Service, repository *store.Store, hostSystem *fakeHost, personID, keyID string) {
	t.Helper()
	hostSystem.reconcileErr = errors.New("authorized_keys unavailable")
	if _, err := service.RevokeSshDeviceKey(ctx, &sodav2.RevokeSshDeviceKeyRequest{PersonId: personID, KeyId: keyID}); status.Code(err) != codes.Internal {
		t.Fatalf("revoke status = %s: %v", status.Code(err), err)
	}
	keys, err := repository.SSHDeviceKeys(ctx, personID)
	if err != nil || len(keys) != 1 || keys[0].ID != keyID {
		t.Fatalf("failed revoke did not restore device key: %#v, %v", keys, err)
	}
}

func TestStartupReconciliationRepairsProjectAccessFromStoredState(t *testing.T) {
	repository := testStore(t)
	ctx := context.Background()
	person := persistedPerson(t, repository, "alice")
	project := domain.Project{ID: uuid.NewString(), Slug: "demo", Name: "Demo", UnixUser: "soda-p-demo", Profile: domain.ToolchainGo, Source: domain.EmptyProjectSource{}}
	if err := repository.CreateProjectWithMemberships(ctx, project, []string{person.ID}); err != nil {
		t.Fatal(err)
	}
	workspace := domain.Worktree{ID: uuid.NewString(), ProjectID: project.ID, PersonID: person.ID, Name: "default", Branch: "people/alice", Path: filepath.Join(t.TempDir(), "alice")}
	if err := repository.CreateWorktree(ctx, workspace); err != nil {
		t.Fatal(err)
	}
	key := domain.SSHDeviceKey{ID: uuid.NewString(), PersonID: person.ID, Label: "Laptop", PublicKey: "ssh-ed25519 AAAA alice", Fingerprint: "SHA256:test", IdentityFileHint: "~/.ssh/id_ed25519", CreatedAt: time.Now().UTC()}
	if err := repository.CreateSSHDeviceKey(ctx, key); err != nil {
		t.Fatal(err)
	}
	hostSystem := &fakeHost{}
	service := New(Options{Store: repository, Host: hostSystem, Toolchains: fakeInstaller{}, ProjectsRoot: t.TempDir()})
	defer service.Close()
	if err := service.ReconcileAllAuthorizedKeys(ctx); err != nil {
		t.Fatal(err)
	}
	if len(hostSystem.reconciliations) != 1 || len(hostSystem.reconciliations[0]) != 1 {
		t.Fatalf("startup reconciliation = %#v", hostSystem.reconciliations)
	}
	access := hostSystem.reconciliations[0][0]
	if len(access.Keys) != 1 {
		t.Fatalf("startup access keys = %#v", access)
	}
	got := [3]string{access.Person.ID, access.Worktree.ID, access.Keys[0].ID}
	want := [3]string{person.ID, workspace.ID, key.ID}
	if got != want {
		t.Fatalf("startup access = %#v, want %v", access, want)
	}
}

func TestFailedPersistenceCompensatesFreshHostResources(t *testing.T) {
	t.Run("person", testFailedPersonPersistence)
	t.Run("project and provisioning admission", testFailedProvisioningAdmission)
	t.Run("project database cleanup failure preserves host resources", testFailedProjectDatabaseCleanup)
	t.Run("project persistence", testFailedProjectPersistence)
}

func testFailedPersonPersistence(t *testing.T) {
	repository := testStore(t)
	injectCreateFailure(t, repository, "Person")
	hostSystem := &fakeHost{}
	service := New(Options{Store: repository, Host: hostSystem, Toolchains: fakeInstaller{}, ProjectsRoot: t.TempDir()})
	defer service.Close()
	_, err := service.CreatePerson(context.Background(), &sodav2.CreatePersonRequest{Username: "alice", DisplayName: "Alice", Email: "alice@example.test", Role: sodav2.Role_ROLE_DEVELOPER, Password: "simple"})
	if err == nil || hostSystem.personCleanups != 1 {
		t.Fatalf("error=%v cleanups=%d", err, hostSystem.personCleanups)
	}
	assertTableCount(t, repository, "people", 0)
}

func testFailedProvisioningAdmission(t *testing.T) {
	repository := testStore(t)
	person := persistedPerson(t, repository, "alice")
	injectCreateFailure(t, repository, "ProvisioningJob")
	hostSystem := &fakeHost{}
	service := New(Options{Store: repository, Host: hostSystem, Toolchains: fakeInstaller{}, ProjectsRoot: t.TempDir()})
	defer service.Close()
	_, err := service.CreateProject(context.Background(), emptyProjectRequest(person.ID))
	if err == nil || hostSystem.projectCleanups != 1 {
		t.Fatalf("error=%v cleanups=%d", err, hostSystem.projectCleanups)
	}
	assertTableCount(t, repository, "projects", 0)
}

func testFailedProjectDatabaseCleanup(t *testing.T) {
	repository := testStore(t)
	person := persistedPerson(t, repository, "alice")
	injectCreateFailure(t, repository, "ProvisioningJob")
	injectDeleteFailure(t, repository, "Project")
	hostSystem := &fakeHost{}
	service := New(Options{Store: repository, Host: hostSystem, Toolchains: fakeInstaller{}, ProjectsRoot: t.TempDir()})
	defer service.Close()
	_, err := service.CreateProject(context.Background(), emptyProjectRequest(person.ID))
	if status.Code(err) != codes.Internal {
		t.Fatalf("status = %s, error = %v", status.Code(err), err)
	}
	if hostSystem.projectCleanups != 0 {
		t.Fatalf("host cleanup ran despite durable project: %d", hostSystem.projectCleanups)
	}
	assertTableCount(t, repository, "projects", 1)
}

func testFailedProjectPersistence(t *testing.T) {
	repository := testStore(t)
	person := persistedPerson(t, repository, "alice")
	injectCreateFailure(t, repository, "Project")
	hostSystem := &fakeHost{}
	service := New(Options{Store: repository, Host: hostSystem, Toolchains: fakeInstaller{}, ProjectsRoot: t.TempDir()})
	defer service.Close()
	_, err := service.CreateProject(context.Background(), emptyProjectRequest(person.ID))
	if err == nil || hostSystem.projectCleanups != 1 {
		t.Fatalf("error=%v cleanups=%d", err, hostSystem.projectCleanups)
	}
	assertTableCount(t, repository, "projects", 0)
}

func TestCompensationPreservesOriginalAndAttachesCleanupFailure(t *testing.T) {
	original := errors.New("persistence failed")
	cleanupFailure := errors.New("cleanup failed")
	service := &Service{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	result := service.compensate(context.Background(), original, func(context.Context) error { return cleanupFailure }, "person", "alice")
	if !errors.Is(result, original) || !errors.Is(result, cleanupFailure) {
		t.Fatalf("compensation error = %v", result)
	}
}

func TestFailedMembershipAndWorktreePersistenceCompensatesHostWorktrees(t *testing.T) {
	repository, err := store.Open(filepath.Join(t.TempDir(), "soda.db"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	person := domain.Person{ID: uuid.NewString(), Username: "alice", DisplayName: "Alice", Email: "alice@example.test", Role: domain.RoleDeveloper}
	project := domain.Project{ID: uuid.NewString(), Slug: "demo", Name: "Demo", UnixUser: "soda-p-demo", Profile: domain.ToolchainGo, Source: domain.EmptyProjectSource{}}
	if err = repository.CreatePerson(ctx, person); err != nil {
		t.Fatal(err)
	}
	if err = repository.CreateProject(ctx, project); err != nil {
		t.Fatal(err)
	}
	if err = repository.CreateJob(ctx, domain.ProvisioningJob{ID: uuid.NewString(), ProjectID: project.ID, State: domain.JobReady}); err != nil {
		t.Fatal(err)
	}
	injectCreateFailure(t, repository, "Membership")
	hostSystem := &fakeHost{}
	service := New(Options{Store: repository, Host: hostSystem, Toolchains: fakeInstaller{}, ProjectsRoot: t.TempDir()})
	defer service.Close()
	if _, err = service.AddCollaborator(ctx, &sodav2.AddCollaboratorRequest{ProjectId: project.ID, PersonId: person.ID}); err == nil {
		t.Fatal("expected membership persistence failure")
	}
	if hostSystem.worktreeCleanups != 1 {
		t.Fatalf("worktree cleanups = %d", hostSystem.worktreeCleanups)
	}
	assertTableCount(t, repository, "memberships", 0)
	assertTableCount(t, repository, "worktrees", 0)
}

func TestProvisioningDeadlineMarksFailedAndAllowsManualRetry(t *testing.T) {
	repository, err := store.Open(filepath.Join(t.TempDir(), "soda.db"))
	if err != nil {
		t.Fatal(err)
	}
	project := domain.Project{ID: uuid.NewString(), Slug: "demo", Name: "Demo", UnixUser: "soda-p-demo", Profile: domain.ToolchainGo, Source: domain.EmptyProjectSource{}}
	if err = repository.CreateProject(context.Background(), project); err != nil {
		t.Fatal(err)
	}
	service := New(Options{Store: repository, Host: &fakeHost{}, Toolchains: timeoutInstaller{}, ProjectsRoot: t.TempDir(), ProvisioningTimeout: 25 * time.Millisecond})
	defer service.Close()
	if _, err = service.StartProvisioning(context.Background(), &sodav2.StartProvisioningRequest{ProjectId: project.ID}); err != nil {
		t.Fatal(err)
	}
	waitForJobState(t, repository, project.ID, domain.JobFailed)
	jobs, err := repository.Jobs(context.Background(), project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if jobs[0].Error == nil || !strings.Contains(*jobs[0].Error, context.DeadlineExceeded.Error()) {
		t.Fatalf("timeout job = %#v", jobs[0])
	}
	if _, err = service.StartProvisioning(context.Background(), &sodav2.StartProvisioningRequest{ProjectId: project.ID}); err != nil {
		t.Fatalf("manual retry after timeout: %v", err)
	}
	waitForJobState(t, repository, project.ID, domain.JobFailed)
}

func injectCreateFailure(t *testing.T, repository *store.Store, model string) {
	t.Helper()
	name := "soda:fail-create:" + model
	if err := repository.DB().Callback().Create().Before("gorm:create").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == model {
			tx.AddError(errors.New("injected " + model + " persistence failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
}

func persistedPerson(t *testing.T, repository *store.Store, username string) domain.Person {
	t.Helper()
	person := domain.Person{ID: uuid.NewString(), Username: username, DisplayName: strings.ToUpper(username[:1]) + username[1:], Email: username + "@example.test", Role: domain.RoleDeveloper}
	if err := repository.CreatePerson(context.Background(), person); err != nil {
		t.Fatal(err)
	}
	return person
}

func emptyProjectRequest(personID string) *sodav2.CreateProjectRequest {
	return &sodav2.CreateProjectRequest{
		Slug: "demo", Name: "Demo", Profile: sodav2.ToolchainProfile_TOOLCHAIN_PROFILE_GO,
		Source:           &sodav2.ProjectSource{Source: &sodav2.ProjectSource_Empty{Empty: &sodav2.EmptyProjectSource{}}},
		InitialPersonIds: []string{personID},
	}
}

func injectDeleteFailure(t *testing.T, repository *store.Store, model string) {
	t.Helper()
	name := "soda:fail-delete:" + model
	if err := repository.DB().Callback().Delete().Before("gorm:delete").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == model {
			tx.AddError(errors.New("injected " + model + " cleanup failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
}

func assertTableCount(t *testing.T, repository *store.Store, table string, want int64) {
	t.Helper()
	var count int64
	if err := repository.DB().Table(table).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("%s count = %d, want %d", table, count, want)
	}
}

func waitForJobState(t *testing.T, repository *store.Store, projectID string, want domain.JobState) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		jobs, err := repository.Jobs(context.Background(), projectID)
		if err != nil {
			t.Fatal(err)
		}
		if len(jobs) > 0 && jobs[0].State == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("latest job did not reach %s: %#v", want, jobs)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
