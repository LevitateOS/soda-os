package image

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRuntimeImageBootcContainerContract(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join("..", "..", "..", "packaging", "bootc", "Containerfile"))
	require.NoError(t, err)
	containerfile := string(contents)
	require.True(t, strings.HasPrefix(containerfile, "FROM fedora-base\n"))
	for _, expected := range []string{
		bootcBaseReference, "systemd-sysusers /usr/lib/sysusers.d/soda.conf", "install -d -m 0755 /opt/soda/toolchains",
		"systemctl enable sshd.service sodad.service soda-authd.service soda-cockpit.service avahi-daemon.service var-srv-soda-projects.mount opt-soda-toolchains.mount",
		"systemctl mask bootc-fetch-apply-updates.timer", "cp -f /usr/lib/soda/os-release /etc/os-release",
		"cp -f /usr/lib/soda/os-release /usr/lib/os-release", "cp -f /usr/lib/soda/issue /etc/issue",
		"cp -f /usr/lib/soda/issue /etc/issue.net", "cp -f /usr/lib/soda/system-release /etc/system-release",
		"cp -f /usr/lib/soda/system-release /etc/redhat-release", "semanage fcontext -a -t var_lib_t '/var/lib/soda(/.*)?'",
		"semanage fcontext -a -e /home /var/lib/soda/projects", "semanage fcontext -a -e /opt /var/lib/soda/toolchains",
		"semanage fcontext -a -e /home /var/srv/soda/projects", "semanage fcontext -a -e /opt /opt/soda/toolchains",
		"semanage fcontext -a -t var_log_t '/var/log/soda(/.*)?'", "semanage fcontext -a -t ssh_home_t '/etc/soda/authorized_keys(/.*)?'",
		"restorecon -RF /etc/soda/authorized_keys /opt/soda/toolchains", "ssh-keygen -q -t ed25519 -N '' -f /run/soda-sshd-hostkey",
		"/usr/sbin/sshd -t -h /run/soda-sshd-hostkey", "rm -f /run/soda-sshd-hostkey /run/soda-sshd-hostkey.pub",
		"--enablerepo=updates-testing", `test "$(rpm -q --qf '%{NAME}-%{EPOCHNUM}:%{VERSION}-%{RELEASE}.%{ARCH}' bootc)" = "bootc-0:1.16.10-1.fc44.aarch64"`,
		"rpm -q skopeo", "/usr/libexec/soda/cosign version | grep -F 'GitVersion:    v3.1.2'",
		"bootc switch --help | grep -F -- '--download-only'", "bootc switch --help | grep -F -- '--from-downloaded'",
		"rpm-inventory.sha256", "sha256sum --check rpm-inventory.sha256", "/usr/lib/sysimage/libdnf5/transaction_history.sqlite*",
		"/var/cache/ldconfig/aux-cache", "/var/cache/libdnf5", "/var/lib/dnf/repos", "/var/log/dnf5.log", "/run/dnf",
		"COPY .artifacts/bootc/trust/registry-ca.crt /usr/share/pki/ca-trust-source/anchors/soda-registry-ca.crt",
		"COPY .artifacts/bootc/trust/cosign.pub /usr/share/soda/release/cosign.pub",
		"COPY packaging/release/policy.json /etc/containers/policy.json",
		"COPY packaging/release/registries.d.yaml /etc/containers/registries.d/soda.yaml", "update-ca-trust extract",
	} {
		require.Contains(t, containerfile, expected)
	}
	require.NotContains(t, containerfile, "bootc-fetch-apply-updates.service")
}

func TestRuntimeImageStateDirectoriesAndSELinuxContract(t *testing.T) {
	sysusers, err := os.ReadFile(filepath.Join("..", "..", "..", "packaging", "sysusers.d", "soda.conf"))
	require.NoError(t, err)
	require.Contains(t, string(sysusers), "g soda-api 976")
	require.Contains(t, string(sysusers), "u soda-cockpit 976:soda-api")

	tmpfiles, err := os.ReadFile(filepath.Join("..", "..", "..", "packaging", "tmpfiles.d", "soda.conf"))
	require.NoError(t, err)
	for _, path := range []string{"/var/lib/soda", "/var/lib/soda/projects", "/var/lib/soda/toolchains", "/var/log/soda", "/var/log/soda/sodad", "/var/log/soda/soda-authd", "/var/log/soda/soda-cockpit", "/var/srv/soda", "/var/srv/soda/projects"} {
		require.Contains(t, string(tmpfiles), "d "+path+" ", "first-boot tmpfiles must create %s after the image installs its SELinux fcontext mapping", path)
	}
	require.NotRegexp(t, `(?m)^d /srv/`, string(tmpfiles))
	require.NotRegexp(t, `(?m)^d /opt/`, string(tmpfiles))
	require.Contains(t, string(tmpfiles), "d /var/log/soda/soda-cockpit 0750 soda-cockpit soda-api -")
}

func TestRuntimeImageRPMStagingAndPackageContract(t *testing.T) {
	staging, err := os.ReadFile("rpm.go")
	require.NoError(t, err)
	require.Contains(t, string(staging), `b.path("packaging/systemd/soda-state-directories.service"), filepath.Join(sources, "soda-state-directories.service")`)
	require.Contains(t, string(staging), `b.path("packaging/systemd/var-srv-soda-projects.mount"), filepath.Join(sources, "var-srv-soda-projects.mount")`)
	require.NotContains(t, string(staging), "00-soda-var-srv.conf")

	runtimeSpec, err := os.ReadFile(filepath.Join("..", "..", "..", "packaging", "rpm", "soda-runtime.spec"))
	require.NoError(t, err)
	require.Contains(t, string(runtimeSpec), "install -m 0644 %{_sourcedir}/soda-state-directories.service %{buildroot}%{_unitdir}/soda-state-directories.service")
	require.Contains(t, string(runtimeSpec), "%{_unitdir}/soda-state-directories.service")
	require.Contains(t, string(runtimeSpec), "install -m 0644 %{_sourcedir}/var-srv-soda-projects.mount %{buildroot}%{_unitdir}/var-srv-soda-projects.mount")
	require.Contains(t, string(runtimeSpec), "%{_unitdir}/var-srv-soda-projects.mount")
	require.NotContains(t, string(runtimeSpec), "00-soda-var-srv.conf")
}

func TestRuntimeImageSystemdMountAndLoggingContract(t *testing.T) {
	preset, err := os.ReadFile(filepath.Join("..", "..", "..", "packaging", "systemd", "90-soda.preset"))
	require.NoError(t, err)
	for _, unit := range []string{"sshd.service", "sodad.service", "soda-authd.service", "soda-cockpit.service", "avahi-daemon.service", "var-srv-soda-projects.mount", "opt-soda-toolchains.mount"} {
		require.True(t, strings.Contains(string(preset), "enable "+unit))
	}

	projectMount, err := os.ReadFile(filepath.Join("..", "..", "..", "packaging", "systemd", "var-srv-soda-projects.mount"))
	require.NoError(t, err)
	require.Contains(t, string(projectMount), "Requires=soda-state-directories.service")
	require.Contains(t, string(projectMount), "After=soda-state-directories.service")
	require.NotContains(t, string(projectMount), "After=systemd-tmpfiles-setup.service")
	require.Contains(t, string(projectMount), "What=/var/lib/soda/projects")
	require.Contains(t, string(projectMount), "Where=/var/srv/soda/projects")
	require.Contains(t, string(projectMount), "Options=bind")

	stateDirectories, err := os.ReadFile(filepath.Join("..", "..", "..", "packaging", "systemd", "soda-state-directories.service"))
	require.NoError(t, err)
	require.Contains(t, string(stateDirectories), "DefaultDependencies=no")
	require.Contains(t, string(stateDirectories), "RequiresMountsFor=/var")
	require.Contains(t, string(stateDirectories), "Before=local-fs.target var-srv-soda-projects.mount opt-soda-toolchains.mount")
	require.Contains(t, string(stateDirectories), "ExecStart=/usr/bin/systemd-tmpfiles --create --prefix=/var/lib/soda --prefix=/var/srv/soda")

	toolchainMount, err := os.ReadFile(filepath.Join("..", "..", "..", "packaging", "systemd", "opt-soda-toolchains.mount"))
	require.NoError(t, err)
	require.Contains(t, string(toolchainMount), "Requires=soda-state-directories.service")
	require.Contains(t, string(toolchainMount), "After=soda-state-directories.service")
	require.NotContains(t, string(toolchainMount), "After=systemd-tmpfiles-setup.service")

	sodadUnit, err := os.ReadFile(filepath.Join("..", "..", "..", "packaging", "systemd", "sodad.service"))
	require.NoError(t, err)
	require.Contains(t, string(sodadUnit), "Requires=var-srv-soda-projects.mount opt-soda-toolchains.mount")
	require.Contains(t, string(sodadUnit), "After=local-fs.target network-online.target var-srv-soda-projects.mount opt-soda-toolchains.mount")

	for _, service := range []string{"sodad.service", "soda-authd.service", "soda-cockpit.service"} {
		unit, readErr := os.ReadFile(filepath.Join("..", "..", "..", "packaging", "systemd", service))
		require.NoError(t, readErr)
		require.Contains(t, string(unit), "StandardOutput=append:/var/log/soda/")
		require.NotContains(t, string(unit), "LogsDirectory=")
	}
	cockpitUnit, err := os.ReadFile(filepath.Join("..", "..", "..", "packaging", "systemd", "soda-cockpit.service"))
	require.NoError(t, err)
	require.Contains(t, string(cockpitUnit), "ReadWritePaths=/var/lib/soda/certs /var/log/soda/soda-cockpit")
}
