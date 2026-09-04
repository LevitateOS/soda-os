package projects

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNativeMiseUsesPrivateAndProjectSharedUpstreamScopes(t *testing.T) {
	runner := &exactCommandRunner{}
	root := t.TempDir()
	platform := &NativePlatform{Runner: runner, MiseRoot: filepath.Join(root, "mise")}
	workspace := Account{Username: "soda-w-example", Home: "/home/soda-w-example"}

	require.NoError(t, platform.InstallMiseTools(context.Background(), workspace, "site", []string{"node@22"}, []string{"go@1.25"}))
	require.Len(t, runner.calls, 5)
	require.Equal(t, "/usr/bin/install", runner.calls[0].Name)
	require.Contains(t, runner.calls[0].Args, filepath.Join(root, "mise", "site"))

	settings := strings.Join(runner.calls[1].Args, " ")
	require.Contains(t, settings, "/usr/bin/env -i")
	require.Contains(t, settings, "mise settings add shared_install_dirs "+filepath.Join(root, "mise", "site", "installs"))
	personal := strings.Join(runner.calls[2].Args, " ")
	require.Contains(t, personal, "MISE_SHARED_INSTALL_DIRS=")
	require.Contains(t, personal, "mise use --env local node@22")
	require.Equal(t, "/home/soda-w-example/Projects/site", runner.calls[2].Directory)
	sharedInstall := strings.Join(runner.calls[3].Args, " ")
	require.NotContains(t, sharedInstall, "MISE_CACHE_DIR=")
	require.Contains(t, sharedInstall, "mise install --shared "+filepath.Join(root, "mise", "site", "installs")+" go@1.25")
	require.Contains(t, strings.Join(runner.calls[4].Args, " "), "mise use go@1.25")
}

func TestNativeMiseReportsCompletedAndRemainingSelections(t *testing.T) {
	runner := &exactCommandRunner{results: []CommandResult{{}, {}, {}, {ExitCode: 1, Stderr: "native backend failed"}}}
	platform := &NativePlatform{Runner: runner, MiseRoot: filepath.Join(t.TempDir(), "mise")}
	workspace := Account{Username: "soda-w-example", Home: "/home/soda-w-example"}

	err := platform.InstallMiseTools(context.Background(), workspace, "site", []string{"node@22"}, []string{"go@1.25", "python@3.13"})
	require.ErrorContains(t, err, "completed project selections none; project selections go@1.25, python@3.13 remain")
	require.ErrorContains(t, err, "native backend failed")
}

func TestNativeMiseProjectRemovalIsScopedToValidatedProject(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "site")
	require.NoError(t, os.MkdirAll(filepath.Join(project, "installs"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(project, "installs", "artifact"), []byte("mise-owned"), 0o600))
	platform := &NativePlatform{MiseRoot: root}

	require.NoError(t, platform.RemoveMiseProject("site"))
	_, err := os.Stat(project)
	require.ErrorIs(t, err, os.ErrNotExist)
	require.Error(t, platform.RemoveMiseProject("../outside"))
}
