package image

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConsoleWelcomeDisplaysNativeLANAndCurrentUser(t *testing.T) {
	tools := t.TempDir()
	writeWelcomeTestCommand(t, tools, "hostnamectl", "echo atlas\n")
	writeWelcomeTestCommand(t, tools, "id", "echo alice\n")
	writeWelcomeTestCommand(t, tools, "nmcli", "printf 'enp1s0:ethernet:connected\\nwlan0:wifi:connected\\ntailscale0:tun:connected\\n'\n")
	writeWelcomeTestCommand(t, tools, "ip", `case "$*" in
		*route*) echo 'default via 192.168.1.1 dev enp1s0';;
		*enp1s0*) echo '2: enp1s0 inet 192.168.1.10/24 scope global enp1s0';;
		*wlan0*) echo '3: wlan0 inet 10.0.0.2/24 scope global wlan0';;
		*) echo '4: tailscale0 inet 100.64.0.1/32 scope global tailscale0';;
	esac
`)
	writeWelcomeTestCommand(t, tools, "soda-tailnet", "echo 'Tailscale: disconnected.'\n")
	command := exec.Command("sh", filepath.Join("..", "..", "..", "packaging", "rpm", "runtime", "sources", "console", "soda-console-welcome"))
	command.Env = append(os.Environ(), "PATH="+tools+":"+os.Getenv("PATH"))
	output, err := command.Output()
	require.NoError(t, err)
	for _, expected := range []string{"Welcome to Soda OS", "Hostname: atlas", "Cockpit: https://192.168.1.10:9090", "Forgejo: http://192.168.1.10:30000/", "ssh alice@192.168.1.10", "https://10.0.0.2:9090", "Firewall: disabled by default.", "Cockpit: Networking > Firewall.", "Tailscale: disconnected.", "/etc/profile.d/soda-console-welcome.sh"} {
		require.Contains(t, string(output), expected)
	}
	require.NotContains(t, string(output), "100.64.0.1")
	require.NotContains(t, string(output), "Setup")
}

func TestWelcomeSurvivesMissingNetworkAndTailnetTools(t *testing.T) {
	tools := t.TempDir()
	writeWelcomeTestCommand(t, tools, "hostnamectl", "echo atlas\n")
	writeWelcomeTestCommand(t, tools, "id", "echo alice\n")
	for _, name := range []string{"nmcli", "ip", "soda-tailnet"} {
		writeWelcomeTestCommand(t, tools, name, "exit 1\n")
	}
	command := exec.Command("sh", filepath.Join("..", "..", "..", "packaging", "rpm", "runtime", "sources", "console", "soda-console-welcome"))
	command.Env = append(os.Environ(), "PATH="+tools+":"+os.Getenv("PATH"))
	output, err := command.Output()
	require.NoError(t, err)
	for _, expected := range []string{"Welcome to Soda OS", "Hostname: atlas", "No local IPv4 address", "ssh alice@atlas", "Firewall: disabled by default.", "Cockpit: Networking > Firewall.", "/etc/profile.d/soda-console-welcome.sh"} {
		require.Contains(t, string(output), expected)
	}
}

func TestConsoleWelcomeProfileRunsForInteractiveShellsOnly(t *testing.T) {
	profile, err := os.ReadFile(filepath.Join("..", "..", "..", "packaging", "rpm", "runtime", "sources", "profile.d", "soda-console-welcome.sh"))
	require.NoError(t, err)
	helper := filepath.Join(t.TempDir(), "soda-console-welcome")
	require.NoError(t, os.WriteFile(helper, []byte("#!/bin/sh\nprintf 'welcome\\n'\n"), 0o755))
	profilePath := filepath.Join(t.TempDir(), "soda-console-welcome.sh")
	require.NoError(t, os.WriteFile(profilePath, []byte(strings.Replace(string(profile), "/usr/libexec/soda/soda-console-welcome", helper, 1)), 0o644))

	for name, test := range map[string]struct {
		interactive bool
		environment []string
		want        string
	}{
		"interactive local shell":     {interactive: true, want: "welcome\n"},
		"interactive SSH shell":       {interactive: true, environment: []string{"SSH_CONNECTION=198.51.100.10 50000 10.0.2.15 22"}, want: "welcome\n"},
		"non-interactive SSH command": {environment: []string{"SSH_CONNECTION=198.51.100.10 50000 10.0.2.15 22", "SSH_ORIGINAL_COMMAND=printf status"}},
		"forced command":              {environment: []string{"SSH_ORIGINAL_COMMAND=internal-sftp"}},
		"automation":                  {},
	} {
		t.Run(name, func(t *testing.T) {
			args := []string{"--noprofile", "--norc"}
			if test.interactive {
				args = append(args, "-ic")
			} else {
				args = append(args, "-c")
			}
			args = append(args, `. "$1"`, "--", profilePath)
			command := exec.Command("bash", args...)
			command.Env = append(os.Environ(), test.environment...)
			output, err := command.Output()
			require.NoError(t, err)
			require.Equal(t, test.want, string(output))
		})
	}
}

func writeWelcomeTestCommand(t *testing.T, directory, name, body string) {
	t.Helper()
	path := filepath.Join(directory, name)
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755))
}

func TestConsoleWelcomeRPMContract(t *testing.T) {
	runtimeSpec, err := os.ReadFile(filepath.Join("..", "..", "..", "packaging", "rpm", "runtime", "soda-runtime.spec"))
	require.NoError(t, err)
	require.Contains(t, string(runtimeSpec), "install -m 0755 %{_sourcedir}/soda-console-welcome %{buildroot}%{_libexecdir}/soda/soda-console-welcome")
	require.Contains(t, string(runtimeSpec), "install -m 0755 %{_sourcedir}/soda-tailnet %{buildroot}%{_bindir}/soda-tailnet")
	require.Contains(t, string(runtimeSpec), "install -m 0644 %{_sourcedir}/soda-console-welcome.sh %{buildroot}%{_sysconfdir}/profile.d/soda-console-welcome.sh")

	staging, err := os.ReadFile("rpm.go")
	require.NoError(t, err)
	require.Contains(t, string(staging), `b.path("packaging/rpm/runtime/sources/console/soda-console-welcome"), filepath.Join(sources, "soda-console-welcome")`)
	require.Contains(t, string(staging), `{"soda-tailnet", "./cmd/soda-tailnet"}`)
	require.Contains(t, string(staging), `{filepath.Join(build, "soda-tailnet"), filepath.Join(sources, "soda-tailnet")}`)
	require.Contains(t, string(staging), `b.path("packaging/rpm/runtime/sources/profile.d/soda-console-welcome.sh"), filepath.Join(sources, "soda-console-welcome.sh")`)
	require.NotContains(t, strings.TrimSpace(string(runtimeSpec)), "%post")
}

func TestConsolePromptIsNotOverwrittenByRoutineKernelNotices(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	policy, err := os.ReadFile(filepath.Join(root, "packaging", "rpm", "runtime", "sources", "sysctl", "60-soda-console.conf"))
	require.NoError(t, err)
	require.Contains(t, string(policy), "kernel.printk = 4 4 1 7")

	runtimeSpec, err := os.ReadFile(filepath.Join(root, "packaging", "rpm", "runtime", "soda-runtime.spec"))
	require.NoError(t, err)
	require.Contains(t, string(runtimeSpec), "install -m 0644 %{_sourcedir}/60-soda-console.conf %{buildroot}%{_sysctldir}/60-soda-console.conf")
	require.Contains(t, string(runtimeSpec), "%{_sysctldir}/60-soda-console.conf")

	staging, err := os.ReadFile("rpm.go")
	require.NoError(t, err)
	require.Contains(t, string(staging), `b.path("packaging/rpm/runtime/sources/sysctl/60-soda-console.conf"), filepath.Join(sources, "60-soda-console.conf")`)
}

func TestConsoleLoginPromptClearsBeforeIssueRedraw(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	override, err := os.ReadFile(filepath.Join(root, "packaging", "rpm", "runtime", "sources", "systemd", "getty@tty1.service.d", "10-soda-console.conf"))
	require.NoError(t, err)
	require.Contains(t, string(override), "ExecStart=\n")
	require.Contains(t, string(override), "ExecStart=-/usr/sbin/agetty --noreset --issue-file=")
	require.NotContains(t, string(override), "--noclear")

	runtimeSpec, err := os.ReadFile(filepath.Join(root, "packaging", "rpm", "runtime", "soda-runtime.spec"))
	require.NoError(t, err)
	require.Contains(t, string(runtimeSpec), "install -m 0644 %{_sourcedir}/10-soda-console.conf %{buildroot}%{_unitdir}/getty@tty1.service.d/10-soda-console.conf")
	require.Contains(t, string(runtimeSpec), "%{_unitdir}/getty@tty1.service.d/10-soda-console.conf")

	staging, err := os.ReadFile("rpm.go")
	require.NoError(t, err)
	require.Contains(t, string(staging), `b.path("packaging/rpm/runtime/sources/systemd/getty@tty1.service.d/10-soda-console.conf"), filepath.Join(sources, "10-soda-console.conf")`)
}
