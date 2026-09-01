package image

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConsoleWelcomeDisplaysHostnameAndIPv4Address(t *testing.T) {
	for name, test := range map[string]struct {
		ipOutput      string
		tailnetOutput string
		want          string
	}{
		"first global address": {
			ipOutput:      "2: enp1s0    inet 10.0.2.15/24 brd 10.0.2.255 scope global dynamic enp1s0\n3: enp2s0    inet 192.0.2.4/24 scope global enp2s0\n",
			tailnetOutput: "\nTailscale is connected.\nMagicDNS identity: atlas.example.ts.net\nOpen the Soda OS dashboard:\n  https://atlas.example.ts.net:9090\n",
			want:          "\nWelcome to Soda OS.\n\nTailscale is connected.\nMagicDNS identity: atlas.example.ts.net\nOpen the Soda OS dashboard:\n  https://atlas.example.ts.net:9090\n\nLocal console address:\n  https://10.0.2.15:9090\n",
		},
		"no global address": {
			tailnetOutput: "\nTailscale is not enrolled. Tailnet access is unavailable.\nInfrastructure owner: run `sudo tailscale up`, then open the one-time URL it prints to authorize this appliance. After authorization, run `sudo systemctl restart forgejo` to load the Tailnet Forgejo address. Soda does not store a Tailnet authorization key.\n",
			want:          "\nWelcome to Soda OS.\n\nTailscale is not enrolled. Tailnet access is unavailable.\nInfrastructure owner: run `sudo tailscale up`, then open the one-time URL it prints to authorize this appliance. After authorization, run `sudo systemctl restart forgejo` to load the Tailnet Forgejo address. Soda does not store a Tailnet authorization key.\n",
		},
	} {
		t.Run(name, func(t *testing.T) {
			tools := t.TempDir()
			writeWelcomeTestCommand(t, tools, "ip", "printf '%s' \"$SODA_TEST_IP_OUTPUT\"\n")
			writeWelcomeTestCommand(t, tools, "soda-tailnet", "printf '%s' \"$SODA_TEST_TAILNET_OUTPUT\"\n")

			command := exec.Command("sh", filepath.Join("..", "..", "..", "packaging", "rpm", "runtime", "sources", "console", "soda-console-welcome"))
			command.Env = append(os.Environ(), "PATH="+tools+":"+os.Getenv("PATH"), "SODA_TEST_IP_OUTPUT="+test.ipOutput, "SODA_TEST_TAILNET_OUTPUT="+test.tailnetOutput)
			output, err := command.Output()
			require.NoError(t, err)
			require.Equal(t, test.want, string(output))
		})
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
