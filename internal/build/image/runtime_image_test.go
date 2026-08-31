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
		"systemctl enable sshd.service sodad.service soda-tailscale-enroll.service soda-authd.service soda-cockpit.service forgejo.service avahi-daemon.service tailscaled.service nftables.service var-srv-soda-projects.mount opt-soda-toolchains.mount",
		"COPY .artifacts/rpms/soda-forgejo-*.rpm /var/tmp/soda-rpms/",
		"getent passwd git",
		"systemctl mask bootc-fetch-apply-updates.timer", "cp -f /usr/lib/soda/os-release /etc/os-release",
		"cp -f /usr/lib/soda/os-release /usr/lib/os-release", "cp -f /usr/lib/soda/issue /etc/issue",
		"cp -f /usr/lib/soda/issue /etc/issue.net", "cp -f /usr/lib/soda/system-release /etc/system-release",
		"rm -f /etc/redhat-release", "semanage fcontext -a -t var_lib_t '/var/lib/soda(/.*)?'",
		"semanage fcontext -a -e /home /var/lib/soda/projects", "semanage fcontext -a -e /opt /var/lib/soda/toolchains",
		"semanage fcontext -a -e /home /var/srv/soda/projects", "semanage fcontext -a -e /opt /opt/soda/toolchains",
		"semanage fcontext -a -t var_log_t '/var/log/soda(/.*)?'", "semanage fcontext -a -t ssh_home_t '/etc/soda/authorized_keys(/.*)?'",
		"semanage fcontext -a -t ssh_home_t '/var/lib/forgejo/.ssh(/.*)?'", "restorecon -RF /etc/soda/authorized_keys /var/lib/forgejo/.ssh /opt/soda/toolchains", "ssh-keygen -q -t ed25519 -N '' -f /run/soda-sshd-hostkey",
		"/usr/sbin/sshd -t -h /run/soda-sshd-hostkey", "rm -f /run/soda-sshd-hostkey /run/soda-sshd-hostkey.pub",
		"--enablerepo=updates-testing", `test "$(rpm -q --qf '%{NAME}-%{EPOCHNUM}:%{VERSION}-%{RELEASE}.%{ARCH}' bootc)" = "${BOOTC_NEVRA}"`,
		"rpm -q skopeo",
		"bootc switch --help | grep -F -- '--download-only'", "bootc switch --help | grep -F -- '--from-downloaded'",
		"rpm-inventory.sha256", "sha256sum --check rpm-inventory.sha256", "/usr/lib/sysimage/libdnf5/transaction_history.sqlite*",
		"/var/cache/ldconfig/aux-cache", "/var/cache/libdnf5", "/var/lib/dnf/repos", "/var/log/dnf5.log", "/run/dnf",
		"COPY .artifacts/bootc/distribution/distribution.json /usr/share/soda/release/distribution.json",
		`org.sodaos.state-schema="4"`,
	} {
		require.Contains(t, containerfile, expected)
	}
	require.NotContains(t, containerfile, "cp -f /usr/lib/soda/system-release /etc/redhat-release")
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
	require.Contains(t, string(staging), `b.path("packaging/rpm/runtime/sources/systemd/soda-tailscale-enroll.service"), filepath.Join(sources, "soda-tailscale-enroll.service")`)
	require.Contains(t, string(staging), `b.path("packaging/rpm/runtime/sources/nftables/soda-ingress.nft"), filepath.Join(sources, "soda-ingress.nft")`)
	require.Contains(t, string(staging), `b.path("packaging/rpm/runtime/sources/systemd/nftables.service.d/10-soda-ingress.conf"), filepath.Join(sources, "10-soda-ingress.conf")`)
	require.NotContains(t, string(staging), "soda-installer-import.service")
	require.NotContains(t, string(staging), "10-soda-state.conf")
	require.Contains(t, string(staging), `b.path("packaging/rpm/runtime/sources/systemd/var-srv-soda-projects.mount"), filepath.Join(sources, "var-srv-soda-projects.mount")`)
	require.Contains(t, string(staging), `b.path("packaging/rpm/forgejo/sources/systemd/forgejo.service"), filepath.Join(sources, "forgejo.service")`)
	require.Contains(t, string(staging), `filepath.Join(build, "soda-forgejo-tailnet"), filepath.Join(sources, "forgejo-tailnet")`)
	require.Contains(t, string(staging), `b.path("packaging/rpm/forgejo/sources/pam/soda-forgejo"), filepath.Join(sources, "soda-forgejo.pam")`)
	require.NotContains(t, string(staging), "00-soda-var-srv.conf")

	runtimeSpec, err := os.ReadFile(filepath.Join("..", "..", "..", "packaging", "rpm", "runtime", "soda-runtime.spec"))
	require.NoError(t, err)
	require.Contains(t, string(runtimeSpec), "install -m 0644 %{_sourcedir}/soda-state-directories.service %{buildroot}%{_unitdir}/soda-state-directories.service")
	require.Contains(t, string(runtimeSpec), "%{_unitdir}/soda-state-directories.service")
	require.Contains(t, string(runtimeSpec), "install -m 0644 %{_sourcedir}/soda-tailscale-enroll.service %{buildroot}%{_unitdir}/soda-tailscale-enroll.service")
	require.Contains(t, string(runtimeSpec), "%{_unitdir}/soda-tailscale-enroll.service")
	require.Contains(t, string(runtimeSpec), "tailscale")
	require.Contains(t, string(runtimeSpec), "nftables-services")
	require.Contains(t, string(runtimeSpec), "install -m 0644 %{_sourcedir}/soda-ingress.nft %{buildroot}%{_prefix}/lib/soda/network/soda-ingress.nft")
	require.Contains(t, string(runtimeSpec), "install -m 0644 %{_sourcedir}/10-soda-ingress.conf %{buildroot}%{_unitdir}/nftables.service.d/10-soda-ingress.conf")
	require.NotContains(t, string(runtimeSpec), "soda-installer-import.service")
	require.NotContains(t, string(runtimeSpec), "10-soda-state.conf")
	require.Contains(t, string(runtimeSpec), "install -m 0644 %{_sourcedir}/var-srv-soda-projects.mount %{buildroot}%{_unitdir}/var-srv-soda-projects.mount")
	require.Contains(t, string(runtimeSpec), "%{_unitdir}/var-srv-soda-projects.mount")
	require.Contains(t, string(runtimeSpec), "soda-forgejo = 15.0.7")
	require.NotContains(t, string(runtimeSpec), "00-soda-var-srv.conf")

	releaseSpec, err := os.ReadFile(filepath.Join("..", "..", "..", "packaging", "rpm", "release", "soda-release.spec"))
	require.NoError(t, err)
	require.Contains(t, string(releaseSpec), `%{_prefix}/lib/soda/os-release`)
	require.NotContains(t, string(releaseSpec), `%{_sysconfdir}/soda-release`)
}

func TestForgejoPackagingContract(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	forgejoRoot := filepath.Join(root, "packaging", "rpm", "forgejo")
	spec, err := os.ReadFile(filepath.Join(forgejoRoot, "soda-forgejo.spec"))
	require.NoError(t, err)
	require.Contains(t, string(spec), "Version:        15.0.7")
	require.Contains(t, string(spec), "Pinned PAM-enabled Forgejo runtime")
	require.Contains(t, string(spec), "%{_unitdir}/forgejo.service")
	require.Contains(t, string(spec), "%{_sysconfdir}/pam.d/soda-forgejo")

	unit, err := os.ReadFile(filepath.Join(forgejoRoot, "sources", "systemd", "forgejo.service"))
	require.NoError(t, err)
	require.Contains(t, string(unit), "User=git")
	require.Contains(t, string(unit), "ExecStart=/usr/bin/forgejo web --config /etc/forgejo/app.ini")
	require.Contains(t, string(unit), "ReadWritePaths=/var/lib/forgejo")

	configuration, err := os.ReadFile(filepath.Join(forgejoRoot, "sources", "app.ini.tmpl"))
	require.NoError(t, err)
	require.Contains(t, string(configuration), "HTTP_ADDR = 127.0.0.1")
	require.Contains(t, string(configuration), "HTTP_PORT = 30000")
	require.Contains(t, string(configuration), "START_SSH_SERVER = false")
	require.Contains(t, string(configuration), "DISABLE_REGISTRATION = true")
	require.Contains(t, string(configuration), "ENABLED = false")
	require.Contains(t, string(configuration), "SSH_USER = git")
	require.Contains(t, string(configuration), "SSH_CREATE_AUTHORIZED_KEYS_FILE = true")

	sysusers, err := os.ReadFile(filepath.Join(forgejoRoot, "sources", "sysusers", "forgejo.conf"))
	require.NoError(t, err)
	require.Contains(t, string(sysusers), "u git 975")

	initialization, err := os.ReadFile(filepath.Join(forgejoRoot, "sources", "forgejo-init"))
	require.NoError(t, err)
	require.Contains(t, string(initialization), "forgejo admin auth add-pam")
	require.Contains(t, string(initialization), "--service-name soda-forgejo")

	sourceLock, err := os.ReadFile(filepath.Join(root, "distro", "locks", "forgejo-source.toml"))
	require.NoError(t, err)
	require.Contains(t, string(sourceLock), `version = "15.0.7"`)
	require.Contains(t, string(sourceLock), `sha256 = "`+forgejoSourceSHA256+`"`)
	require.Contains(t, string(sourceLock), `build_tags = "bindata timetzdata sqlite sqlite_unlock_notify pam"`)

	buildPipeline, err := os.ReadFile("rpm.go")
	require.NoError(t, err)
	require.Contains(t, string(buildPipeline), `"EXTRA_GOFLAGS=-buildvcs=false"`)
	require.Contains(t, string(buildPipeline), `TAGS='bindata timetzdata sqlite sqlite_unlock_notify pam' make backend`)
}

func TestForgejoTailnetPackagingContract(t *testing.T) {
	forgejoRoot := filepath.Join("..", "..", "..", "packaging", "rpm", "forgejo")
	spec, err := os.ReadFile(filepath.Join(forgejoRoot, "soda-forgejo.spec"))
	require.NoError(t, err)
	require.Contains(t, string(spec), "tailscale")
	require.Contains(t, string(spec), "%{_libexecdir}/soda/forgejo-tailnet")

	initialization, err := os.ReadFile(filepath.Join(forgejoRoot, "sources", "forgejo-init"))
	require.NoError(t, err)
	for _, expected := range []string{
		"/usr/libexec/soda/forgejo-tailnet", "address=127.0.0.1", "port=30000", "root_url=http://127.0.0.1:${port}/",
		"HTTP_ADDR = ${address}", "HTTP_PORT = ${port}", "DOMAIN = ${identity}", "root_url=http://${identity}:${port}/", "ROOT_URL = ${root_url}",
	} {
		require.Contains(t, string(initialization), expected)
	}
	require.NotContains(t, string(initialization), "tailscale serve")

	initUnit, err := os.ReadFile(filepath.Join(forgejoRoot, "sources", "systemd", "forgejo-init.service"))
	require.NoError(t, err)
	require.Contains(t, string(initUnit), "Wants=tailscaled.service")
	require.NotContains(t, string(initUnit), "Wants=soda-tailscale-enroll.service")
	require.Contains(t, string(initUnit), "After=systemd-sysusers.service systemd-tmpfiles-setup.service tailscaled.service soda-tailscale-enroll.service")
}

func TestRuntimeImageSystemdMountAndLoggingContract(t *testing.T) {
	runtimeSources := filepath.Join("..", "..", "..", "packaging", "rpm", "runtime", "sources")
	preset, err := os.ReadFile(filepath.Join(runtimeSources, "systemd", "90-soda.preset"))
	require.NoError(t, err)
	for _, unit := range []string{"sshd.service", "sodad.service", "soda-tailscale-enroll.service", "soda-authd.service", "soda-cockpit.service", "avahi-daemon.service", "tailscaled.service", "nftables.service", "var-srv-soda-projects.mount", "opt-soda-toolchains.mount"} {
		require.True(t, strings.Contains(string(preset), "enable "+unit))
	}
	require.Contains(t, string(preset), "enable forgejo.service")

	enrollment, err := os.ReadFile(filepath.Join(runtimeSources, "systemd", "soda-tailscale-enroll.service"))
	require.NoError(t, err)
	require.Contains(t, string(enrollment), "--auth-key=file:/var/lib/soda-install/tailscale-auth-key")
	require.Contains(t, string(enrollment), "ExecStopPost=-/usr/bin/unlink /var/lib/soda-install/tailscale-auth-key")
	require.NotContains(t, string(enrollment), "Restart=")

	nftRules, err := os.ReadFile(filepath.Join(runtimeSources, "nftables", "soda-ingress.nft"))
	require.NoError(t, err)
	require.Contains(t, string(nftRules), `iifname { "lo", "tailscale0" } tcp dport { 22, 9090, 30000 } accept`)
	require.Contains(t, string(nftRules), `tcp dport { 22, 9090, 30000 } reject with tcp reset`)
	require.Contains(t, string(nftRules), "policy accept")

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
	require.Contains(t, string(sodadUnit), "Requires=var-srv-soda-projects.mount opt-soda-toolchains.mount\n")
	require.Contains(t, string(sodadUnit), "After=local-fs.target network-online.target var-srv-soda-projects.mount opt-soda-toolchains.mount forgejo.service")
	require.Contains(t, string(sodadUnit), "Wants=network-online.target forgejo.service")
	require.NotContains(t, string(sodadUnit), "Requires=var-srv-soda-projects.mount opt-soda-toolchains.mount forgejo.service")

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
