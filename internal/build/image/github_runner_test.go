package image

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LevitateOS/soda-os/internal/config"
	"github.com/stretchr/testify/require"
)

func TestGitHubRunnerLockBindsExactSiblingArchitectureAssets(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	require.NoError(t, err)
	lock, err := readGitHubRunnerSourceLock(filepath.Join(root, "distro", "locks", "github-runner-source.toml"))
	require.NoError(t, err)
	require.Equal(t, "2.337.0", lock.Version)
	arm, err := lock.asset("aarch64")
	require.NoError(t, err)
	require.Equal(t, "actions-runner-linux-arm64-2.337.0.tar.gz", arm.Archive)
	x86, err := lock.asset("x86_64")
	require.NoError(t, err)
	require.Equal(t, "actions-runner-linux-x64-2.337.0.tar.gz", x86.Archive)
	require.NotEqual(t, arm.SHA256, x86.SHA256)
}

func TestGitHubRunnerLockRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "github-runner-source.toml")
	require.NoError(t, os.WriteFile(path, []byte(`version = "2.337.0"
release_url = "https://github.com/actions/runner/releases/tag/v2.337.0"
unexpected = true
`), 0o600))
	_, err := readGitHubRunnerSourceLock(path)
	require.ErrorContains(t, err, "unknown key")
}

func TestGitHubRunnerSourceStagesOnlyTheSelectedNativeArchive(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	require.NoError(t, err)
	lock, err := readGitHubRunnerSourceLock(filepath.Join(root, "distro", "locks", "github-runner-source.toml"))
	require.NoError(t, err)
	asset, err := lock.asset("aarch64")
	require.NoError(t, err)
	artifactRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(artifactRoot, ".artifacts", "tools"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(artifactRoot, "distro", "locks"), 0o755))
	sourceArchive := filepath.Join(root, ".artifacts", "tools", asset.Archive)
	if _, err = os.Stat(sourceArchive); os.IsNotExist(err) {
		t.Skip("reviewed GitHub runner archive is not fetched")
	}
	require.NoError(t, err)
	require.NoError(t, copyFile(sourceArchive, filepath.Join(artifactRoot, ".artifacts", "tools", asset.Archive)))
	require.NoError(t, copyFile(filepath.Join(root, "distro", "locks", "github-runner-source.toml"), filepath.Join(artifactRoot, "distro", "locks", "github-runner-source.toml")))
	sources := t.TempDir()
	builder := &Builder{Root: artifactRoot, Spec: config.DistroSpec{Platform: config.PlatformSpec{Architecture: config.PlatformArchitecture{Name: "aarch64"}}}}
	require.NoError(t, builder.stageGitHubRunnerSource(sources))
	require.FileExists(t, filepath.Join(sources, "github-actions-runner.tar.gz"))
}
