package store

import (
	"context"
	"fmt"
	"testing"

	"github.com/LevitateOS/soda-os/internal/domain"
	"github.com/google/uuid"
)

func TestGitIdentityIsPublicOneToOnePersonState(t *testing.T) {
	repository := testRepository(t)
	ctx := context.Background()
	person := domain.Person{ID: uuid.NewString(), Username: "alice", DisplayName: "Alice", Email: "alice@example.test", Role: domain.RoleDeveloper}
	identity := domain.GitIdentity{PersonID: person.ID, PublicKey: "ssh-ed25519 AAAA alice", Fingerprint: "SHA256:alice"}
	if err := repository.CreatePersonWithGitIdentity(ctx, person, identity); err != nil {
		t.Fatal(err)
	}
	got, err := repository.GitIdentity(ctx, person.ID)
	if err != nil || got != identity {
		t.Fatalf("Git identity = %#v, %v", got, err)
	}
	rows, err := repository.DB().Raw("PRAGMA table_info(git_identities)").Rows()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rows.Close() })
	var columns []string
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue any
		if err = rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatal(err)
		}
		columns = append(columns, name)
	}
	if fmt.Sprint(columns) != "[person_id public_key fingerprint]" {
		t.Fatalf("Git identity columns = %v", columns)
	}
}

func TestProjectBootstrapConstraintMatchesRepositorySource(t *testing.T) {
	repository := testRepository(t)
	ctx := context.Background()
	person := domain.Person{ID: uuid.NewString(), Username: "alice", DisplayName: "Alice", Email: "alice@example.test", Role: domain.RoleDeveloper}
	if err := repository.CreatePerson(ctx, person); err != nil {
		t.Fatal(err)
	}
	external := domain.Project{ID: uuid.NewString(), Slug: "external", Name: "External", UnixUser: "soda-p-external", Profile: domain.ToolchainGo, Source: domain.GitProjectSource{RemoteURL: "git@example.test:demo.git"}}
	if err := repository.CreateProjectWithMemberships(ctx, external, []string{person.ID}); err == nil {
		t.Fatal("external project without bootstrap person was accepted")
	}
	builtIn := domain.Project{ID: uuid.NewString(), Slug: "built-in", Name: "Built in", UnixUser: "soda-p-built-in", Profile: domain.ToolchainGo, Source: domain.EmptyProjectSource{}, BootstrapPersonID: person.ID}
	if err := repository.CreateProjectWithMemberships(ctx, builtIn, []string{person.ID}); err == nil {
		t.Fatal("built-in project with bootstrap person was accepted")
	}
}
