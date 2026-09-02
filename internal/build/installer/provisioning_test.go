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
		"Usage:\n  tests/acceptance/unattended.sh run",
		"installer-input",
		`--tailscale-auth-key-file "$tailscale_key"`,
		`--password-file "$password_file"`,
		`--output "$oemdrv"`,
		`work_dir=$(mktemp -d "${TMPDIR:-/tmp}/soda-acceptance-run.XXXXXX")`,
		`export SODA_ACCEPTANCE_DISK=$disk`,
		`sanitize_evidence`,
	} {
		require.Contains(t, runner, expected)
	}
	for _, obsolete := range []string{
		"%addon org_fedoraproject_soda",
		"tailscale_auth_key=$tailscale_auth_key",
		`user --name=$admin`,
		"start_x86_unattended_boot_selector",
		"runner.env",
		"prepare)",
		`admin_key=$evidence_dir/admin`,
		`password_file=$evidence_dir/admin-password`,
		`stat -c %a "$oemdrv"`,
	} {
		require.NotContains(t, runner, obsolete)
	}
	require.Contains(t, runner, `'.Peer[]? | select(.ID == $id)'`)
	require.Contains(t, runner, `wait_for_exit "$qemu_pid" 120`)
	require.Contains(t, runner, `tailscale_command=/Applications/Tailscale.app/Contents/MacOS/Tailscale`)
	require.Contains(t, runner, `TAILSCALE_BE_CLI=1 "$tailscale_command" "$@"`)
	require.Contains(t, runner, `host_tailscale status --json`)
	require.NotContains(t, runner, "\n\t\ttailscale status --json")

	bootRunner := readInstallerFixture(t, root, "tests/acceptance/internal/bootc.sh")
	require.Contains(t, bootRunner, "SODA_ACCEPTANCE_KICKSTART_ISO is required for launch install")
	require.Contains(t, bootRunner, "start_installer_input_ejector")
	require.Contains(t, bootRunner, "while kill -0 \"$qemu_pid\" 2>/dev/null; do")
	require.Contains(t, bootRunner, `kill -KILL "$qemu_pid"`)
	require.Contains(t, bootRunner, `"execute":"blockdev-remove-medium"`)
	require.Contains(t, bootRunner, `failed_units=$(systemctl --failed --no-legend --plain || true)`)
	require.Contains(t, bootRunner, `if test -n "$failed_units"; then`)
	require.Contains(t, bootRunner, `printf "%s\n" "$failed_units"`)
	require.Contains(t, bootRunner, `systemctl status --no-pager --full -- "$failed_unit"`)
	require.Contains(t, bootRunner, `journalctl --boot --no-pager --unit "$failed_unit" --lines 100`)
	require.Contains(t, bootRunner, `uid=$(id -u nokey)`)
	require.Contains(t, bootRunner, `Keyless fixture still owns processes after native logind termination`)
	require.Contains(t, bootRunner, `/etc/ssh/sshd_config.d/41-soda-project-accounts.conf`)
	require.Contains(t, bootRunner, `/usr/libexec/soda/soda-cockpit`)
	require.NotContains(t, bootRunner, "start_x86_unattended_boot_selector")
	require.NotContains(t, bootRunner, `"execute":"send-key"`)
}

func TestAcceptanceExposesOnePublicWorkflow(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	publicPath := filepath.Join(root, "tests", "acceptance", "unattended.sh")
	publicInfo, err := os.Stat(publicPath)
	require.NoError(t, err)
	require.NotZero(t, publicInfo.Mode().Perm()&0o111)

	privatePath := filepath.Join(root, "tests", "acceptance", "internal", "bootc.sh")
	privateInfo, err := os.Stat(privatePath)
	require.NoError(t, err)
	require.Zero(t, privateInfo.Mode().Perm()&0o111)

	_, err = os.Stat(filepath.Join(root, "tests", "acceptance", "bootc.sh"))
	require.ErrorIs(t, err, os.ErrNotExist)

	runner := readInstallerFixture(t, root, "tests/acceptance/unattended.sh")
	for _, expected := range []string{
		`fallback seed-b`,
		`fallback stage "$target"`,
		`fallback compare b-current a-selected`,
		`fallback compare b-current b-restored`,
		`scenario product`,
		`capture final`,
		`registry_data=$work_dir/registry`,
		`--volume "$registry_data:/var/lib/registry"`,
	} {
		require.Contains(t, runner, expected)
	}
	for _, obsolete := range []string{"runner.env", "VNC", "vnc", "two terminals"} {
		require.NotContains(t, runner, obsolete)
	}
	require.NotContains(t, runner, "final-pre-capstone")
	for _, relative := range []string{
		"tests/acceptance/registry-image.txt",
		"tests/acceptance/skopeo-image.txt",
	} {
		value := strings.TrimSpace(readInstallerFixture(t, root, relative))
		require.Regexp(t, `^[a-z0-9./-]+@sha256:[0-9a-f]{64}$`, value)
	}
}

func readInstallerFixture(t *testing.T, root, relative string) string {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	require.NoError(t, err)
	return strings.ReplaceAll(string(contents), "\r\n", "\n")
}
