package daemon

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/LevitateOS/soda-os/internal/builtingit"
	"github.com/LevitateOS/soda-os/internal/domain"
	sodav2 "github.com/LevitateOS/soda-os/internal/gen/soda/v2"
	"github.com/LevitateOS/soda-os/internal/store"
	"github.com/stretchr/testify/require"
)

type fakeBuiltInGit struct {
	people    []domain.Person
	kinds     []builtingit.PersonKind
	projects  []domain.Project
	members   [][]domain.Person
	deleted   []int64
	deleteErr error
}

func (f *fakeBuiltInGit) EnsurePerson(_ context.Context, person domain.Person, kind builtingit.PersonKind) (builtingit.User, error) {
	f.people = append(f.people, person)
	f.kinds = append(f.kinds, kind)
	return builtingit.User{ID: int64(len(f.people))}, nil
}

func (*fakeBuiltInGit) EnsureKey(context.Context, domain.Person, domain.SSHDeviceKey) (builtingit.Key, error) {
	return builtingit.Key{ID: 20}, nil
}

func (f *fakeBuiltInGit) DeleteKey(_ context.Context, _ string, keyID int64) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deleted = append(f.deleted, keyID)
	return nil
}

func (f *fakeBuiltInGit) EnsureRepository(_ context.Context, project domain.Project, members []domain.Person, _ string) (builtingit.Repository, error) {
	f.projects = append(f.projects, project)
	f.members = append(f.members, append([]domain.Person(nil), members...))
	return builtingit.Repository{ID: 30, DeployKeyID: 40, WebURL: "http://127.0.0.1:30000/soda/demo", SSHURL: "git@localhost:soda/demo.git"}, nil
}

func TestBuiltInGitUsesTheExistingProjectAndMembershipFlow(t *testing.T) {
	repository, err := store.Open(filepath.Join(t.TempDir(), "soda.db"))
	require.NoError(t, err)
	host := &fakeHost{}
	git := &fakeBuiltInGit{}
	service := New(Options{Store: repository, Host: host, Toolchains: fakeInstaller{}, BuiltInGit: git, ProjectsRoot: filepath.Join(t.TempDir(), "projects")})
	defer service.Close()
	personResponse, err := service.CreatePerson(context.Background(), &sodav2.CreatePersonRequest{Username: "alice", DisplayName: "Alice", Email: "alice@example.test", Role: sodav2.Role_ROLE_ADMIN, Password: "simple"})
	require.NoError(t, err)
	require.Len(t, git.people, 1)
	require.Equal(t, builtingit.PersonAdministrator, git.kinds[0])
	_, err = repository.BuiltInGitUser(context.Background(), personResponse.Person.Id)
	require.NoError(t, err)
	project := domain.Project{ID: "project-1", Slug: "demo", Name: "Demo", UnixUser: "soda-p-demo", Profile: domain.ToolchainGo, Source: domain.EmptyProjectSource{}}
	require.NoError(t, repository.CreateProjectWithMemberships(context.Background(), project, []string{personResponse.Person.Id}))
	require.NoError(t, service.ensureBuiltInGitProject(context.Background(), project))
	require.Len(t, git.projects, 1)
	require.Len(t, git.members[0], 1)
	require.Len(t, host.builtInRemotes, 1)
	_, err = repository.BuiltInGitRepository(context.Background(), project.ID)
	require.NoError(t, err)

	external := domain.Project{ID: "project-2", Slug: "external", Name: "External", UnixUser: "soda-p-external", Profile: domain.ToolchainGo, Source: domain.GitProjectSource{RemoteURL: "ssh://git@example.test/external.git"}}
	require.NoError(t, repository.CreateProjectWithMemberships(context.Background(), external, []string{personResponse.Person.Id}))
	require.NoError(t, service.ensureBuiltInGitProject(context.Background(), external))
	require.Len(t, git.projects, 1)
}

func TestBuiltInGitMirrorsSodaRolesAndBootstrapsAnAdministratorFirst(t *testing.T) {
	repository, err := store.Open(filepath.Join(t.TempDir(), "soda.db"))
	require.NoError(t, err)
	developer := domain.Person{ID: "person-developer", Username: "alice", DisplayName: "Alice", Email: "alice@example.test", Role: domain.RoleDeveloper}
	administrator := domain.Person{ID: "person-admin", Username: "zoe", DisplayName: "Zoe", Email: "zoe@example.test", Role: domain.RoleAdmin}
	require.NoError(t, repository.CreatePerson(context.Background(), developer))
	require.NoError(t, repository.CreatePerson(context.Background(), administrator))
	git := &fakeBuiltInGit{}
	service := New(Options{Store: repository, Host: &fakeHost{}, Toolchains: fakeInstaller{}, BuiltInGit: git, ProjectsRoot: filepath.Join(t.TempDir(), "projects")})
	defer service.Close()

	require.NoError(t, service.ReconcileAllBuiltInGit(context.Background()))
	require.Equal(t, []domain.Person{administrator, developer}, git.people)
	require.Equal(t, []builtingit.PersonKind{builtingit.PersonAdministrator, builtingit.PersonMember}, git.kinds)
}

func TestRevokeSSHDeviceKeyRemovesBuiltInGitMappingAndRemoteKey(t *testing.T) {
	repository, err := store.Open(filepath.Join(t.TempDir(), "soda.db"))
	require.NoError(t, err)
	git := &fakeBuiltInGit{}
	service := New(Options{Store: repository, Host: &fakeHost{}, Toolchains: fakeInstaller{}, BuiltInGit: git, ProjectsRoot: filepath.Join(t.TempDir(), "projects")})
	defer service.Close()
	person, err := service.CreatePerson(context.Background(), &sodav2.CreatePersonRequest{Username: "alice", DisplayName: "Alice", Email: "alice@example.test", Role: sodav2.Role_ROLE_ADMIN, Password: "simple"})
	require.NoError(t, err)
	created, err := service.CreateSshDeviceKey(context.Background(), &sodav2.CreateSshDeviceKeyRequest{PersonId: person.Person.Id, Label: "Laptop", PublicKey: "ssh-ed25519 AAAA alice", IdentityFileHint: "~/.ssh/id_ed25519"})
	require.NoError(t, err)

	_, err = service.RevokeSshDeviceKey(context.Background(), &sodav2.RevokeSshDeviceKeyRequest{PersonId: person.Person.Id, KeyId: created.Key.Id})
	require.NoError(t, err)
	_, err = repository.BuiltInGitKey(context.Background(), created.Key.Id)
	require.ErrorIs(t, err, store.ErrNotFound)
	require.Equal(t, []int64{20}, git.deleted)
}

func TestRevokeSSHDeviceKeyRestoresSodaStateWhenBuiltInGitFails(t *testing.T) {
	repository, err := store.Open(filepath.Join(t.TempDir(), "soda.db"))
	require.NoError(t, err)
	git := &fakeBuiltInGit{deleteErr: errors.New("delete failed")}
	service := New(Options{Store: repository, Host: &fakeHost{}, Toolchains: fakeInstaller{}, BuiltInGit: git, ProjectsRoot: filepath.Join(t.TempDir(), "projects")})
	defer service.Close()
	person, err := service.CreatePerson(context.Background(), &sodav2.CreatePersonRequest{Username: "alice", DisplayName: "Alice", Email: "alice@example.test", Role: sodav2.Role_ROLE_ADMIN, Password: "simple"})
	require.NoError(t, err)
	created, err := service.CreateSshDeviceKey(context.Background(), &sodav2.CreateSshDeviceKeyRequest{PersonId: person.Person.Id, Label: "Laptop", PublicKey: "ssh-ed25519 AAAA alice", IdentityFileHint: "~/.ssh/id_ed25519"})
	require.NoError(t, err)

	_, err = service.RevokeSshDeviceKey(context.Background(), &sodav2.RevokeSshDeviceKeyRequest{PersonId: person.Person.Id, KeyId: created.Key.Id})
	require.Error(t, err)
	keys, loadErr := repository.SSHDeviceKeys(context.Background(), person.Person.Id)
	require.NoError(t, loadErr)
	require.Len(t, keys, 1)
	_, loadErr = repository.BuiltInGitKey(context.Background(), created.Key.Id)
	require.NoError(t, loadErr)
}
