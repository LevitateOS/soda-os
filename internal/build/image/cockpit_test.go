package image

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCockpitStagingCopiesAllBuiltRuntimeAssets(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	require.NoError(t, err)
	sources := t.TempDir()
	require.NoError(t, (&Builder{Root: root}).stageCockpitSources(sources))
	for _, name := range []string{"soda-projects", "soda-runners", "soda-tailscale"} {
		assertCockpitStaged(t, root, sources, name)
	}
}

func assertCockpitStaged(t *testing.T, root, sources, name string) {
	t.Helper()
	original := filepath.Join(root, "cockpit", "dist", name)
	destination := filepath.Join(sources, name+"-cockpit")
	var expected, actual []string
	require.NoError(t, filepath.WalkDir(original, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(original, path)
		require.NoError(t, relErr)
		expected = append(expected, rel)
		contents, readErr := os.ReadFile(path)
		require.NoError(t, readErr)
		staged, readErr := os.ReadFile(filepath.Join(destination, rel))
		require.NoError(t, readErr)
		require.Equal(t, contents, staged, rel)
		return nil
	}))
	require.NoError(t, filepath.WalkDir(destination, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			rel, relErr := filepath.Rel(destination, path)
			require.NoError(t, relErr)
			actual = append(actual, rel)
		}
		return nil
	}))
	require.Equal(t, expected, actual)
}

func TestCockpitStagingRejectsMissingBuild(t *testing.T) {
	err := (&Builder{Root: t.TempDir()}).stageCockpitSources(t.TempDir())
	require.ErrorContains(t, err, "missing built Cockpit package")
}

func TestCockpitBuildInstallsLockedDependenciesBeforeProductionBuild(t *testing.T) {
	runner := &recordingRunner{}
	builder := &Builder{Root: "/workspace/soda", runner: runner}
	require.NoError(t, builder.buildCockpit(context.Background()))
	require.Len(t, runner.Commands, 2)
	require.Equal(t, "/workspace/soda/cockpit", runner.Commands[0].Dir)
	require.Equal(t, "vp", runner.Commands[0].Name)
	require.Equal(t, []string{"install", "--frozen-lockfile"}, runner.Commands[0].Args)
	require.Equal(t, []string{"build"}, runner.Commands[1].Args)
}
