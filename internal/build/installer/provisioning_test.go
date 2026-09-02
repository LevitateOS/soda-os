package installer

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInstallerEnvironmentUsesProtectedKickstartComposition(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	containerfile := readInstallerFixture(t, root, "packaging/installer/Containerfile")
	for _, expected := range []string{
		"COPY --chmod=0755 packaging/installer/soda-installer-input /usr/libexec/soda/soda-installer-input",
		"COPY --chmod=0755 packaging/installer/soda-installer-finalize /usr/libexec/soda/soda-installer-finalize",
		"test -x /usr/bin/eject",
		"test -x /usr/bin/openssl",
		"test -x /usr/bin/ssh-keygen",
	} {
		require.Contains(t, containerfile, expected)
	}
	for _, obsolete := range []string{
		"org_fedoraproject_soda",
		"org.fedoraproject.Anaconda.Addons.SodaInstaller",
	} {
		require.NotContains(t, containerfile, obsolete)
	}

	profile := readInstallerFixture(t, root, "packaging/installer/branding/sodaos.conf")
	require.Contains(t, profile, "can_copy_input_kickstart = False")
	require.Contains(t, profile, "can_save_output_kickstart = False")
	require.Contains(t, profile, "hidden_spokes = UserSpoke PasswordSpoke")

	for _, architecture := range []string{"aarch64", "x86_64"} {
		config := readInstallerFixture(t, root, "packaging/installer/iso-"+architecture+".yaml")
		require.Contains(t, config, "inst.ks=hd:LABEL=OEMDRV:/ks.cfg")
		require.Contains(t, config, "inst.nosave=all_ks")
	}

	for _, obsolete := range []string{
		"packaging/installer/addons/org_fedoraproject_soda",
		"packaging/installer/addons/org.fedoraproject.Anaconda.Addons.SodaInstaller.conf",
		"packaging/installer/addons/org.fedoraproject.Anaconda.Addons.SodaInstaller.service",
	} {
		_, err := os.Lstat(filepath.Join(root, obsolete))
		require.ErrorIs(t, err, os.ErrNotExist)
	}
}

func TestInstallerOnlyExecutablesAreFixedAndParse(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	inputPath := filepath.Join(root, "packaging", "installer", "soda-installer-input")
	finalizerPath := filepath.Join(root, "packaging", "installer", "soda-installer-finalize")
	for _, path := range []string{inputPath, finalizerPath} {
		info, err := os.Stat(path)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o755), info.Mode().Perm())
		check := exec.Command("python3", "-c", "import ast,pathlib,sys; ast.parse(pathlib.Path(sys.argv[1]).read_text())", path)
		output, err := check.CombinedOutput()
		require.NoErrorf(t, err, "invalid installer Python in %s:\n%s", path, output)
	}

	input := readInstallerFixture(t, root, "packaging/installer/soda-installer-input")
	for _, expected := range []string{
		`DEVICE_LINK = Path("/dev/disk/by-label/OEMDRV")`,
		`MEDIA_INPUT_DIR = MOUNT_POINT / "soda"`,
		`RUNTIME_DIR = Path("/run/soda-installer")`,
		`SSH_KEY_FILE = "administrator-authorized-key"`,
		`"ro,nodev,nosuid,noexec"`,
		`["/usr/bin/openssl", "passwd", "-6", "-stdin"]`,
		`user --name={username} --groups=wheel`,
		`sshkey --username={username}`,
		`["/usr/bin/eject", str(device)]`,
	} {
		require.Contains(t, input, expected)
	}

	finalizer := readInstallerFixture(t, root, "packaging/installer/soda-installer-finalize")
	for _, expected := range []string{
		`SYSROOT = Path("/mnt/sysroot")`,
		`RUNTIME_DIR = Path("/run/soda-installer")`,
		`SSH_KEY_FILE = "administrator-authorized-key"`,
		`Path("/proc/self/mountinfo")`,
		`mountpoint = str(target_var)`,
		`def _bind_target_mount(source, relative_target):`,
		`mounted_dev = _bind_target_mount("/dev", "dev")`,
		`mounted_proc = _bind_target_mount("/proc", "proc")`,
		`["/usr/libexec/soda/forgejo-init"]`,
		`"POST",`,
		`"/user/sign_up"`,
		`os.replace(temporary, destination)`,
	} {
		require.Contains(t, finalizer, expected)
	}
	for _, forbidden := range []string{
		"pyanaconda", "dasbus", "Gtk", "ostree/deploy", "setfiles", "semanage",
		`["/usr/bin/mount", "--bind", str(persistent_var)`,
		"installer-admin.json", "if (target / probe).exists()",
	} {
		require.NotContains(t, input+finalizer, forbidden)
	}
	require.NotContains(t, finalizer, `"/usr/sbin/restorecon"`)
	require.NotContains(t, finalizer, `"/usr/sbin/matchpathcon"`)
	require.NotContains(t, finalizer, "--password")
}

func TestTailscaleEnrollmentRemainsOneAttemptAndAlwaysCleansSecrets(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	unit := readInstallerFixture(t, root, "packaging/rpm/runtime/sources/systemd/soda-tailscale-enroll.service")
	for _, expected := range []string{
		"Type=oneshot",
		"TimeoutStartSec=2min",
		"ExecStart=/usr/bin/tailscale up --auth-key=file:/var/lib/soda-install/tailscale-auth-key",
		"ExecStopPost=-/usr/bin/unlink /var/lib/soda-install/tailscale-auth-key",
		"ExecStopPost=-/usr/bin/rmdir /var/lib/soda-install",
		"ExecStopPost=-/usr/bin/systemctl --no-reload --quiet disable soda-tailscale-enroll.service",
	} {
		require.Contains(t, unit, expected)
	}
	for _, forbidden := range []string{"ConditionPathExists", "Restart=", "Environment="} {
		require.NotContains(t, unit, forbidden)
	}
}

func TestAcceptanceUsesTheProtectedAnswerMediaBoundary(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	runner := readInstallerFixture(t, root, "tests/acceptance/unattended.sh")
	for _, expected := range []string{
		"installer-input",
		`--tailscale-auth-key-file "$tailscale_auth_key_file"`,
		`--password-file "$password_file"`,
		`--output "$kickstart_iso"`,
	} {
		require.Contains(t, runner, expected)
	}
	for _, obsolete := range []string{
		"%addon org_fedoraproject_soda",
		"tailscale_auth_key=$tailscale_auth_key",
		`user --name=$admin`,
		"start_x86_unattended_boot_selector",
	} {
		require.NotContains(t, runner, obsolete)
	}

	bootRunner := readInstallerFixture(t, root, "tests/acceptance/bootc.sh")
	require.Contains(t, bootRunner, "SODA_ACCEPTANCE_KICKSTART_ISO is required for launch install")
	require.Contains(t, bootRunner, "start_installer_input_ejector")
	require.Contains(t, bootRunner, `"execute":"blockdev-remove-medium"`)
	require.NotContains(t, bootRunner, "start_x86_unattended_boot_selector")
	require.NotContains(t, bootRunner, `"execute":"send-key"`)
}

func readInstallerFixture(t *testing.T, root, relative string) string {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	require.NoError(t, err)
	return strings.ReplaceAll(string(contents), "\r\n", "\n")
}
