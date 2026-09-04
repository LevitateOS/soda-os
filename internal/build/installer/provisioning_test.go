package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInstallerEnvironmentUsesOneStockInteractivePath(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	containerfile := readInstallerFixture(t, root, "packaging/installer/Containerfile")
	for _, obsolete := range []string{
		"soda-installer-input",
		"soda-installer-finalize",
		"org_fedoraproject_soda",
		"org.fedoraproject.Anaconda.Addons.SodaInstaller",
	} {
		require.NotContains(t, containerfile, obsolete)
	}

	profile := readInstallerFixture(t, root, "packaging/installer/branding/sodaos.conf")
	require.Contains(t, profile, "can_copy_input_kickstart = False")
	require.Contains(t, profile, "can_save_output_kickstart = False")
	require.Contains(t, profile, "hidden_spokes = UserSpoke PasswordSpoke")
	require.Contains(t, containerfile, `install_items+=" /usr/share/anaconda/interactive-defaults.ks "`)

	for _, architecture := range []string{"aarch64", "x86_64"} {
		config := readInstallerFixture(t, root, "packaging/installer/iso-"+architecture+".yaml")
		require.NotContains(t, config, "OEMDRV")
		require.NotContains(t, config, "inst.ks=hd:")
		require.NotContains(t, config, "inst.ks=cdrom:")
		require.Contains(t, config, "inst.ks=file:/usr/share/anaconda/interactive-defaults.ks")
		require.Contains(t, config, "inst.graphical")
	}
}

func TestInstallerEnvironmentCorrectsAnacondaBootcSELinuxFSMount(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	containerfile := readInstallerFixture(t, root, "packaging/installer/Containerfile")
	patcher := readInstallerFixture(t, root, "packaging/installer/patch-anaconda-bootc-selinuxfs.py")

	require.Contains(t, containerfile, "RUN --mount=type=bind,source=packaging/installer/patch-anaconda-bootc-selinuxfs.py")
	require.Contains(t, containerfile, "/usr/bin/python3 /run/patch-anaconda-bootc-selinuxfs.py")
	require.Contains(t, patcher, `ANACONDA_CORE_NEVR = "anaconda-core-0:44.30-2.fc44"`)
	require.Contains(t, patcher, `SOURCE_SHA256 = "614ac3f3061d959144e0a2e80919012c7254d44b1fab04daea35b2bef52f3f86"`)
	require.Contains(t, patcher, `PATCHED_SHA256 = "de1400f91d39bcdba5f34d17b4173ef779c9d890e3ac404565d0c781026163de"`)
	require.Contains(t, patcher, `OLD = b'        for path in ("/proc", "/sys"):\n'`)
	require.Contains(t, patcher, `NEW = b'        for path in ("/proc", "/sys", "/sys/fs/selinux"):\n'`)
}

func TestSupersededProvisioningPathsAreAbsent(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	for _, path := range []string{
		"packaging/installer/soda-installer-input",
		"packaging/installer/soda-installer-finalize",
		"packaging/rpm/runtime/sources/soda-cloud-finalize",
		"packaging/rpm/runtime/sources/cloud/99-soda-datasources.cfg",
		"packaging/rpm/runtime/sources/systemd/soda-tailscale-enroll.service",
	} {
		_, err := os.Lstat(filepath.Join(root, path))
		require.ErrorIs(t, err, os.ErrNotExist)
	}
	command := readInstallerFixture(t, root, "cmd/soda-image/main.go")
	for _, obsolete := range []string{"installer-input", "cloud-input", "NoCloud", "ConfigDrive"} {
		require.NotContains(t, command, obsolete)
	}
}

func TestAcceptanceUsesOneGoWorkflow(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	entry := readInstallerFixture(t, root, "cmd/soda-acceptance/main.go")
	for _, expected := range []string{"soda-acceptance", "runCommand()", "recordCommand()", "candidate-iso", "candidate-qcow2", "fallback-oci"} {
		require.Contains(t, entry, expected)
	}
	for _, old := range []string{"tests/acceptance/unattended.sh", "tests/acceptance/internal/bootc.sh"} {
		_, err := os.Lstat(filepath.Join(root, old))
		require.ErrorIs(t, err, os.ErrNotExist)
	}
}

func readInstallerFixture(t *testing.T, root, relative string) string {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	require.NoError(t, err)
	return strings.ReplaceAll(string(contents), "\r\n", "\n")
}
