package daemon

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LevitateOS/soda-os/internal/domain"
	sodav2 "github.com/LevitateOS/soda-os/internal/gen/soda/v2"
	"github.com/LevitateOS/soda-os/internal/store"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

func TestFailedPersonPersistenceCompensatesHostAccount(t *testing.T) {
	repository := testStore(t)
	injectCreateFailure(t, repository, "Person")
	hostSystem := &fakeHost{}
	service := New(Options{Store: repository, Host: hostSystem, Toolchains: fakeInstaller{}, ProjectsRoot: t.TempDir()})
	defer service.Close()
	_, err := service.CreatePerson(context.Background(), &sodav2.CreatePersonRequest{Username: "alice", DisplayName: "Alice", Email: "alice@example.test", Role: sodav2.Role_ROLE_DEVELOPER, Password: "simple"})
	if err == nil || hostSystem.people.cleanups != 1 {
		t.Fatalf("error=%v cleanups=%d", err, hostSystem.people.cleanups)
	}
	assertTableCount(t, repository, "people", 0)
}

func TestFailedProvisioningAdmissionCompensatesHostProject(t *testing.T) {
	repository := testStore(t)
	person := persistedPerson(t, repository, "alice")
	injectCreateFailure(t, repository, "ProvisioningJob")
	hostSystem := &fakeHost{}
	service := New(Options{Store: repository, Host: hostSystem, Toolchains: fakeInstaller{}, ProjectsRoot: t.TempDir()})
	defer service.Close()
	_, err := service.CreateProject(context.Background(), emptyProjectRequest(person.ID))
	if err == nil || hostSystem.workspaces.projectCleanups != 1 {
		t.Fatalf("error=%v cleanups=%d", err, hostSystem.workspaces.projectCleanups)
	}
	assertTableCount(t, repository, "projects", 0)
}

func TestFailedProjectDatabaseCleanupPreservesHostProject(t *testing.T) {
	repository := testStore(t)
	person := persistedPerson(t, repository, "alice")
	injectCreateFailure(t, repository, "ProvisioningJob")
	if err := repository.DB().Callback().Delete().Before("gorm:delete").Register("soda:fail-delete:Project", func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "Project" {
			tx.AddError(errors.New("injected Project cleanup failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	hostSystem := &fakeHost{}
	service := New(Options{Store: repository, Host: hostSystem, Toolchains: fakeInstaller{}, ProjectsRoot: t.TempDir()})
	defer service.Close()
	_, err := service.CreateProject(context.Background(), emptyProjectRequest(person.ID))
	if status.Code(err) != codes.Internal {
		t.Fatalf("status = %s, error = %v", status.Code(err), err)
	}
	if hostSystem.workspaces.projectCleanups != 0 {
		t.Fatalf("host cleanup ran despite durable project: %d", hostSystem.workspaces.projectCleanups)
	}
	assertTableCount(t, repository, "projects", 1)
}

func TestFailedProjectPersistenceCompensatesHostProject(t *testing.T) {
	repository := testStore(t)
	person := persistedPerson(t, repository, "alice")
	injectCreateFailure(t, repository, "Project")
	hostSystem := &fakeHost{}
	service := New(Options{Store: repository, Host: hostSystem, Toolchains: fakeInstaller{}, ProjectsRoot: t.TempDir()})
	defer service.Close()
	_, err := service.CreateProject(context.Background(), emptyProjectRequest(person.ID))
	if err == nil || hostSystem.workspaces.projectCleanups != 1 {
		t.Fatalf("error=%v cleanups=%d", err, hostSystem.workspaces.projectCleanups)
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
	if err = repository.CreateProjectWithMemberships(ctx, project, nil); err != nil {
		t.Fatal(err)
	}
	if err = repository.DB().WithContext(ctx).Create(&store.ProvisioningJob{ID: uuid.NewString(), ProjectID: project.ID, State: string(domain.JobReady)}).Error; err != nil {
		t.Fatal(err)
	}
	injectCreateFailure(t, repository, "Membership")
	hostSystem := &fakeHost{}
	service := New(Options{Store: repository, Host: hostSystem, Toolchains: fakeInstaller{}, ProjectsRoot: t.TempDir()})
	defer service.Close()
	if _, err = service.AddCollaborator(ctx, &sodav2.AddCollaboratorRequest{ProjectId: project.ID, PersonId: person.ID}); err == nil {
		t.Fatal("expected membership persistence failure")
	}
	if hostSystem.workspaces.worktreeCleanups != 1 {
		t.Fatalf("worktree cleanups = %d", hostSystem.workspaces.worktreeCleanups)
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
	if err = repository.CreateProjectWithMemberships(context.Background(), project, nil); err != nil {
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

func TestExternalProjectWaitsForAccessAndRetriesWithBootstrapPerson(t *testing.T) {
	repository := testStore(t)
	hostSystem := &fakeHost{}
	service := New(Options{Store: repository, Host: hostSystem, Toolchains: fakeInstaller{}, ProjectsRoot: t.TempDir()})
	defer service.Close()
	personResponse, err := service.CreatePerson(context.Background(), &sodav2.CreatePersonRequest{
		Username: "alice", DisplayName: "Alice", Email: "alice@example.test",
		Role: sodav2.Role_ROLE_ADMIN, Password: "simple",
	})
	require.NoError(t, err)
	personID := personResponse.Person.Id
	response, err := service.CreateProject(context.Background(), &sodav2.CreateProjectRequest{
		Slug: "external", Name: "External", Profile: sodav2.ToolchainProfile_TOOLCHAIN_PROFILE_GO,
		Source:           &sodav2.ProjectSource{Source: &sodav2.ProjectSource_Git{Git: &sodav2.GitProjectSource{RemoteUrl: "git@example.test:team/external.git"}}},
		InitialPersonIds: []string{personID}, BootstrapPersonId: personID,
	})
	require.NoError(t, err)
	projectID := response.Project.Id
	jobs, err := repository.Jobs(context.Background(), projectID)
	require.NoError(t, err)
	require.Empty(t, jobs)
	project, err := repository.Project(context.Background(), projectID)
	require.NoError(t, err)
	require.Equal(t, personID, project.BootstrapPersonID)
	require.Len(t, hostSystem.workspaces.projects, 1)
	require.Empty(t, hostSystem.workspaces.worktrees)

	hostSystem.workspaces.repositoryErr = errors.New("repository access denied")
	_, err = service.StartProvisioning(context.Background(), &sodav2.StartProvisioningRequest{ProjectId: projectID})
	require.NoError(t, err)
	waitForJobState(t, repository, projectID, domain.JobFailed)
	hostSystem.mu.Lock()
	hostSystem.workspaces.repositoryErr = nil
	hostSystem.mu.Unlock()
	_, err = service.StartProvisioning(context.Background(), &sodav2.StartProvisioningRequest{ProjectId: projectID})
	require.NoError(t, err)
	waitForJobState(t, repository, projectID, domain.JobReady)

	hostSystem.mu.Lock()
	defer hostSystem.mu.Unlock()
	require.Len(t, hostSystem.workspaces.repositoryPeople, 2)
	for _, bootstrap := range hostSystem.workspaces.repositoryPeople {
		require.Equal(t, personID, bootstrap.ID)
		require.Equal(t, "alice", bootstrap.Username)
	}
}

func TestExternalProjectRequiresInitialMemberWithGitIdentity(t *testing.T) {
	repository := testStore(t)
	service := New(Options{Store: repository, Host: &fakeHost{}, Toolchains: fakeInstaller{}, ProjectsRoot: t.TempDir()})
	defer service.Close()
	alice := persistedPerson(t, repository, "alice")
	bob := persistedPerson(t, repository, "bob")
	source := &sodav2.ProjectSource{Source: &sodav2.ProjectSource_Git{Git: &sodav2.GitProjectSource{RemoteUrl: "ssh://git@example.test/team/demo.git"}}}
	base := &sodav2.CreateProjectRequest{Slug: "demo", Name: "Demo", Profile: sodav2.ToolchainProfile_TOOLCHAIN_PROFILE_GO, Source: source, InitialPersonIds: []string{alice.ID}}

	if _, err := service.CreateProject(context.Background(), base); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("missing bootstrap status = %s: %v", status.Code(err), err)
	}
	base.BootstrapPersonId = bob.ID
	if _, err := service.CreateProject(context.Background(), base); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("non-member bootstrap status = %s: %v", status.Code(err), err)
	}
	base.BootstrapPersonId = alice.ID
	if _, err := service.CreateProject(context.Background(), base); status.Code(err) != codes.NotFound {
		t.Fatalf("missing Git identity status = %s: %v", status.Code(err), err)
	}
	base.Source = &sodav2.ProjectSource{Source: &sodav2.ProjectSource_Empty{Empty: &sodav2.EmptyProjectSource{}}}
	if _, err := service.CreateProject(context.Background(), base); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("built-in bootstrap status = %s: %v", status.Code(err), err)
	}
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
