package daemon

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/LevitateOS/soda-os/internal/domain"
	sodav2 "github.com/LevitateOS/soda-os/internal/gen/soda/v2"
	"github.com/LevitateOS/soda-os/internal/store"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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

func TestCreatePersonPersistsPublicGitIdentityForRetrieval(t *testing.T) {
	repository := testStore(t)
	service := New(Options{Store: repository, Host: &fakeHost{}, Toolchains: fakeInstaller{}, ProjectsRoot: t.TempDir()})
	defer service.Close()
	created, err := service.CreatePerson(context.Background(), &sodav2.CreatePersonRequest{
		Username: "alice", DisplayName: "Alice", Email: "alice@example.test",
		Role: sodav2.Role_ROLE_DEVELOPER, Password: "simple",
	})
	if err != nil {
		t.Fatal(err)
	}
	response, err := service.GetGitIdentity(context.Background(), &sodav2.GetGitIdentityRequest{PersonId: created.Person.Id})
	if err != nil {
		t.Fatal(err)
	}
	identity := response.GetIdentity()
	if identity.GetPersonId() != created.Person.Id || identity.GetPublicKey() == "" || identity.GetFingerprint() == "" {
		t.Fatalf("Git identity = %#v", identity)
	}
	descriptor := identity.ProtoReflect().Descriptor()
	if descriptor.Fields().ByName("private_key") != nil || descriptor.Fields().ByName("private_key_path") != nil {
		t.Fatalf("private Git key field exposed in API: %s", descriptor.FullName())
	}
}

func assertDuplicatePreflightsHostState(t *testing.T, hostSystem *fakeHost) {
	t.Helper()
	hostSystem.mu.Lock()
	defer hostSystem.mu.Unlock()
	if hostSystem.people.created != 2 || hostSystem.people.imported != 0 || len(hostSystem.workspaces.projects) != 1 || len(hostSystem.workspaces.worktrees) != 2 {
		t.Fatalf("host mutations: people=%d imports=%d projects=%d worktrees=%d", hostSystem.people.created, hostSystem.people.imported, len(hostSystem.workspaces.projects), len(hostSystem.workspaces.worktrees))
	}
	for _, baseRef := range hostSystem.workspaces.baseRefs {
		if baseRef != "trunk" {
			t.Fatalf("workspace base ref = %q, want symbolic default branch trunk", baseRef)
		}
	}
}

func TestProvisioningAdmissionIsAtomicAndFailedJobsCanRetry(t *testing.T) {
	repository := testStore(t)
	project := domain.Project{ID: uuid.NewString(), Slug: "demo", Name: "Demo", UnixUser: "soda-p-demo", Profile: domain.ToolchainGo, Source: domain.EmptyProjectSource{}}
	if err := repository.CreateProjectWithMemberships(context.Background(), project, nil); err != nil {
		t.Fatal(err)
	}
	installer := &blockingInstaller{started: make(chan struct{}), release: make(chan struct{})}
	service := New(Options{Store: repository, Host: &fakeHost{}, Toolchains: installer, ProjectsRoot: t.TempDir()})
	defer service.Close()
	results := make(chan error, 2)
	ready := make(chan struct{}, 2)
	start := make(chan struct{})
	startProvisioning := func() {
		go func() {
			ready <- struct{}{}
			<-start
			_, callErr := service.StartProvisioning(context.Background(), &sodav2.StartProvisioningRequest{ProjectId: project.ID})
			results <- callErr
		}()
	}
	startProvisioning()
	startProvisioning()
	<-ready
	<-ready
	close(start)
	statuses := map[codes.Code]int{status.Code(<-results): 1}
	statuses[status.Code(<-results)]++
	if statuses[codes.OK] != 1 || statuses[codes.FailedPrecondition] != 1 {
		t.Fatalf("provisioning admission statuses = %#v", statuses)
	}
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
func TestProvisioningFailureKeepsSuccessfulPersonalWorkspace(t *testing.T) {
	repository := testStore(t)
	ctx := context.Background()
	alice := persistedPerson(t, repository, "alice")
	bob := persistedPerson(t, repository, "bob")
	project := domain.Project{ID: uuid.NewString(), Slug: "demo", Name: "Demo", UnixUser: "soda-p-demo", Profile: domain.ToolchainGo, Source: domain.EmptyProjectSource{}}
	if err := repository.CreateProjectWithMemberships(ctx, project, []string{alice.ID, bob.ID}); err != nil {
		t.Fatal(err)
	}
	hostSystem := &fakeHost{workspaces: workspaceEvents{failAt: 2}}
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
}

func TestProvisioningRetryAddsOnlyMissingPersonalWorkspace(t *testing.T) {
	repository := testStore(t)
	ctx := context.Background()
	alice := persistedPerson(t, repository, "alice")
	bob := persistedPerson(t, repository, "bob")
	project := domain.Project{ID: uuid.NewString(), Slug: "demo", Name: "Demo", UnixUser: "soda-p-demo", Profile: domain.ToolchainGo, Source: domain.EmptyProjectSource{}}
	if err := repository.CreateProjectWithMemberships(ctx, project, []string{alice.ID, bob.ID}); err != nil {
		t.Fatal(err)
	}
	hostSystem := &fakeHost{workspaces: workspaceEvents{failAt: 2}}
	service := New(Options{Store: repository, Host: hostSystem, Toolchains: fakeInstaller{}, ProjectsRoot: t.TempDir()})
	defer service.Close()
	if _, err := service.StartProvisioning(ctx, &sodav2.StartProvisioningRequest{ProjectId: project.ID}); err != nil {
		t.Fatal(err)
	}
	waitForJobState(t, repository, project.ID, domain.JobFailed)
	hostSystem.mu.Lock()
	hostSystem.workspaces.failAt = 0
	hostSystem.mu.Unlock()
	if _, err := service.StartProvisioning(ctx, &sodav2.StartProvisioningRequest{ProjectId: project.ID}); err != nil {
		t.Fatal(err)
	}
	waitForJobState(t, repository, project.ID, domain.JobReady)
	workspaces, err := repository.Worktrees(ctx, project.ID)
	if err != nil || len(workspaces) != 2 {
		t.Fatalf("workspaces after retry = %#v, %v", workspaces, err)
	}
	hostSystem.mu.Lock()
	defer hostSystem.mu.Unlock()
	if len(hostSystem.workspaces.worktrees) != 2 || hostSystem.workspaces.worktrees[0].PersonID == hostSystem.workspaces.worktrees[1].PersonID {
		t.Fatalf("retry replaced or duplicated a successful workspace: %#v", hostSystem.workspaces.worktrees)
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
	before := hostSystem.people.created
	for _, password := range []string{"short", "bad\x00password"} {
		request.Password = password
		if _, err = service.CreatePerson(context.Background(), request); status.Code(err) != codes.InvalidArgument {
			t.Fatalf("invalid password %q status = %s: %v", password, status.Code(err), err)
		}
	}
	if hostSystem.people.created != before {
		t.Fatalf("delimiter reached host: people=%d before=%d", hostSystem.people.created, before)
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
	if hostSystem.people.created != 2 {
		t.Fatalf("device-key conflict changed person provisioning: people=%d", hostSystem.people.created)
	}
}

func TestSSHDeviceCreationReconcilesProjectAccess(t *testing.T) {
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
	_, err := service.CreateSshDeviceKey(ctx, &sodav2.CreateSshDeviceKeyRequest{PersonId: person.ID, Label: "Laptop", PublicKey: "ssh-ed25519 AAAA alice", IdentityFileHint: "~/.ssh/id_ed25519"})
	if err != nil {
		t.Fatal(err)
	}
	if len(hostSystem.access.reconciliations) != 1 || len(hostSystem.access.reconciliations[0].keys) != 1 {
		t.Fatalf("create reconciliation = %#v", hostSystem.access.reconciliations)
	}
}

func TestFailedSSHDeviceRevocationRestoresDurableKey(t *testing.T) {
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
	hostSystem.access.err = errors.New("authorized_keys unavailable")
	if _, err := service.RevokeSshDeviceKey(ctx, &sodav2.RevokeSshDeviceKeyRequest{PersonId: person.ID, KeyId: created.Key.Id}); status.Code(err) != codes.Internal {
		t.Fatalf("revoke status = %s: %v", status.Code(err), err)
	}
	keys, err := repository.SSHDeviceKeys(ctx, person.ID)
	if err != nil || len(keys) != 1 || keys[0].ID != created.Key.Id {
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
	if len(hostSystem.access.reconciliations) != 1 {
		t.Fatalf("startup reconciliation = %#v", hostSystem.access.reconciliations)
	}
	access := hostSystem.access.reconciliations[0]
	if len(access.keys) != 1 {
		t.Fatalf("startup access keys = %#v", access)
	}
	got := [2]string{access.person.ID, access.keys[0].ID}
	want := [2]string{person.ID, key.ID}
	if got != want {
		t.Fatalf("startup access = %#v, want %v", access, want)
	}
}
