package installer

import (
	"encoding/xml"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSodaInstallerSpokeUsesNativeAccountsAndFourInputs(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	addon := filepath.Join(root, "packaging", "installer", "addons", "org_fedoraproject_soda")
	required := []string{
		"__init__.py", "constants.py", "service/__main__.py", "service/installer.py",
		"service/interface.py", "service/installation.py", "service/kickstart.py",
		"gui/spokes/installation.py", "gui/spokes/installation.glade",
	}
	for _, name := range required {
		_, err := os.Stat(filepath.Join(addon, name))
		require.NoErrorf(t, err, "missing Anaconda add-on file %s", name)
	}
	for _, obsolete := range []string{
		"service/identity.py", "gui/spokes/identity.py", "gui/spokes/identity.glade",
	} {
		_, err := os.Stat(filepath.Join(addon, obsolete))
		require.ErrorIs(t, err, os.ErrNotExist)
	}

	containerfile, err := os.ReadFile(filepath.Join(root, "packaging", "installer", "Containerfile"))
	require.NoError(t, err)
	require.Contains(t, string(containerfile), "COPY packaging/installer/addons/org_fedoraproject_soda /usr/share/anaconda/addons/org_fedoraproject_soda")
	require.Contains(t, string(containerfile), "org.fedoraproject.Anaconda.Addons.SodaInstaller.service")
	require.Contains(t, string(containerfile), "org.fedoraproject.Anaconda.Addons.SodaInstaller.conf")
	require.Contains(t, string(containerfile), "packaging/installer/var-tmp.mount /usr/lib/systemd/system/var-tmp.mount")
	require.Contains(t, string(containerfile), "anaconda.target.wants/var-tmp.mount")
	require.NotContains(t, string(containerfile), "org.fedoraproject.Anaconda.Modules.Payloads.service")
	require.NotContains(t, string(containerfile), "SodaIdentity")

	profile, err := os.ReadFile(filepath.Join(root, "packaging", "installer", "branding", "sodaos.conf"))
	require.NoError(t, err)
	require.Contains(t, string(profile), "can_copy_input_kickstart = False")
	require.Contains(t, string(profile), "hidden_spokes = UserSpoke PasswordSpoke")
	require.NotContains(t, string(profile), "org.fedoraproject.Anaconda.Addons.SodaInstaller")

	spoke, err := os.ReadFile(filepath.Join(addon, "gui", "spokes", "installation.py"))
	require.NoError(t, err)
	for _, expected := range []string{
		"GUISpokeInputCheckHandler", "PasswordChecker", "PASSWORD_POLICY_USER",
		"UsernameCheck", "PasswordEmptyCheck", "PasswordConfirmationCheck",
		"PasswordValidityCheck", "PasswordASCIICheck", "try_to_go_back",
		"UserData", "set_admin_priviledges(True)", "SshKeyData", "/usr/bin/ssh-keygen",
		"user.is_crypted = False", `self.password_entry.set_text("")`,
	} {
		require.Contains(t, string(spoke), expected)
	}
	require.NotContains(t, string(spoke), "set_text(user.password)")
	require.NotContains(t, string(spoke), "emailEntry")
}

func TestSodaInstallerTaskUsesBoundedNativeForgejoAndSecrets(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	addon := filepath.Join(root, "packaging", "installer", "addons", "org_fedoraproject_soda")
	installation, err := os.ReadFile(filepath.Join(addon, "service", "installation.py"))
	require.NoError(t, err)
	for _, expected := range []string{
		"get_user_list(self._users)", "has_admin_priviledges", "etc/passwd", "etc/group",
		".ssh/authorized_keys", "crypt_password", "set_user_list", "os.memfd_create",
		`["/usr/bin/mount", "--bind"`,
		`"/usr/bin/setfiles"`, `"-F"`, `"-r"`, `self._sysroot / "var" / "home" / username`,
		`"targeted"`, `"file_contexts"`, `logical_home.samefile(physical_home)`,
		"fcntl.F_SEAL_WRITE", "/proc/self/fd/", `"/user/sign_up"`,
		`"email": f"{username}@localhost"`, "DISABLE_REGISTRATION = false",
		`"/usr/bin/systemd-tmpfiles"`, `"forgejo.conf"`,
		"TAILSCALE_KEY_PATH", "os.O_EXCL", "os.O_NOFOLLOW", "os.replace",
		"Anaconda's later SetContextsTask relabels /var/lib",
	} {
		require.Contains(t, string(installation), expected)
	}
	require.NotContains(t, string(installation), "installer-admin.json")
	require.NotContains(t, string(installation), "--password")
	require.NotContains(t, string(installation), `"/usr/sbin/restorecon"`)
	require.NotContains(t, string(installation), `self._physical_root`)
	require.NotContains(t, string(installation), `ostree/deploy`)
	require.NotContains(t, string(installation), `persistent bootc variable-data`)

	service, err := os.ReadFile(filepath.Join(addon, "service", "installer.py"))
	require.NoError(t, err)
	require.Contains(t, string(service), "ProvisionSodaInstallationTask")
	require.Contains(t, string(service), "conf.target.system_root")
	require.NotContains(t, string(service), "conf.target.physical_root")
	require.NotContains(t, string(service), "return []")

	iface, err := os.ReadFile(filepath.Join(addon, "service", "interface.py"))
	require.NoError(t, err)
	require.Contains(t, string(iface), "SetTailscaleAuthKey")
	require.NotContains(t, strings.ToLower(string(iface)), "password")

	kickstart, err := os.ReadFile(filepath.Join(addon, "service", "kickstart.py"))
	require.NoError(t, err)
	require.Contains(t, string(kickstart), "tailscale_auth_key")
	require.Contains(t, string(kickstart), `return ""`)

	busPolicy, err := os.ReadFile(filepath.Join(root, "packaging", "installer", "addons", "org.fedoraproject.Anaconda.Addons.SodaInstaller.conf"))
	require.NoError(t, err)
	require.Contains(t, string(busPolicy), `<policy user="root">`)
	require.Contains(t, string(busPolicy), `<deny send_destination="org.fedoraproject.Anaconda.Addons.SodaInstaller"/>`)
}

func TestSodaInstallerAddonDocumentsAndPythonParse(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	addon := filepath.Join(root, "packaging", "installer", "addons", "org_fedoraproject_soda")
	for _, document := range []string{
		filepath.Join(addon, "gui", "spokes", "installation.glade"),
		filepath.Join(root, "packaging", "installer", "addons", "org.fedoraproject.Anaconda.Addons.SodaInstaller.conf"),
	} {
		file, err := os.Open(document)
		require.NoError(t, err)
		var parsed struct{}
		require.NoError(t, xml.NewDecoder(file).Decode(&parsed))
		require.NoError(t, file.Close())
	}

	var pythonFiles []string
	require.NoError(t, filepath.WalkDir(addon, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr == nil && !entry.IsDir() && filepath.Ext(path) == ".py" {
			pythonFiles = append(pythonFiles, path)
		}
		return walkErr
	}))
	for _, path := range pythonFiles {
		check := exec.Command("python3", "-c", "import ast, pathlib, sys; ast.parse(pathlib.Path(sys.argv[1]).read_text())", path)
		output, checkErr := check.CombinedOutput()
		require.NoErrorf(t, checkErr, "invalid Python in %s:\n%s", path, output)
	}

	spoke, err := os.ReadFile(filepath.Join(addon, "gui", "spokes", "installation.py"))
	require.NoError(t, err)
	require.Contains(t, string(spoke), "for check in (\n            self._username_check,")
	require.Contains(t, string(spoke), "self.checker.add_check(check)")
}

func TestTailscaleEnrollmentIsOneAttemptAndAlwaysCleansSecrets(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	unitPath := filepath.Join(root, "packaging", "rpm", "runtime", "sources", "systemd", "soda-tailscale-enroll.service")
	unit, err := os.ReadFile(unitPath)
	require.NoError(t, err)
	for _, expected := range []string{
		"Wants=network-online.target tailscaled.service",
		"After=network-online.target tailscaled.service",
		"Type=oneshot", "TimeoutStartSec=2min",
		"ExecStart=/usr/bin/tailscale up --auth-key=file:/var/lib/soda-install/tailscale-auth-key",
		"ExecStopPost=-/usr/bin/unlink /var/lib/soda-install/tailscale-auth-key",
		"ExecStopPost=-/usr/bin/rmdir /var/lib/soda-install",
		"ExecStopPost=-/usr/bin/systemctl --no-reload --quiet disable soda-tailscale-enroll.service",
	} {
		require.Contains(t, string(unit), expected)
	}
	for _, forbidden := range []string{"ConditionPathExists", "Restart=", "Requires=", "Environment="} {
		require.NotContains(t, string(unit), forbidden)
	}

	preset, err := os.ReadFile(filepath.Join(root, "packaging", "rpm", "runtime", "sources", "systemd", "90-soda.preset"))
	require.NoError(t, err)
	require.Contains(t, string(preset), "enable soda-tailscale-enroll.service")
	require.NotContains(t, string(preset), "soda-installer-import.service")

	for _, obsolete := range []string{
		filepath.Join(root, "packaging", "rpm", "runtime", "sources", "systemd", "soda-installer-import.service"),
		filepath.Join(root, "packaging", "rpm", "runtime", "sources", "systemd", "tailscaled.service.d", "10-soda-state.conf"),
	} {
		_, err := os.Stat(obsolete)
		require.ErrorIs(t, err, os.ErrNotExist)
	}
}

func TestUnattendedInstallerInputsMatchTheFourValueContract(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	runner, err := os.ReadFile(filepath.Join(root, "tests", "acceptance", "unattended.sh"))
	require.NoError(t, err)
	for _, expected := range []string{
		"SODA_ACCEPTANCE_TAILSCALE_AUTH_KEY_FILE",
		`user --name=$admin --groups=wheel --password="$password" --plaintext`,
		`sshkey --username=$admin "$public_key"`,
		"tailscale_auth_key=$tailscale_auth_key",
		"os.lstat(source_name)",
		"source_stat.st_mode & 0o077",
		"Tailscale auth key input must remain outside acceptance evidence",
		"aarch64|arm64)",
		"expected_platform=linux/arm64",
		"expected_platform=linux/amd64",
		"export SODA_ACCEPTANCE_ARCHITECTURE=$architecture",
		"%pre --erroronfail",
		"/dev/disk/by-label/OEMDRV",
		`/usr/bin/eject "\$oemdrv"`,
		`eject_attempts=\$((eject_attempts + 1))`,
		"The parsed OEMDRV installer input was not removed after guest ejection",
		`scratch_type=\$(findmnt -n -o FSTYPE --target /var/tmp)`,
		`scratch_size=\$(findmnt -n -b -o SIZE --target /var/tmp)`,
		`[ "\$scratch_type" = tmpfs ] && [ "\$scratch_size" -ge 4294967296 ]`,
		`rm -f "$kickstart"`,
		"trap abort_prepare 1 2 15",
	} {
		require.Contains(t, string(runner), expected)
	}
	for _, obsolete := range []string{
		"SODA_ACCEPTANCE_ADMIN_NAME", "SODA_ACCEPTANCE_ADMIN_EMAIL",
		"name=$admin_name", "email=$admin_email", "/etc/soda/authorized_keys",
		"soda-acceptance-kickstart-consumed",
	} {
		require.NotContains(t, string(runner), obsolete)
	}

	bootRunner, err := os.ReadFile(filepath.Join(root, "tests", "acceptance", "bootc.sh"))
	require.NoError(t, err)
	require.Contains(t, string(bootRunner), "id=soda-oemdrv-device")
	require.Contains(t, string(bootRunner), `rm -f "$installer_input"`)
	require.Contains(t, string(bootRunner), "start_x86_unattended_boot_selector")
	require.Contains(t, string(bootRunner), "installer-boot-override.jsonl")
	require.Contains(t, string(bootRunner), "down down end spc i n s t dot c m d l i n e")
	require.Contains(t, string(bootRunner), "installer-input-eject.jsonl")
	require.Contains(t, string(bootRunner), `{"execute":"query-block","id":"soda-oemdrv-guest-ejected"}`)
	require.Contains(t, string(bootRunner), `.device == "soda-oemdrv"`)
	require.Contains(t, string(bootRunner), `.qdev == "soda-oemdrv-device"`)
	require.Contains(t, string(bootRunner), `.removable == true`)
	require.Contains(t, string(bootRunner), `.tray_open == true`)
	require.Contains(t, string(bootRunner), `.locked == false`)
	require.Contains(t, string(bootRunner), `{"execute":"blockdev-remove-medium","arguments":{"id":"soda-oemdrv-device"},"id":"soda-oemdrv-remove-medium"}`)
	require.Contains(t, string(bootRunner), "soda-oemdrv-guest-ejected")
	require.Contains(t, string(bootRunner), "soda-oemdrv-medium-absent")
	require.NotContains(t, string(bootRunner), "soda-acceptance-kickstart-consumed")
	require.NotContains(t, string(bootRunner), `"execute":"blockdev-open-tray"`)
	require.NotContains(t, string(bootRunner), `"force":true`)
	require.Contains(t, string(bootRunner), "-m 8192")
	require.Contains(t, string(bootRunner), "-boot order=c,once=d")
	require.Contains(t, string(bootRunner), "-boot order=c")
	require.Contains(t, string(bootRunner), `"$admin@$guest_host"`)
	require.Contains(t, string(bootRunner), `https://$guest_host:$guest_cockpit_port/ping`)
}

func TestInstallerPayloadUsesRAMBackedTemporaryStorage(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	mountPath := filepath.Join(root, "packaging", "installer", "var-tmp.mount")
	mount, err := os.ReadFile(mountPath)
	require.NoError(t, err)
	require.Equal(t, "[Unit]\nDescription=Ephemeral Anaconda payload scratch\nBefore=anaconda.target anaconda.service\n\n[Mount]\nWhat=tmpfs\nWhere=/var/tmp\nType=tmpfs\nOptions=mode=1777,size=4G\n\n[Install]\nWantedBy=anaconda.target\n", string(mount))
	require.NotContains(t, string(mount), "nosuid")
	require.NotContains(t, string(mount), "/mnt/sysimage")
}
