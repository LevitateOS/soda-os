package workspace

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

type repositoryRunner struct {
	calls         []linuxhost.Command
	key           string
	cloneFailures int
}

func (runner *repositoryRunner) Run(_ context.Context, command linuxhost.Command) (linuxhost.CommandResult, error) {
	runner.calls = append(runner.calls, command)
	arguments := strings.Join(command.Args, " ")
	if strings.Contains(arguments, "/usr/bin/rm --recursive --force") {
		return linuxhost.CommandResult{}, os.RemoveAll(command.Args[len(command.Args)-1])
	}
	if strings.Contains(arguments, "ssh-keygen -y") && len(runner.calls) == 1 {
		return linuxhost.CommandResult{ExitCode: 1, Stderr: "key does not exist"}, nil
	}
	if strings.Contains(arguments, "/usr/bin/git clone") {
		target := command.Args[len(command.Args)-1]
		if runner.cloneFailures > 0 {
			runner.cloneFailures--
			return linuxhost.CommandResult{ExitCode: 1, Stderr: "native clone failed"}, os.WriteFile(filepath.Join(target, "partial"), []byte("partial"), 0o600)
		}
		if err := os.MkdirAll(filepath.Join(target, ".git"), 0o700); err != nil {
			return linuxhost.CommandResult{}, err
		}
	}
	if strings.Contains(arguments, "ssh-keygen -y") {
		return linuxhost.CommandResult{Stdout: runner.key}, nil
	}
	return linuxhost.CommandResult{}, nil
}

func TestRepositoryPublishRetryRemovesOnlyItsPreviousTemporaryClone(t *testing.T) {
	root := t.TempDir()
	account := repositoryAccount(root)
	require.NoError(t, os.MkdirAll(account.Home, 0o700))
	runner := &repositoryRunner{cloneFailures: 1}
	repository := NewRepository(&linuxhost.Native{Runner: runner, HomeRoot: root}, runner)
	entry := projectEntry("site")

	err := repository.Publish(context.Background(), account, entry)
	require.ErrorContains(t, err, "native clone failed")
	require.FileExists(t, filepath.Join(account.Home, "Projects", ".soda-site.tmp", "partial"))
	require.NoError(t, repository.Publish(context.Background(), account, entry))
	require.NoFileExists(t, filepath.Join(account.Home, "Projects", ".soda-site.tmp", "partial"))
	require.DirExists(t, filepath.Join(account.Home, "Projects", "site", ".git"))
}

func TestRepositoryPrivateKeyAndNativeCloneStayInWorkspace(t *testing.T) {
	root := t.TempDir()
	account := repositoryAccount(root)
	require.NoError(t, os.MkdirAll(account.Home, 0o700))
	runner := &repositoryRunner{key: strings.TrimSpace(string(testAuthorizedKey(t)))}
	repository := NewRepository(&linuxhost.Native{Runner: runner, HomeRoot: root}, runner)
	entry := projectEntry("site")

	publicKey, err := repository.GenerateOutboundKey(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, runner.key, publicKey)
	require.NoError(t, repository.Publish(context.Background(), account, entry))

	require.Len(t, runner.calls, 6)
	generated := strings.Join(runner.calls[1].Args, " ")
	require.Contains(t, generated, "--user "+account.Username+" -- /usr/bin/ssh-keygen")
	require.Contains(t, generated, "-f "+outboundKeyPath(account))
	cleanup := strings.Join(runner.calls[3].Args, " ")
	require.Contains(t, cleanup, "--user "+account.Username+" -- /usr/bin/rm --recursive --force -- ")
	clone := strings.Join(runner.calls[4].Args, " ")
	require.Contains(t, clone, "GIT_SSH_COMMAND=/usr/bin/ssh -i "+outboundKeyPath(account)+" -o IdentitiesOnly=yes -o BatchMode=yes -o StrictHostKeyChecking=accept-new")
	require.Contains(t, clone, "/usr/bin/git clone -- "+entry.CanonicalURL)
	require.NotContains(t, clone, "authorized_keys")
	require.DirExists(t, filepath.Join(account.Home, "Projects", "site", ".git"))
}

func TestRepositoryUsesManagedSymlinkHomeRoot(t *testing.T) {
	root := t.TempDir()
	realHomeRoot := filepath.Join(root, "var", "home")
	require.NoError(t, os.MkdirAll(realHomeRoot, 0o755))
	homeRoot := filepath.Join(root, "home")
	require.NoError(t, os.Symlink(filepath.Join("var", "home"), homeRoot))
	account := linuxhost.Account{
		Username: "soda-w-example",
		UID:      os.Getuid(),
		GID:      os.Getgid(),
		Home:     filepath.Join(homeRoot, "soda-w-example"),
	}
	realHome := filepath.Join(realHomeRoot, account.Username)
	require.NoError(t, os.Mkdir(realHome, 0o700))
	host := &linuxhost.Native{HomeRoot: homeRoot}
	repository := NewRepository(host, commandRunnerFunc(func(_ context.Context, command linuxhost.Command) (linuxhost.CommandResult, error) {
		require.Equal(t, "/usr/sbin/restorecon", command.Name)
		return linuxhost.CommandResult{}, nil
	}))
	require.NoError(t, os.MkdirAll(filepath.Join(realHome, "Projects", "site", ".git"), 0o700))

	exists, err := repository.CloneExists(account, projectEntry("site"))
	require.NoError(t, err)
	require.True(t, exists)
	projects, err := repository.openProjectsForPublication(account)
	require.NoError(t, err)
	require.NoError(t, projects.Close())
}

func TestRepositoryCloneEvidenceRejectsSymlinks(t *testing.T) {
	root := t.TempDir()
	account := repositoryAccount(root)
	projects := filepath.Join(account.Home, "Projects")
	checkout := filepath.Join(projects, "site")
	require.NoError(t, os.MkdirAll(filepath.Join(checkout, ".git"), 0o700))
	repository := NewRepository(&linuxhost.Native{HomeRoot: root}, commandRunnerFunc(func(context.Context, linuxhost.Command) (linuxhost.CommandResult, error) {
		return linuxhost.CommandResult{}, nil
	}))
	exists, err := repository.CloneExists(account, projectEntry("site"))
	require.NoError(t, err)
	require.True(t, exists)

	require.NoError(t, os.RemoveAll(checkout))
	target := filepath.Join(root, "other")
	require.NoError(t, os.MkdirAll(filepath.Join(target, ".git"), 0o700))
	require.NoError(t, os.Symlink(target, checkout))
	_, err = repository.CloneExists(account, projectEntry("site"))
	require.Error(t, err)

	require.NoError(t, os.Remove(checkout))
	require.NoError(t, os.Mkdir(checkout, 0o700))
	require.NoError(t, os.Symlink(filepath.Join(target, ".git"), filepath.Join(checkout, ".git")))
	_, err = repository.CloneExists(account, projectEntry("site"))
	require.Error(t, err)
}

func TestRepositoryCloneEvidenceDoesNotDependOnCurrentInboundKeys(t *testing.T) {
	root := t.TempDir()
	account := repositoryAccount(root)
	require.NoError(t, os.MkdirAll(filepath.Join(account.Home, "Projects", "site", ".git"), 0o700))
	repository := NewRepository(&linuxhost.Native{HomeRoot: root}, commandRunnerFunc(func(context.Context, linuxhost.Command) (linuxhost.CommandResult, error) {
		return linuxhost.CommandResult{}, nil
	}))
	exists, err := repository.CloneExists(account, projectEntry("site"))
	require.NoError(t, err)
	require.True(t, exists)

	require.NoError(t, os.MkdirAll(filepath.Join(account.Home, ".ssh"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(account.Home, ".ssh", "authorized_keys"), []byte("user-managed malformed contents\n"), 0o600))
	exists, err = repository.CloneExists(account, projectEntry("site"))
	require.NoError(t, err)
	require.True(t, exists)
}

func repositoryAccount(root string) linuxhost.Account {
	return linuxhost.Account{
		Username:     "soda-w-example",
		UID:          os.Getuid(),
		GID:          os.Getgid(),
		PrimaryGroup: "soda-w-example",
		Home:         filepath.Join(root, "soda-w-example"),
	}
}

func testAuthorizedKey(t *testing.T) []byte {
	t.Helper()
	public, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	key, err := ssh.NewPublicKey(public)
	require.NoError(t, err)
	return ssh.MarshalAuthorizedKey(key)
}
