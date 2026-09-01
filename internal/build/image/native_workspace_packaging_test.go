package image

import (
	"context"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/config"
	"github.com/stretchr/testify/require"
)

func TestNativeWorkspaceBinariesAreBuilt(t *testing.T) {
	runner := &recordingRunner{}
	builder := &Builder{Root: "/workspace/soda", runner: runner, Spec: config.DistroSpec{
		Identity: config.IdentitySpec{Version: "0.4.0"},
		Base:     config.BaseSpec{Platform: "linux/amd64"},
		Platform: config.PlatformSpec{Architecture: config.PlatformArchitecture{Artifact: "x86_64"}},
		Build:    config.BuildSpec{SourceDateEpoch: 1788220139},
	}}

	require.NoError(t, builder.buildGoBinaries(context.Background(), strings.Repeat("a", 40)))
	commands := make([]string, 0, len(runner.Commands))
	for _, command := range runner.Commands {
		commands = append(commands, command.String())
	}
	joined := strings.Join(commands, "\n")
	require.Contains(t, joined, "-o /src/.artifacts/build/soda-projects ./cmd/soda-projects")
	require.Contains(t, joined, "-o /src/.artifacts/build/soda-workspace-helper ./cmd/soda-workspace-helper")
}

func TestNativeWorkspaceCockpitPackagesAreLockedForX86_64(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	require.NoError(t, err)
	lock, err := (&Builder{Root: root, Spec: config.DistroSpec{
		Image: config.ImageSpec{PackageLock: "distro/locks/runtime-packages-x86_64.toml"},
	}}).packageLock()
	require.NoError(t, err)

	var cockpitPackages []lockedPackage
	for _, item := range lock.Package {
		if strings.HasPrefix(item.Name, "cockpit-") {
			cockpitPackages = append(cockpitPackages, item)
		}
	}
	require.Equal(t, []lockedPackage{
		{Name: "cockpit-bridge", NEVRA: "cockpit-bridge-0:366-1.fc44.noarch", Source: "fedora"},
		{Name: "cockpit-system", NEVRA: "cockpit-system-0:366-1.fc44.noarch", Source: "fedora"},
		{Name: "cockpit-ws", NEVRA: "cockpit-ws-0:366-1.fc44.x86_64", Source: "fedora"},
		{Name: "cockpit-ws-selinux", NEVRA: "cockpit-ws-selinux-0:366-1.fc44.x86_64", Source: "fedora"},
	}, cockpitPackages)
}

func TestStockCockpitLockClosureBlocksUnresolvedSiblingInputs(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	require.NoError(t, err)
	for _, test := range []struct {
		architecture string
		wantError    string
	}{
		{architecture: "x86_64"},
		{architecture: "aarch64", wantError: "runtime package lock requires matching-native resolution for: cockpit-bridge, cockpit-system, cockpit-ws, cockpit-ws-selinux"},
	} {
		t.Run(test.architecture, func(t *testing.T) {
			builder, buildErr := NewBuilder(root, filepath.Join(root, "distro", "soda.toml"), test.architecture, nil)
			require.NoError(t, buildErr)
			lock, lockErr := builder.packageLock()
			require.NoError(t, lockErr)
			if test.wantError == "" {
				require.NoError(t, validateStockCockpitLockClosure(lock))
				return
			}
			require.EqualError(t, validateStockCockpitLockClosure(lock), test.wantError)
		})
	}
}

func TestAArch64ArtifactBuildFailsBeforeDockerUntilCockpitLockIsResolved(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	require.NoError(t, err)
	runner := &recordingRunner{Outputs: map[string]string{
		"git status --porcelain=v1 --untracked-files=all": "",
		"git rev-parse HEAD":                              strings.Repeat("a", 40) + "\n",
	}}
	builder, err := NewBuilder(root, filepath.Join(root, "distro", "soda.toml"), "aarch64", runner)
	require.NoError(t, err)
	builder.hostArchitecture = "arm64"

	require.EqualError(t, builder.BuildRPMs(context.Background()), "runtime package lock requires matching-native resolution for: cockpit-bridge, cockpit-system, cockpit-ws, cockpit-ws-selinux")
	require.Equal(t, []string{
		"git status --porcelain=v1 --untracked-files=all",
		"git rev-parse HEAD",
	}, []string{runner.Commands[0].String(), runner.Commands[1].String()})
}

func TestNativeWorkspaceSourcesAreStagedForRPMBuild(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	require.NoError(t, err)
	build := t.TempDir()
	sources := t.TempDir()
	for _, name := range []string{
		"sodad", "sodactl", "soda-projects", "soda-workspace-helper",
		"soda-tailnet", "soda-forgejo-tailnet", "forgejo",
	} {
		require.NoError(t, os.WriteFile(filepath.Join(build, name), []byte(name), 0o755))
	}

	require.NoError(t, (&Builder{Root: root}).stageRPMSources(build, sources))
	for _, name := range []string{
		"soda-projects", "soda-workspace-helper", "soda-projects-manifest.json",
		"soda-projects-index.html", "soda-projects-app.mjs", "soda-projects-protocol.mjs",
		"soda-projects-ui.mjs", "soda-projects-app.css", "soda-projects-branding.css", "soda-projects-symbol.svg",
		"org.sodaos.projects.policy", "soda-projects.tmpfiles", "soda-projects.sysusers", "cockpit-stock.pam",
	} {
		info, statErr := os.Stat(filepath.Join(sources, name))
		require.NoError(t, statErr, name)
		require.False(t, info.IsDir(), name)
	}
}

func TestNativeWorkspacePolkitPolicyBindsOnlyTheFixedHelper(t *testing.T) {
	path := filepath.Join("..", "..", "..", "packaging", "rpm", "projects", "sources", "polkit", "org.sodaos.projects.policy")
	contents, err := os.ReadFile(path)
	require.NoError(t, err)
	var policy struct {
		Actions []struct {
			ID          string `xml:"id,attr"`
			Annotations []struct {
				Key   string `xml:"key,attr"`
				Value string `xml:",chardata"`
			} `xml:"annotate"`
		} `xml:"action"`
	}
	require.NoError(t, xml.Unmarshal(contents, &policy))
	require.Len(t, policy.Actions, 1)
	require.Equal(t, "org.sodaos.projects.manage", policy.Actions[0].ID)
	require.Equal(t, []struct {
		Key   string `xml:"key,attr"`
		Value string `xml:",chardata"`
	}{
		{Key: "org.freedesktop.policykit.exec.path", Value: "/usr/libexec/soda/soda-workspace-helper"},
		{Key: "org.freedesktop.policykit.exec.allow_gui", Value: "false"},
	}, policy.Actions[0].Annotations)
	require.NotContains(t, string(contents), "/bin/sh")
	require.NotContains(t, string(contents), "auth_admin_keep")
}

func TestStockCockpitPAMTemplatePreservesVendorStackAndRejectsWorkspaceAccounts(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join("..", "..", "..", "packaging", "rpm", "projects", "sources", "pam", "cockpit-stock"))
	require.NoError(t, err)
	lines := packagingNonCommentLines(string(contents))
	require.Equal(t, []string{
		"auth required pam_sepermit.so",
		"auth substack password-auth",
		"auth include postlogin",
		"auth optional pam_ssh_add.so",
		"account required pam_listfile.so item=user sense=deny file=/etc/cockpit/disallowed-users onerr=succeed",
		"account requisite pam_usertype.so isregular",
		"account requisite pam_succeed_if.so quiet user notingroup soda-workspaces",
		"account required pam_nologin.so",
		"account include password-auth",
		"password include password-auth",
		"session required pam_selinux.so close",
		"session required pam_loginuid.so",
		"session required pam_selinux.so open env_params",
		"session optional pam_keyinit.so force revoke",
		"session optional pam_ssh_add.so",
		"session include password-auth",
		"session include postlogin",
	}, lines)
}

func TestNativeWorkspaceRPMOwnsTheStockCockpitProjectsSurface(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	spec, err := os.ReadFile(filepath.Join(root, "packaging", "rpm", "projects", "soda-projects.spec"))
	require.NoError(t, err)
	text := string(spec)
	for _, expected := range []string{
		"%{_libexecdir}/soda/soda-projects",
		"%{_libexecdir}/soda/soda-workspace-helper",
		"%{_datadir}/polkit-1/actions/org.sodaos.projects.policy",
		"%{_tmpfilesdir}/soda-projects.conf",
		"%{_sysusersdir}/soda-projects.conf",
		"%{_prefix}/lib/soda/pam/cockpit",
		"%{_datadir}/cockpit/soda-projects/",
		"%{_datadir}/cockpit/branding/sodaos/",
	} {
		require.Contains(t, text, expected)
	}
	require.NotContains(t, text, "%{_sysconfdir}/pam.d/cockpit")
	require.NotContains(t, text, "%{_unitdir}/soda-cockpit.service")
	require.NotContains(t, text, "soda-authd")
	dependencies := specRequires(text)
	for _, name := range []string{
		"cockpit-system", "cockpit-ws", "coreutils", "git-core", "glibc-common", "openssh-clients", "policycoreutils", "polkit",
		"procps-ng", "shadow-utils", "systemd", "tailscale", "util-linux",
	} {
		require.Contains(t, dependencies, name)
	}
	require.NotContains(t, dependencies, "soda-runtime")
	require.NotContains(t, dependencies, "util-linux-core")

	tmpfiles, err := os.ReadFile(filepath.Join(root, "packaging", "rpm", "projects", "sources", "tmpfiles", "soda-projects.conf"))
	require.NoError(t, err)
	require.Equal(t, []string{
		"d /var/lib/soda 0755 root root -",
		"d /var/lib/soda/catalog 0755 root root -",
		`f /var/lib/soda/catalog/projects.json 0644 root root - []\n`,
		"d /run/lock/soda 0755 root root -",
		"f /run/lock/soda/workspace-operations.lock 0444 root root -",
	}, packagingNonCommentLines(string(tmpfiles)))

	runtimeTmpfiles, err := os.ReadFile(filepath.Join(root, "packaging", "rpm", "runtime", "sources", "tmpfiles", "soda.conf"))
	require.NoError(t, err)
	require.NotContains(t, string(runtimeTmpfiles), "d /var/lib/soda ")
	require.NotContains(t, string(runtimeTmpfiles), "/var/lib/soda/catalog")
	require.NotContains(t, string(runtimeTmpfiles), "/var/lib/soda/projects")
}

func specRequires(contents string) map[string]bool {
	dependencies := map[string]bool{}
	for line := range strings.Lines(contents) {
		if !strings.HasPrefix(line, "Requires:") {
			continue
		}
		for item := range strings.SplitSeq(strings.TrimPrefix(line, "Requires:"), ",") {
			fields := strings.Fields(item)
			if len(fields) > 0 {
				dependencies[fields[0]] = true
			}
		}
	}
	return dependencies
}

func packagingNonCommentLines(contents string) []string {
	var lines []string
	for line := range strings.Lines(contents) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lines = append(lines, strings.Join(strings.Fields(line), " "))
	}
	return lines
}
