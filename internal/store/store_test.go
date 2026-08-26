package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/LevitateOS/soda-os/internal/domain"
	"github.com/google/uuid"
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
