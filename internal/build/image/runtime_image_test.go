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
		"ARG FEDORA_BASE_REFERENCE", "org.opencontainers.image.base.name=\"${FEDORA_BASE_REFERENCE}\"", "systemd-sysusers /usr/lib/sysusers.d/soda-projects.conf",
		"COPY --from=rpm-inputs /soda-release-*.rpm /var/tmp/soda-rpms/",
		"COPY --from=rpm-inputs /soda-runtime-*.rpm /var/tmp/soda-rpms/",
		"COPY --from=rpm-inputs /soda-projects-*.rpm /var/tmp/soda-rpms/",
		"COPY --from=rpm-inputs /soda-forgejo-*.rpm /var/tmp/soda-rpms/",
		"COPY --from=rpm-inputs /soda-bun-*.rpm /var/tmp/soda-rpms/",
		"COPY --from=rpm-inputs /soda-tea-*.rpm /var/tmp/soda-rpms/",
		"COPY --from=lock-inputs /fedora-packages.txt /var/tmp/soda-lock/fedora-packages.txt",
		"COPY --from=lock-inputs /expected-packages.txt /var/tmp/soda-lock/expected-packages.txt",
		"getent group soda-workspaces",
		"install -o root -g root -m 0644 /usr/lib/soda/pam/cockpit /etc/pam.d/cockpit",
		"systemctl enable sshd.service soda-tailscale-enroll.service cockpit.socket forgejo.service tailscaled.service nftables.service",
		"getent passwd git",
		"systemctl mask bootc-fetch-apply-updates.timer", "cp -f /usr/lib/soda/os-release /etc/os-release",
		"cp -f /usr/lib/soda/os-release /usr/lib/os-release", "cp -f /usr/lib/soda/issue /etc/issue",
		"cp -f /usr/lib/soda/issue /etc/issue.net", "cp -f /usr/lib/soda/system-release /etc/system-release",
		"rm -f /etc/redhat-release", "semanage fcontext -a -t var_lib_t '/var/lib/soda(/.*)?'",
		"semanage fcontext -a -t ssh_home_t '/var/lib/forgejo/.ssh(/.*)?'", "restorecon -RF /var/lib/forgejo/.ssh", "ssh-keygen -q -t ed25519 -N '' -f /run/soda-sshd-hostkey",
		"/usr/sbin/sshd -t -h /run/soda-sshd-hostkey", "rm -f /run/soda-sshd-hostkey /run/soda-sshd-hostkey.pub",
		"--enablerepo=updates-testing", `test "$(rpm -q --qf '%{NAME}-%{EPOCHNUM}:%{VERSION}-%{RELEASE}.%{ARCH}' bootc)" = "${BOOTC_NEVRA}"`,
		"bootc switch --help | grep -F -- '--download-only'", "bootc switch --help | grep -F -- '--from-downloaded'",
		"test -r /usr/share/soda/toolset-commands.txt", `while IFS= read -r command; do command -v "$command" >/dev/null; done < /usr/share/soda/toolset-commands.txt`,
		"rpm-inventory.sha256", "sha256sum --check rpm-inventory.sha256", "/usr/lib/sysimage/libdnf5/transaction_history.sqlite*",
		"/var/cache/ldconfig/aux-cache", "/var/cache/libdnf5", "/var/lib/dnf/repos", "/var/log/dnf5.log", "/run/dnf",
	} {
		require.Contains(t, containerfile, expected)
	}
	for _, obsolete := range []string{
		"rpm -q skopeo",
		".artifacts/bootc/distribution",
		"/usr/share/soda/release/distribution.json",
		"org.sodaos.state-schema",
		"/opt/soda/toolchains",
		"/var/lib/soda/toolchains",
		"opt-soda-toolchains.mount",
		"sodad", "sodactl", "soda-api", "/var/log/soda",
	} {
		require.NotContains(t, containerfile, obsolete)
	}
	require.NotContains(t, containerfile, "cp -f /usr/lib/soda/system-release /etc/redhat-release")
	require.NotContains(t, containerfile, "bootc-fetch-apply-updates.service")
	for _, obsolete := range []string{
		"soda-authd.service", "soda-cockpit.service", "avahi-daemon.service",
		"var-srv-soda-projects.mount", "/var/lib/soda/projects", "/var/srv/soda/projects",
		"/etc/soda/authorized_keys",
	} {
		require.NotContains(t, containerfile, obsolete)
	}
}

func TestRuntimeImageBuildContextContract(t *testing.T) {
	contents, err := os.ReadFile("builder.go")
	require.NoError(t, err)
	build := string(contents)
	require.Contains(t, build, `"--build-context", "rpm-inputs=" + b.artifactPath("rpms")`)
	require.Contains(t, build, `"--build-context", "lock-inputs=" + b.artifactPath("bootc")`)
	require.Contains(t, build, `"packaging/bootc",`)

	containerfile, err := os.ReadFile(filepath.Join("..", "..", "..", "packaging", "bootc", "Containerfile"))
	require.NoError(t, err)
	require.NotContains(t, string(containerfile), ".artifacts/")
}

func TestRepositoryBuildContextExcludesCredentialsAndUnrelatedArtifacts(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join("..", "..", "..", ".dockerignore"))
	require.NoError(t, err)
	ignore := string(contents)
	for _, excluded := range []string{
		".tailscale_auth_key", "**/*.key", "**/*.pem", "**/*.db", "**/*.qcow2", "**/*.oci-archive.tar",
		"distro/base/**/*.oci-archive.tar", ".artifacts/**", ".artifacts/builder/**", ".artifacts/installer/**", ".artifacts/installer/context/**",
	} {
		require.Contains(t, ignore, excluded+"\n")
	}
	for _, included := range []string{
		"!.artifacts/builder/packages.lock", "!.artifacts/builder/go.tar.gz",
		"!.artifacts/installer/context/installer-packages.txt", "!.artifacts/installer/context/installer-boot-packages.txt",
		"!.artifacts/installer/context/installer-efi-vendor.txt", "!.artifacts/installer/context/interactive-defaults.ks",
		"!.artifacts/installer/context/iso.yaml",
	} {
		require.Contains(t, ignore, included+"\n")
	}
}

func TestRuntimeImageStateDirectoriesAndSELinuxContract(t *testing.T) {
	runtimeRoot := filepath.Join("..", "..", "..", "packaging", "rpm", "runtime", "sources")
	_, err := os.Stat(filepath.Join(runtimeRoot, "sysusers", "soda.conf"))
	require.ErrorIs(t, err, os.ErrNotExist)
	_, err = os.Stat(filepath.Join(runtimeRoot, "tmpfiles", "soda.conf"))
	require.ErrorIs(t, err, os.ErrNotExist)

	projectSysusers, err := os.ReadFile(filepath.Join("..", "..", "..", "packaging", "rpm", "projects", "sources", "sysusers", "soda-projects.conf"))
	require.NoError(t, err)
	require.Equal(t, []string{"g soda-workspaces -"}, packagingNonCommentLines(string(projectSysusers)))

	projectTmpfiles, err := os.ReadFile(filepath.Join("..", "..", "..", "packaging", "rpm", "projects", "sources", "tmpfiles", "soda-projects.conf"))
	require.NoError(t, err)
	require.Contains(t, packagingNonCommentLines(string(projectTmpfiles)), "d /var/lib/soda 0755 root root -")
}

func TestRuntimeImageRPMStagingContract(t *testing.T) {
	staging, err := os.ReadFile("rpm.go")
	require.NoError(t, err)
	require.NotContains(t, string(staging), "soda-state-directories.service")
	require.NotContains(t, string(staging), "opt-soda-toolchains.mount")
	require.Contains(t, string(staging), `b.path("distro/toolset-commands.txt"), filepath.Join(sources, "toolset-commands.txt")`)
	require.Contains(t, string(staging), `b.path("packaging/rpm/runtime/sources/systemd/soda-tailscale-enroll.service"), filepath.Join(sources, "soda-tailscale-enroll.service")`)
	require.Contains(t, string(staging), `b.path("packaging/rpm/runtime/sources/nftables/soda-ingress.nft"), filepath.Join(sources, "soda-ingress.nft")`)
	require.Contains(t, string(staging), `b.path("packaging/rpm/runtime/sources/systemd/nftables.service.d/10-soda-ingress.conf"), filepath.Join(sources, "10-soda-ingress.conf")`)
	require.NotContains(t, string(staging), "soda-installer-import.service")
	require.NotContains(t, string(staging), "soda-installer-input")
	require.NotContains(t, string(staging), "soda-installer-finalize")
	require.NotContains(t, string(staging), "10-soda-state.conf")
	require.NotContains(t, string(staging), "var-srv-soda-projects.mount")
	require.NotContains(t, string(staging), "soda-ssh")
	require.NotContains(t, string(staging), "soda-cockpit.service")
	require.NotContains(t, string(staging), "soda-authd.service")
	require.Contains(t, string(staging), `b.path("packaging/rpm/forgejo/sources/systemd/forgejo.service"), filepath.Join(sources, "forgejo.service")`)
	require.Contains(t, string(staging), `filepath.Join(build, "soda-forgejo-tailnet"), filepath.Join(sources, "forgejo-tailnet")`)
	require.Contains(t, string(staging), `b.path("packaging/rpm/forgejo/sources/pam/soda-forgejo"), filepath.Join(sources, "soda-forgejo.pam")`)
	require.NotContains(t, string(staging), "00-soda-var-srv.conf")
}

func TestRuntimeHostCompositionRPMContract(t *testing.T) {
	runtimeSpec, err := os.ReadFile(filepath.Join("..", "..", "..", "packaging", "rpm", "runtime", "soda-runtime.spec"))
	require.NoError(t, err)
	require.NotContains(t, string(runtimeSpec), "soda-state-directories.service")
	require.NotContains(t, string(runtimeSpec), "opt-soda-toolchains.mount")
	require.Contains(t, string(runtimeSpec), "install -m 0644 %{_sourcedir}/soda-tailscale-enroll.service %{buildroot}%{_unitdir}/soda-tailscale-enroll.service")
	require.Contains(t, string(runtimeSpec), "%{_unitdir}/soda-tailscale-enroll.service")
	require.Contains(t, string(runtimeSpec), "tailscale")
	require.Contains(t, string(runtimeSpec), "nftables-services")
	require.Contains(t, string(runtimeSpec), "install -m 0644 %{_sourcedir}/soda-ingress.nft %{buildroot}%{_prefix}/lib/soda/network/soda-ingress.nft")
	require.Contains(t, string(runtimeSpec), "install -m 0644 %{_sourcedir}/10-soda-ingress.conf %{buildroot}%{_unitdir}/nftables.service.d/10-soda-ingress.conf")
	require.NotContains(t, string(runtimeSpec), "soda-installer-import.service")
	require.NotContains(t, string(runtimeSpec), "soda-installer-input")
	require.NotContains(t, string(runtimeSpec), "soda-installer-finalize")
	require.NotContains(t, string(runtimeSpec), "10-soda-state.conf")
	require.NotContains(t, string(runtimeSpec), "var-srv-soda-projects.mount")
	require.NotContains(t, string(runtimeSpec), "soda-ssh")
	require.NotContains(t, string(runtimeSpec), "soda-cockpit")
	require.NotContains(t, string(runtimeSpec), "soda-authd")
	require.Contains(t, string(runtimeSpec), "soda-forgejo = 15.0.7")
	require.Contains(t, string(runtimeSpec), "Soda OS host composition")
	for _, obsolete := range []string{"sodad", "sodactl", "soda-api", "/var/log/soda"} {
		require.NotContains(t, string(runtimeSpec), obsolete)
	}
	require.NotContains(t, string(runtimeSpec), "host telemetry")
	require.NotContains(t, string(runtimeSpec), "telemetry and update")
	require.NotContains(t, string(runtimeSpec), "update RPCs")
	require.NotContains(t, string(runtimeSpec), "00-soda-var-srv.conf")
}

func TestReleaseRPMContract(t *testing.T) {
	releaseSpec, err := os.ReadFile(filepath.Join("..", "..", "..", "packaging", "rpm", "release", "soda-release.spec"))
	require.NoError(t, err)
	require.Contains(t, string(releaseSpec), `%{_prefix}/lib/soda/os-release`)
	require.Contains(t, string(releaseSpec), `%{_datadir}/soda/toolset-commands.txt`)
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
	require.Contains(t, string(unit), "SupplementaryGroups=soda-forgejo-shadow")
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
	require.Contains(t, string(sysusers), "g soda-forgejo-shadow -")
	require.Contains(t, string(sysusers), "u git 975")

	tmpfiles, err := os.ReadFile(filepath.Join(forgejoRoot, "sources", "tmpfiles", "forgejo.conf"))
	require.NoError(t, err)
	require.Contains(t, string(tmpfiles), "z /etc/shadow 0040 root soda-forgejo-shadow - -")

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
	require.Contains(t, string(buildPipeline), `"GOCACHE=/src/.artifacts/build/forgejo-go-cache"`)
	require.Contains(t, string(buildPipeline), `"GOTMPDIR=/src/.artifacts/build/forgejo-go-tmp"`)
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
	require.Contains(t, string(initUnit), "ExecStartPre=-/usr/bin/tailscale wait --timeout=30s")
}

func TestRuntimeImageSystemdHostCompositionContract(t *testing.T) {
	runtimeSources := filepath.Join("..", "..", "..", "packaging", "rpm", "runtime", "sources")
	preset, err := os.ReadFile(filepath.Join(runtimeSources, "systemd", "90-soda.preset"))
	require.NoError(t, err)
	for _, unit := range []string{"sshd.service", "soda-tailscale-enroll.service", "forgejo.service", "cockpit.socket", "tailscaled.service", "nftables.service"} {
		require.True(t, strings.Contains(string(preset), "enable "+unit))
	}
	for _, obsolete := range []string{"soda-authd.service", "soda-cockpit.service", "avahi-daemon.service", "var-srv-soda-projects.mount"} {
		require.NotContains(t, string(preset), obsolete)
	}

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

	_, err = os.Stat(filepath.Join(runtimeSources, "systemd", "var-srv-soda-projects.mount"))
	require.ErrorIs(t, err, os.ErrNotExist)

	for _, obsolete := range []string{"soda-state-directories.service", "opt-soda-toolchains.mount"} {
		_, statErr := os.Stat(filepath.Join(runtimeSources, "systemd", obsolete))
		require.ErrorIs(t, statErr, os.ErrNotExist)
	}

	_, err = os.Stat(filepath.Join(runtimeSources, "systemd", "sodad.service"))
	require.ErrorIs(t, err, os.ErrNotExist)

	for _, obsolete := range []string{
		filepath.Join("..", "..", "..", "packaging", "rpm", "cockpit", "sources", "systemd", "soda-authd.service"),
		filepath.Join("..", "..", "..", "packaging", "rpm", "cockpit", "sources", "systemd", "soda-cockpit.service"),
	} {
		_, statErr := os.Stat(obsolete)
		require.ErrorIs(t, statErr, os.ErrNotExist)
	}
}
