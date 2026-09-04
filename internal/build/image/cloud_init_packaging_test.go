package image

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCloudInitUsesNativePackagesAndStageActivation(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	for _, architecture := range []string{"aarch64", "x86_64"} {
		lock, err := readPackageLock(filepath.Join(root, "distro", "locks", "runtime-packages-"+architecture+".toml"))
		require.NoError(t, err)
		var names []string
		for _, item := range lock.Package {
			names = append(names, item.Name)
		}
		require.Contains(t, names, "cloud-init")
	}
	container, err := os.ReadFile(filepath.Join(root, "packaging", "bootc", "Containerfile"))
	require.NoError(t, err)
	require.Contains(t, string(container), "systemctl enable cloud-init-main.service cloud-init-local.service cloud-config.service cloud-final.service")
	require.NotContains(t, string(container), "remove avahi cloud-init")
	require.NotContains(t, string(container), "cloud-init.disabled")
	runtime, err := os.ReadFile(filepath.Join(root, "packaging", "rpm", "runtime", "soda-runtime.spec"))
	require.NoError(t, err)
	require.Contains(t, string(runtime), "Requires:       cloud-init,")
	require.Contains(t, string(runtime), ", sudo,")
	_, err = os.Stat(filepath.Join(root, "packaging", "rpm", "runtime", "sources", "systemd", "soda-setup.service"))
	require.ErrorIs(t, err, os.ErrNotExist)
}
