package workspace

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/stretchr/testify/require"
)

// Execute the exact inspection Git arguments as the test UID in temporary homes,
// not through runuser and never against real workspace accounts.
func runInspectionGit(ctx context.Context, command linuxhost.Command) (linuxhost.CommandResult, error) {
	index := slices.Index(command.Args, "/usr/bin/git")
	if command.Name != "/usr/sbin/runuser" || index < 5 {
		return linuxhost.CommandResult{}, errors.New("unexpected inspection command")
	}
	process := exec.CommandContext(ctx, "/usr/bin/git", command.Args[index+1:]...)
	process.Env = command.Args[5:index]
	var stdout, stderr bytes.Buffer
	process.Stdout, process.Stderr = &stdout, &stderr
	err := process.Run()
	result := linuxhost.CommandResult{Stdout: stdout.String(), Stderr: stderr.String()}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		result.ExitCode = exit.ExitCode()
		return result, nil
	}
	return result, err
}

func fixtureGit(t *testing.T, directory string, args ...string) string {
	t.Helper()
	command := exec.Command("/usr/bin/git", append([]string{"-C", directory, "-c", "user.name=Inspection test", "-c", "user.email=inspection@example.test", "-c", "commit.gpgSign=false"}, args...)...)
	command.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + t.TempDir(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null"}
	output, err := command.CombinedOutput()
	require.NoError(t, err, string(output))
	return string(output)
}

func initInspectionGit(t *testing.T, directory string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(directory, 0o700))
	fixtureGit(t, directory, "init", "--quiet")
}

func TestCheckoutInspectionUsesLocalGitWithoutMutation(t *testing.T) {
	root := t.TempDir()
	account := repositoryAccount(root)
	path := filepath.Join(account.Home, "Projects", "site")
	initInspectionGit(t, path)
	var calls []linuxhost.Command
	repository := NewRepository(&linuxhost.Native{HomeRoot: root}, commandRunnerFunc(func(ctx context.Context, command linuxhost.Command) (linuxhost.CommandResult, error) {
		calls = append(calls, command)
		return runInspectionGit(ctx, command)
	}))

	ready, err := repository.CloneExists(context.Background(), account, projectEntry("site"))
	require.NoError(t, err)
	require.True(t, ready, "an empty repository with an unborn branch is usable")
	require.NoError(t, os.WriteFile(filepath.Join(path, "file"), []byte("committed"), 0o600))
	fixtureGit(t, path, "add", "file")
	fixtureGit(t, path, "commit", "--quiet", "-m", "initial")
	index, err := os.ReadFile(filepath.Join(path, ".git", "index"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(path, "file"), []byte("uncommitted work"), 0o600))
	marker := filepath.Join(root, "unexpected-hook")
	fixtureGit(t, path, "config", "core.fsmonitor", "touch "+marker)

	ready, err = repository.CloneExists(context.Background(), account, projectEntry("site"))
	require.NoError(t, err)
	require.True(t, ready, "dirty work is not a setup failure")
	contents, err := os.ReadFile(filepath.Join(path, "file"))
	require.NoError(t, err)
	require.Equal(t, "uncommitted work", string(contents))
	after, err := os.ReadFile(filepath.Join(path, ".git", "index"))
	require.NoError(t, err)
	require.Equal(t, index, after)
	require.NoFileExists(t, marker)
	for _, call := range calls {
		require.Contains(t, call.Args, "GIT_NO_LAZY_FETCH=1")
		require.Contains(t, call.Args, "GIT_OPTIONAL_LOCKS=0")
		require.Contains(t, call.Args, "GIT_CONFIG_GLOBAL=/dev/null")
		require.Equal(t, account.Username, call.Args[1])
	}
}

func TestCheckoutInspectionRejectsFakeAndBrokenGitMetadata(t *testing.T) {
	root := t.TempDir()
	account := repositoryAccount(root)
	path := filepath.Join(account.Home, "Projects", "site")
	require.NoError(t, os.MkdirAll(filepath.Join(path, ".git"), 0o700))
	repository := NewRepository(&linuxhost.Native{HomeRoot: root}, commandRunnerFunc(runInspectionGit))
	ready, err := repository.CloneExists(context.Background(), account, projectEntry("site"))
	require.ErrorContains(t, err, "not a usable Git working tree")
	require.False(t, ready)
	fixtureGit(t, path, "init", "--quiet")
	require.NoError(t, os.WriteFile(filepath.Join(path, ".git", "HEAD"), []byte("ref: refs/heads/broken\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(path, ".git", "refs", "heads", "broken"), []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n"), 0o600))
	ready, err = repository.CloneExists(context.Background(), account, projectEntry("site"))
	require.ErrorContains(t, err, "unreadable commit")
	require.False(t, ready)
}

func TestCheckoutInspectionDoesNotMistakeAnAncestorForTheCheckout(t *testing.T) {
	root := t.TempDir()
	account := repositoryAccount(root)
	path := filepath.Join(account.Home, "Projects", "site")
	initInspectionGit(t, path)
	fixtureGit(t, path, "config", "core.worktree", account.Home)
	repository := NewRepository(&linuxhost.Native{HomeRoot: root}, commandRunnerFunc(runInspectionGit))
	ready, err := repository.CloneExists(context.Background(), account, projectEntry("site"))
	require.Error(t, err)
	require.False(t, ready)
}
