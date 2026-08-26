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

func TestOpenCreatesSchemaVersionOneAndEnforcesConstraints(t *testing.T) {
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
	person := domain.Person{ID: uuid.NewString(), Username: "alice", DisplayName: "Alice", Email: "alice@example.test", Role: domain.RoleDeveloper, SSHPublicKey: "ssh-ed25519 AAAA alice"}
	if err = repository.CreatePerson(ctx, person); err != nil {
		t.Fatal(err)
	}
	duplicate := person
	duplicate.ID = uuid.NewString()
	if err = repository.CreatePerson(ctx, duplicate); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("duplicate error = %v", err)
	}
	project := domain.Project{ID: uuid.NewString(), Slug: "demo", Name: "Demo", UnixUser: "soda-p-demo", Profile: domain.ToolchainGo, Source: domain.EmptyProjectSource{}}
	if err = repository.CreateProject(ctx, project); err != nil {
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
	if err = repository.DB().Exec("PRAGMA user_version = 2").Error; err != nil {
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

func TestPersonFingerprintUniquenessAllowsEmptyBootstrapKeys(t *testing.T) {
	repository, err := Open(filepath.Join(t.TempDir(), "soda.db"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	key := "ssh-ed25519 AAAA"
	first := domain.Person{ID: uuid.NewString(), Username: "alice", DisplayName: "Alice", Email: "alice@example.test", Role: domain.RoleDeveloper, SSHPublicKey: key + " first-comment"}
	second := domain.Person{ID: uuid.NewString(), Username: "bob", DisplayName: "Bob", Email: "bob@example.test", Role: domain.RoleDeveloper, SSHPublicKey: key + " different-comment"}
	if err = repository.CreatePerson(ctx, first); err != nil {
		t.Fatal(err)
	}
	if err = repository.PreflightPerson(ctx, second.Username, second.SSHPublicKey); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("fingerprint preflight = %v", err)
	}
	if err = repository.CreatePerson(ctx, second); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("fingerprint constraint = %v", err)
	}
	for i := 0; i < 2; i++ {
		person := domain.Person{ID: uuid.NewString(), Username: fmt.Sprintf("bootstrap%d", i), DisplayName: "Bootstrap", Email: fmt.Sprintf("bootstrap%d@soda.local", i), Role: domain.RoleAdmin}
		if err = repository.CreatePerson(ctx, person); err != nil {
			t.Fatalf("empty key %d: %v", i, err)
		}
	}
}

func TestPersonFingerprintConstraintIsConcurrent(t *testing.T) {
	repository, err := Open(filepath.Join(t.TempDir(), "soda.db"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	key := "ssh-ed25519 AAAA"
	start := make(chan struct{})
	errorsByAttempt := make(chan error, 2)
	var ready sync.WaitGroup
	ready.Add(2)
	for i, username := range []string{"alice", "bob"} {
		go func(index int, name string) {
			ready.Done()
			<-start
			errorsByAttempt <- repository.CreatePerson(ctx, domain.Person{ID: uuid.NewString(), Username: name, DisplayName: name, Email: name + "@example.test", Role: domain.RoleDeveloper, SSHPublicKey: fmt.Sprintf("%s comment-%d", key, index)})
		}(i, username)
	}
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
	repository, err := Open(filepath.Join(t.TempDir(), "soda.db"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	person := domain.Person{ID: uuid.NewString(), Username: "alice", DisplayName: "Alice", Email: "alice@example.test", Role: domain.RoleDeveloper, SSHPublicKey: "ssh-ed25519 AAAA alice"}
	project := domain.Project{ID: uuid.NewString(), Slug: "demo", Name: "Demo", UnixUser: "soda-p-demo", Profile: domain.ToolchainGo, Source: domain.GitProjectSource{RemoteURL: "git@example.com:team/demo.git"}}
	if err = repository.CreatePerson(ctx, person); err != nil {
		t.Fatal(err)
	}
	if err = repository.CreateProject(ctx, project); err != nil {
		t.Fatal(err)
	}
	tree := domain.Worktree{ID: uuid.NewString(), ProjectID: project.ID, PersonID: person.ID, Name: "default", Branch: "people/alice", Path: "/projects/demo/alice"}
	membership := domain.Membership{ProjectID: project.ID, PersonID: person.ID}
	if err = repository.AddMembershipAndWorktree(ctx, membership, tree); err != nil {
		t.Fatal(err)
	}
	if err = repository.AddMembershipAndWorktree(ctx, membership, tree); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("duplicate membership = %v", err)
	}
	job := domain.ProvisioningJob{ID: uuid.NewString(), ProjectID: project.ID, State: domain.JobInstalling}
	if err = repository.CreateJob(ctx, job); err != nil {
		t.Fatal(err)
	}
	message := "download failed"
	job.State = domain.JobFailed
	job.Error = &message
	if err = repository.UpdateJob(ctx, job); err != nil {
		t.Fatal(err)
	}
	jobs, err := repository.Jobs(ctx, project.ID)
	if err != nil || len(jobs) != 1 || jobs[0].State != domain.JobFailed {
		t.Fatalf("jobs = %#v, %v", jobs, err)
	}
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

func TestSaveInstallationReusesProfileVersionAcrossProjects(t *testing.T) {
	repository, err := Open(filepath.Join(t.TempDir(), "soda.db"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	firstProject := domain.Project{ID: uuid.NewString(), Slug: "first", Name: "First", UnixUser: "soda-p-first", Profile: domain.ToolchainGo, Source: domain.EmptyProjectSource{}}
	secondProject := domain.Project{ID: uuid.NewString(), Slug: "second", Name: "Second", UnixUser: "soda-p-second", Profile: domain.ToolchainGo, Source: domain.EmptyProjectSource{}}
	for _, project := range []domain.Project{firstProject, secondProject} {
		if err = repository.CreateProject(ctx, project); err != nil {
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
	if err = repository.CreateProject(ctx, project); err != nil {
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
