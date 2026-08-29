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
		"ARG FEDORA_BASE_REFERENCE", "org.opencontainers.image.base.name=\"${FEDORA_BASE_REFERENCE}\"", "systemd-sysusers /usr/lib/sysusers.d/soda.conf", "install -d -m 0755 /opt/soda/toolchains",
		"systemctl enable sshd.service sodad.service soda-authd.service soda-cockpit.service avahi-daemon.service var-srv-soda-projects.mount opt-soda-toolchains.mount",
		"COPY .artifacts/rpms/soda-forgejo-*.rpm /var/tmp/soda-rpms/",
		"getent passwd forgejo",
		"systemctl mask bootc-fetch-apply-updates.timer", "cp -f /usr/lib/soda/os-release /etc/os-release",
		"cp -f /usr/lib/soda/os-release /usr/lib/os-release", "cp -f /usr/lib/soda/issue /etc/issue",
		"cp -f /usr/lib/soda/issue /etc/issue.net", "cp -f /usr/lib/soda/system-release /etc/system-release",
		"cp -f /usr/lib/soda/system-release /etc/redhat-release", "semanage fcontext -a -t var_lib_t '/var/lib/soda(/.*)?'",
		"semanage fcontext -a -e /home /var/lib/soda/projects", "semanage fcontext -a -e /opt /var/lib/soda/toolchains",
		"semanage fcontext -a -e /home /var/srv/soda/projects", "semanage fcontext -a -e /opt /opt/soda/toolchains",
		"semanage fcontext -a -t var_log_t '/var/log/soda(/.*)?'", "semanage fcontext -a -t ssh_home_t '/etc/soda/authorized_keys(/.*)?'",
		"restorecon -RF /etc/soda/authorized_keys /opt/soda/toolchains", "ssh-keygen -q -t ed25519 -N '' -f /run/soda-sshd-hostkey",
		"/usr/sbin/sshd -t -h /run/soda-sshd-hostkey", "rm -f /run/soda-sshd-hostkey /run/soda-sshd-hostkey.pub",
		"--enablerepo=updates-testing", `test "$(rpm -q --qf '%{NAME}-%{EPOCHNUM}:%{VERSION}-%{RELEASE}.%{ARCH}' bootc)" = "${BOOTC_NEVRA}"`,
		"rpm -q skopeo", "/usr/libexec/soda/cosign version | grep -F 'GitVersion:    v3.1.2'",
		"bootc switch --help | grep -F -- '--download-only'", "bootc switch --help | grep -F -- '--from-downloaded'",
		"rpm-inventory.sha256", "sha256sum --check rpm-inventory.sha256", "/usr/lib/sysimage/libdnf5/transaction_history.sqlite*",
		"/var/cache/ldconfig/aux-cache", "/var/cache/libdnf5", "/var/lib/dnf/repos", "/var/log/dnf5.log", "/run/dnf",
		"COPY .artifacts/bootc/trust/registry-ca.crt /usr/share/pki/ca-trust-source/anchors/soda-registry-ca.crt",
		"COPY .artifacts/bootc/trust/cosign.pub /usr/share/soda/release/cosign.pub",
		"COPY packaging/bootc/trust/policy.json /etc/containers/policy.json",
		"COPY packaging/bootc/trust/registries.d.yaml /etc/containers/registries.d/soda.yaml", "update-ca-trust extract",
	} {
		require.Contains(t, containerfile, expected)
	}
	require.NotContains(t, containerfile, "bootc-fetch-apply-updates.service")
}

func TestRuntimeImageStateDirectoriesAndSELinuxContract(t *testing.T) {
	sysusers, err := os.ReadFile(filepath.Join("..", "..", "..", "packaging", "rpm", "runtime", "sources", "sysusers", "soda.conf"))
	require.NoError(t, err)
	require.Contains(t, string(sysusers), "g soda-api 976")
	require.Contains(t, string(sysusers), "u soda-cockpit 976:soda-api")

	tmpfiles, err := os.ReadFile(filepath.Join("..", "..", "..", "packaging", "rpm", "runtime", "sources", "tmpfiles", "soda.conf"))
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
	require.Contains(t, string(staging), `b.path("packaging/rpm/runtime/sources/systemd/soda-state-directories.service"), filepath.Join(sources, "soda-state-directories.service")`)
	require.Contains(t, string(staging), `b.path("packaging/rpm/runtime/sources/systemd/var-srv-soda-projects.mount"), filepath.Join(sources, "var-srv-soda-projects.mount")`)
	require.Contains(t, string(staging), `b.path("packaging/rpm/forgejo/sources/systemd/forgejo.service"), filepath.Join(sources, "forgejo.service")`)
	require.NotContains(t, string(staging), "00-soda-var-srv.conf")

	runtimeSpec, err := os.ReadFile(filepath.Join("..", "..", "..", "packaging", "rpm", "runtime", "soda-runtime.spec"))
	require.NoError(t, err)
	require.Contains(t, string(runtimeSpec), "install -m 0644 %{_sourcedir}/soda-state-directories.service %{buildroot}%{_unitdir}/soda-state-directories.service")
	require.Contains(t, string(runtimeSpec), "%{_unitdir}/soda-state-directories.service")
	require.Contains(t, string(runtimeSpec), "install -m 0644 %{_sourcedir}/var-srv-soda-projects.mount %{buildroot}%{_unitdir}/var-srv-soda-projects.mount")
	require.Contains(t, string(runtimeSpec), "%{_unitdir}/var-srv-soda-projects.mount")
	require.Contains(t, string(runtimeSpec), "soda-forgejo = 15.0.7")
	require.NotContains(t, string(runtimeSpec), "00-soda-var-srv.conf")
}

func TestForgejoPackagingContract(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	forgejoRoot := filepath.Join(root, "packaging", "rpm", "forgejo")
	spec, err := os.ReadFile(filepath.Join(forgejoRoot, "soda-forgejo.spec"))
	require.NoError(t, err)
	require.Contains(t, string(spec), "Version:        15.0.7")
	require.Contains(t, string(spec), "Pinned PAM-enabled Forgejo runtime")
	require.Contains(t, string(spec), "%{_unitdir}/forgejo.service")

	unit, err := os.ReadFile(filepath.Join(forgejoRoot, "sources", "systemd", "forgejo.service"))
	require.NoError(t, err)
	require.Contains(t, string(unit), "User=forgejo")
	require.Contains(t, string(unit), "ExecStart=/usr/bin/forgejo web --config /etc/forgejo/app.ini")
	require.Contains(t, string(unit), "ReadWritePaths=/var/lib/forgejo")

	configuration, err := os.ReadFile(filepath.Join(forgejoRoot, "sources", "app.ini.tmpl"))
	require.NoError(t, err)
	require.Contains(t, string(configuration), "HTTP_ADDR = 127.0.0.1")
	require.Contains(t, string(configuration), "START_SSH_SERVER = false")
	require.Contains(t, string(configuration), "DISABLE_REGISTRATION = true")
	require.Contains(t, string(configuration), "ENABLED = false")

	sourceLock, err := os.ReadFile(filepath.Join(root, "distro", "locks", "forgejo-source.toml"))
	require.NoError(t, err)
	require.Contains(t, string(sourceLock), `version = "15.0.7"`)
	require.Contains(t, string(sourceLock), `sha256 = "`+forgejoSourceSHA256+`"`)
	require.Contains(t, string(sourceLock), `build_tags = "bindata timetzdata sqlite sqlite_unlock_notify pam"`)
}

func TestRuntimeImageSystemdMountAndLoggingContract(t *testing.T) {
	runtimeSources := filepath.Join("..", "..", "..", "packaging", "rpm", "runtime", "sources")
	preset, err := os.ReadFile(filepath.Join(runtimeSources, "systemd", "90-soda.preset"))
	require.NoError(t, err)
	for _, unit := range []string{"sshd.service", "sodad.service", "soda-authd.service", "soda-cockpit.service", "avahi-daemon.service", "var-srv-soda-projects.mount", "opt-soda-toolchains.mount"} {
		require.True(t, strings.Contains(string(preset), "enable "+unit))
	}
	require.NotContains(t, string(preset), "forgejo.service")

	projectMount, err := os.ReadFile(filepath.Join(runtimeSources, "systemd", "var-srv-soda-projects.mount"))
	require.NoError(t, err)
	require.Contains(t, string(projectMount), "Requires=soda-state-directories.service")
	require.Contains(t, string(projectMount), "After=soda-state-directories.service")
	require.NotContains(t, string(projectMount), "After=systemd-tmpfiles-setup.service")
	require.Contains(t, string(projectMount), "What=/var/lib/soda/projects")
	require.Contains(t, string(projectMount), "Where=/var/srv/soda/projects")
	require.Contains(t, string(projectMount), "Options=bind")

	stateDirectories, err := os.ReadFile(filepath.Join(runtimeSources, "systemd", "soda-state-directories.service"))
	require.NoError(t, err)
	require.Contains(t, string(stateDirectories), "DefaultDependencies=no")
	require.Contains(t, string(stateDirectories), "RequiresMountsFor=/var")
	require.Contains(t, string(stateDirectories), "Before=local-fs.target var-srv-soda-projects.mount opt-soda-toolchains.mount")
	require.Contains(t, string(stateDirectories), "ExecStart=/usr/bin/systemd-tmpfiles --create --prefix=/var/lib/soda --prefix=/var/srv/soda")

	toolchainMount, err := os.ReadFile(filepath.Join(runtimeSources, "systemd", "opt-soda-toolchains.mount"))
	require.NoError(t, err)
	require.Contains(t, string(toolchainMount), "Requires=soda-state-directories.service")
	require.Contains(t, string(toolchainMount), "After=soda-state-directories.service")
	require.NotContains(t, string(toolchainMount), "After=systemd-tmpfiles-setup.service")

	sodadUnit, err := os.ReadFile(filepath.Join(runtimeSources, "systemd", "sodad.service"))
	require.NoError(t, err)
	require.Contains(t, string(sodadUnit), "Requires=var-srv-soda-projects.mount opt-soda-toolchains.mount")
	require.Contains(t, string(sodadUnit), "After=local-fs.target network-online.target var-srv-soda-projects.mount opt-soda-toolchains.mount")

	services := []string{
		filepath.Join(runtimeSources, "systemd", "sodad.service"),
		filepath.Join("..", "..", "..", "packaging", "rpm", "cockpit", "sources", "systemd", "soda-authd.service"),
		filepath.Join("..", "..", "..", "packaging", "rpm", "cockpit", "sources", "systemd", "soda-cockpit.service"),
	}
	for _, service := range services {
		unit, readErr := os.ReadFile(service)
		require.NoError(t, readErr)
		require.Contains(t, string(unit), "StandardOutput=append:/var/log/soda/")
		require.NotContains(t, string(unit), "LogsDirectory=")
	}
	cockpitUnit, err := os.ReadFile(filepath.Join("..", "..", "..", "packaging", "rpm", "cockpit", "sources", "systemd", "soda-cockpit.service"))
	require.NoError(t, err)
	require.Contains(t, string(cockpitUnit), "ReadWritePaths=/var/lib/soda/certs /var/log/soda/soda-cockpit")
}
