package store

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/LevitateOS/soda-os/internal/domain"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestOpenCreatesSchemaVersionTwoAndEnforcesConstraints(t *testing.T) {
	path := filepath.Join(t.TempDir(), "soda.db")
	repository, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	var version int
	if err = repository.DB().Raw("PRAGMA user_version").Scan(&version).Error; err != nil {
		t.Fatal(err)
	}
	if version != SchemaVersion {
		t.Fatalf("schema version = %d", version)
	}
	ctx := context.Background()
	person := domain.Person{ID: uuid.NewString(), Username: "alice", DisplayName: "Alice", Email: "alice@example.test", Role: domain.RoleDeveloper}
	if err = repository.CreatePerson(ctx, person); err != nil {
		t.Fatal(err)
	}
	duplicate := person
	duplicate.ID = uuid.NewString()
	if err = repository.CreatePerson(ctx, duplicate); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("duplicate error = %v", err)
	}
	project := domain.Project{ID: uuid.NewString(), Slug: "demo", Name: "Demo", UnixUser: "soda-p-demo", Profile: domain.ToolchainGo, Source: domain.EmptyProjectSource{}}
	if err = repository.CreateProjectWithMemberships(ctx, project, nil); err != nil {
		t.Fatal(err)
	}
	remote := "https://example.test/contradictory.git"
	contradictory := Project{ID: uuid.NewString(), Slug: "bad-source", Name: "Bad", UnixUser: "soda-p-bad-source", Profile: "go", SourceKind: "empty", SourceRemoteURL: &remote}
	if err = repository.DB().WithContext(ctx).Create(&contradictory).Error; err == nil {
		t.Fatal("contradictory normalized project source accepted")
	}
	invalid := domain.Worktree{ID: uuid.NewString(), ProjectID: project.ID, PersonID: uuid.NewString(), Name: "default", Branch: "people/missing", Path: "/tmp/missing"}
	if err = repository.CreateWorktree(ctx, invalid); err == nil {
		t.Fatal("foreign key violation accepted")
	}
}

func TestOpenRejectsUnsupportedSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "soda.db")
	repository, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := repository.DB().DB()
	if err != nil {
		t.Fatal(err)
	}
	if err = repository.DB().Exec("PRAGMA user_version = 1").Error; err != nil {
		t.Fatal(err)
	}
	if err = sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
	_, err = Open(path)
	if !errors.Is(err, ErrUnsupportedSchema) {
		t.Fatalf("error = %v", err)
	}
}

func TestSchemaInitializationRollsBackTablesAndVersionTogether(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "soda.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	injected := errors.New("injected version failure")
	if err = initializeSchema(db, func(*gorm.DB) error { return injected }); !errors.Is(err, injected) {
		t.Fatalf("initialization error = %v", err)
	}
	var version int
	if err = db.Raw("PRAGMA user_version").Scan(&version).Error; err != nil {
		t.Fatal(err)
	}
	var tables int64
	if err = db.Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Scan(&tables).Error; err != nil {
		t.Fatal(err)
	}
	if version != 0 || tables != 0 {
		t.Fatalf("partial schema remained: version=%d tables=%d", version, tables)
	}
}

func TestSSHDeviceKeyUniquenessAllowsKeylessPeople(t *testing.T) {
	repository, err := Open(filepath.Join(t.TempDir(), "soda.db"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	first := domain.Person{ID: uuid.NewString(), Username: "alice", DisplayName: "Alice", Email: "alice@example.test", Role: domain.RoleDeveloper}
	second := domain.Person{ID: uuid.NewString(), Username: "bob", DisplayName: "Bob", Email: "bob@example.test", Role: domain.RoleDeveloper}
	for _, person := range []domain.Person{first, second} {
		if err = repository.CreatePerson(ctx, person); err != nil {
			t.Fatal(err)
		}
	}
	firstKey := domain.SSHDeviceKey{ID: uuid.NewString(), PersonID: first.ID, Label: "Laptop", PublicKey: "ssh-ed25519 AAAA first", Fingerprint: "SHA256:same", IdentityFileHint: "~/.ssh/id_ed25519"}
	if err = repository.CreateSSHDeviceKey(ctx, firstKey); err != nil {
		t.Fatal(err)
	}
	duplicateFingerprint := domain.SSHDeviceKey{ID: uuid.NewString(), PersonID: second.ID, Label: "Laptop", PublicKey: "ssh-ed25519 AAAA second", Fingerprint: firstKey.Fingerprint, IdentityFileHint: "~/.ssh/work"}
	if err = repository.CreateSSHDeviceKey(ctx, duplicateFingerprint); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("global fingerprint constraint = %v", err)
	}
	duplicateLabel := firstKey
	duplicateLabel.ID = uuid.NewString()
	duplicateLabel.Fingerprint = "SHA256:other"
	if err = repository.CreateSSHDeviceKey(ctx, duplicateLabel); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("per-person label constraint = %v", err)
	}
}

func TestSSHDeviceFingerprintConstraintIsConcurrent(t *testing.T) {
	repository, err := Open(filepath.Join(t.TempDir(), "soda.db"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	people := []domain.Person{
		{ID: uuid.NewString(), Username: "alice", DisplayName: "Alice", Email: "alice@example.test", Role: domain.RoleDeveloper},
		{ID: uuid.NewString(), Username: "bob", DisplayName: "Bob", Email: "bob@example.test", Role: domain.RoleDeveloper},
	}
	for _, person := range people {
		if err = repository.CreatePerson(ctx, person); err != nil {
			t.Fatal(err)
		}
	}
	start := make(chan struct{})
	errorsByAttempt := make(chan error, len(people))
	var ready sync.WaitGroup
	ready.Add(len(people))
	attempt := func(index int, person domain.Person) {
		go func() {
			ready.Done()
			<-start
			errorsByAttempt <- repository.CreateSSHDeviceKey(ctx, domain.SSHDeviceKey{ID: uuid.NewString(), PersonID: person.ID, Label: fmt.Sprintf("device-%d", index), PublicKey: fmt.Sprintf("ssh-ed25519 AAAA comment-%d", index), Fingerprint: "SHA256:shared", IdentityFileHint: "~/.ssh/id_ed25519"})
		}()
	}
	attempt(0, people[0])
	attempt(1, people[1])
	ready.Wait()
	close(start)
	successes, duplicates := 0, 0
	for range 2 {
		switch err = <-errorsByAttempt; {
		case err == nil:
			successes++
		case errors.Is(err, ErrAlreadyExists):
			duplicates++
		default:
			t.Fatalf("unexpected create error: %v", err)
		}
	}
	if successes != 1 || duplicates != 1 {
		t.Fatalf("successes=%d duplicates=%d", successes, duplicates)
	}
}
func TestMembershipWorktreeJobsAndToolchainResolution(t *testing.T) {
	repository := testRepository(t)
	ctx := context.Background()
	person := domain.Person{ID: uuid.NewString(), Username: "alice", DisplayName: "Alice", Email: "alice@example.test", Role: domain.RoleDeveloper}
	project := domain.Project{ID: uuid.NewString(), Slug: "demo", Name: "Demo", UnixUser: "soda-p-demo", Profile: domain.ToolchainGo, Source: domain.GitProjectSource{RemoteURL: "git@example.com:team/demo.git"}}
	if err := repository.CreatePerson(ctx, person); err != nil {
		t.Fatal(err)
	}
	if err := repository.CreateProjectWithMemberships(ctx, project, nil); err != nil {
		t.Fatal(err)
	}
	tree := domain.Worktree{ID: uuid.NewString(), ProjectID: project.ID, PersonID: person.ID, Name: "default", Branch: "people/alice", Path: "/projects/demo/alice"}
	membership := domain.Membership{ProjectID: project.ID, PersonID: person.ID}
	if err := repository.AddMembershipAndWorktree(ctx, membership, tree); err != nil {
		t.Fatal(err)
	}
	if err := repository.AddMembershipAndWorktree(ctx, membership, tree); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("duplicate membership = %v", err)
	}
	secondTree := tree
	secondTree.ID, secondTree.Branch, secondTree.Path = uuid.NewString(), "people/alice-task", "/projects/demo/alice-task"
	if err := repository.CreateWorktree(ctx, secondTree); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("second personal workspace = %v", err)
	}
	assertProvisioningJobHistory(t, repository, ctx, project.ID)
	installation := domain.ToolchainInstallation{ID: uuid.NewString(), Profile: domain.ToolchainGo, Version: "go1.25", Path: "/opt/go", Checksum: "abc", State: domain.JobReady}
	resolution, err := repository.SaveInstallation(ctx, project.ID, installation)
	if err != nil {
		t.Fatal(err)
	}
	got, gotResolution, err := repository.ProjectInstallation(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != installation.Version || gotResolution != resolution {
		t.Fatalf("installation = %#v, resolution = %#v", got, gotResolution)
	}
}

func testRepository(t *testing.T) *Store {
	t.Helper()
	repository, err := Open(filepath.Join(t.TempDir(), "soda.db"))
	if err != nil {
		t.Fatal(err)
	}
	return repository
}

func assertProvisioningJobHistory(t *testing.T, repository *Store, ctx context.Context, projectID string) {
	t.Helper()
	job := domain.ProvisioningJob{ID: uuid.NewString(), ProjectID: projectID, State: domain.JobInstalling}
	if err := repository.DB().WithContext(ctx).Create(jobRow(job)).Error; err != nil {
		t.Fatal(err)
	}
	message := "download failed"
	job.State = domain.JobFailed
	job.Error = &message
	if err := repository.UpdateJob(ctx, job); err != nil {
		t.Fatal(err)
	}
	jobs, err := repository.Jobs(ctx, projectID)
	if err != nil || len(jobs) != 1 || jobs[0].State != domain.JobFailed {
		t.Fatalf("jobs = %#v, %v", jobs, err)
	}
}

func TestSaveInstallationReusesProfileVersionAcrossProjects(t *testing.T) {
	repository, err := Open(filepath.Join(t.TempDir(), "soda.db"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	firstProject := domain.Project{ID: uuid.NewString(), Slug: "first", Name: "First", UnixUser: "soda-p-first", Profile: domain.ToolchainGo, Source: domain.EmptyProjectSource{}}
	secondProject := domain.Project{ID: uuid.NewString(), Slug: "second", Name: "Second", UnixUser: "soda-p-second", Profile: domain.ToolchainGo, Source: domain.EmptyProjectSource{}}
	for _, project := range []domain.Project{firstProject, secondProject} {
		if err = repository.CreateProjectWithMemberships(ctx, project, nil); err != nil {
			t.Fatal(err)
		}
	}
	first := domain.ToolchainInstallation{ID: uuid.NewString(), Profile: domain.ToolchainGo, Version: "go1.25.1", Path: "/opt/soda/toolchains/go/go1.25.1", Checksum: "verified", State: domain.JobReady}
	firstResolution, err := repository.SaveInstallation(ctx, firstProject.ID, first)
	if err != nil {
		t.Fatal(err)
	}
	second := first
	second.ID = uuid.NewString()
	secondResolution, err := repository.SaveInstallation(ctx, secondProject.ID, second)
	if err != nil {
		t.Fatal(err)
	}
	if firstResolution.ToolchainInstallationID != first.ID {
		t.Fatalf("first installation ID = %s", firstResolution.ToolchainInstallationID)
	}
	if secondResolution.ToolchainInstallationID != first.ID {
		t.Fatalf("second project did not reuse installation: %#v", secondResolution)
	}
	var count int64
	if err = repository.DB().Model(&ToolchainInstallation{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("installation count = %d", count)
	}
}

func TestBeginProvisioningEnforcesLatestState(t *testing.T) {
	repository, err := Open(filepath.Join(t.TempDir(), "soda.db"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	project := domain.Project{ID: uuid.NewString(), Slug: "demo", Name: "Demo", UnixUser: "soda-p-demo", Profile: domain.ToolchainGo, Source: domain.EmptyProjectSource{}}
	if err = repository.CreateProjectWithMemberships(ctx, project, nil); err != nil {
		t.Fatal(err)
	}
	first := domain.ProvisioningJob{ID: uuid.NewString(), ProjectID: project.ID, State: domain.JobInstalling}
	if err = repository.BeginProvisioning(ctx, first); err != nil {
		t.Fatal(err)
	}
	if err = repository.BeginProvisioning(ctx, domain.ProvisioningJob{ID: uuid.NewString(), ProjectID: project.ID, State: domain.JobInstalling}); !errors.Is(err, ErrFailedPrecondition) {
		t.Fatalf("installing retry = %v", err)
	}
	message := "failed"
	first.State, first.Error = domain.JobFailed, &message
	if err = repository.UpdateJob(ctx, first); err != nil {
		t.Fatal(err)
	}
	second := domain.ProvisioningJob{ID: uuid.NewString(), ProjectID: project.ID, State: domain.JobInstalling}
	if err = repository.BeginProvisioning(ctx, second); err != nil {
		t.Fatalf("failed retry: %v", err)
	}
	second.State = domain.JobReady
	if err = repository.UpdateJob(ctx, second); err != nil {
		t.Fatal(err)
	}
	if err = repository.BeginProvisioning(ctx, domain.ProvisioningJob{ID: uuid.NewString(), ProjectID: project.ID, State: domain.JobInstalling}); !errors.Is(err, ErrFailedPrecondition) {
		t.Fatalf("ready retry = %v", err)
	}
}

func TestFailInterruptedProvisioningAllowsRetryAfterRestart(t *testing.T) {
	repository := testRepository(t)
	ctx := context.Background()
	project := domain.Project{ID: uuid.NewString(), Slug: "demo", Name: "Demo", UnixUser: "soda-p-demo", Profile: domain.ToolchainGo, Source: domain.EmptyProjectSource{}}
	if err := repository.CreateProjectWithMemberships(ctx, project, nil); err != nil {
		t.Fatal(err)
	}
	abandoned := domain.ProvisioningJob{ID: uuid.NewString(), ProjectID: project.ID, State: domain.JobInstalling}
	if err := repository.BeginProvisioning(ctx, abandoned); err != nil {
		t.Fatal(err)
	}
	count, err := repository.FailInterruptedProvisioning(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("reconciled jobs = %d, want 1", count)
	}
	jobs, err := repository.Jobs(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 {
		t.Fatalf("reconciled jobs = %#v", jobs)
	}
	if jobs[0].Error == nil {
		t.Fatalf("reconciled job has no failure message: %#v", jobs[0])
	}
	got := struct {
		state   domain.JobState
		message string
	}{state: jobs[0].State, message: *jobs[0].Error}
	want := struct {
		state   domain.JobState
		message string
	}{state: domain.JobFailed, message: "provisioning interrupted by daemon restart; retry provisioning manually"}
	if got != want {
		t.Fatalf("reconciled job = %#v", jobs[0])
	}
	retry := domain.ProvisioningJob{ID: uuid.NewString(), ProjectID: project.ID, State: domain.JobInstalling}
	if err := repository.BeginProvisioning(ctx, retry); err != nil {
		t.Fatalf("manual retry after restart reconciliation: %v", err)
	}
}
