package host

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/LevitateOS/soda-os/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestImportPersonAddsGitIdentityToExistingLinuxAccount(t *testing.T) {
	runner := &recordingRunner{}
	system := New(t.TempDir())
	system.PeopleRoot = t.TempDir()
	system.Runner = runner
	identity, cleanup, err := system.ImportPerson(context.Background(), domain.Person{ID: "person-1", Username: "alice", Role: domain.RoleDeveloper})
	require.NoError(t, err)
	require.Equal(t, "person-1", identity.PersonID)
	require.NotEmpty(t, identity.PublicKey)
	require.NotEmpty(t, identity.Fingerprint)
	keyPath := system.gitPrivateKeyPath("alice")
	info, err := os.Stat(keyPath)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	require.True(t, hasCall(runner.calls, "chown", "--recursive", "alice", filepath.Dir(keyPath)), "ownership calls = %#v", runner.calls)
	require.True(t, hasCall(runner.calls, "restorecon", "-R", filepath.Dir(keyPath)), "SELinux calls = %#v", runner.calls)
	require.NoError(t, cleanup(context.Background()))
	_, err = os.Stat(keyPath)
	require.ErrorIs(t, err, os.ErrNotExist)
	require.True(t, hasCall(runner.calls, "getent", "passwd", "alice"), "import calls = %#v", runner.calls)
	require.True(t, hasCall(runner.calls, "usermod", "--append", "--groups", "soda-people", "alice"), "import calls = %#v", runner.calls)
}

func TestGitIdentityFailureDoesNotExposePrivateKeyPath(t *testing.T) {
	runner := &recordingRunner{failName: "ssh-keygen"}
	system := New(t.TempDir())
	system.PeopleRoot = t.TempDir()
	system.Runner = runner
	_, _, err := system.ImportPerson(context.Background(), domain.Person{ID: "person-1", Username: "alice"})
	require.Error(t, err)
	require.NotContains(t, err.Error(), system.PeopleRoot)
	require.True(t, hasCall(runner.calls, "gpasswd", "--delete", "alice", "soda-people"))
}

func TestExternalCloneUsesBootstrapPersonsProtectedGitKey(t *testing.T) {
	runner := &recordingRunner{}
	system := New(t.TempDir())
	system.PeopleRoot = t.TempDir()
	system.Runner = runner
	project := domain.Project{Slug: "demo", UnixUser: "soda-p-demo", Source: domain.GitProjectSource{RemoteURL: "git@example.test:team/demo.git"}}
	bootstrap := domain.Person{ID: "person-1", Username: "alice"}
	require.NoError(t, system.EnsureRepository(context.Background(), project, bootstrap))
	for _, call := range runner.calls {
		if call.name == "git" && len(call.args) > 0 && call.args[0] == "clone" {
			command := call.environment["GIT_SSH_COMMAND"]
			require.Contains(t, command, "-i "+system.gitPrivateKeyPath("alice"))
			require.Contains(t, command, "IdentitiesOnly=yes")
			return
		}
	}
	t.Fatalf("Git clone was not invoked: %#v", runner.calls)
}

func TestExternalCloneFailureDoesNotExposePrivateKeyPath(t *testing.T) {
	runner := &recordingRunner{failName: "git"}
	system := New(t.TempDir())
	system.PeopleRoot = t.TempDir()
	system.Runner = runner
	project := domain.Project{Slug: "demo", UnixUser: "soda-p-demo", Source: domain.GitProjectSource{RemoteURL: "git@example.test:team/demo.git"}}
	err := system.EnsureRepository(context.Background(), project, domain.Person{ID: "person-1", Username: "alice"})
	require.Error(t, err)
	require.NotContains(t, err.Error(), system.PeopleRoot)
	require.Contains(t, err.Error(), "repository access")
}
