package image

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTailscalePageShipsThroughRuntimeWithoutSetup(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	require.NoError(t, err)
	build, sources := t.TempDir(), t.TempDir()
	for _, name := range []string{"soda-projects", "soda-workspace-helper", "soda-runners", "soda-runner-helper", "soda-runner-launch", "soda-tailnet", "soda-forgejo-tailnet", "forgejo"} {
		require.NoError(t, os.WriteFile(filepath.Join(build, name), []byte(name), 0o755))
	}
	require.NoError(t, (&Builder{Root: root}).stageProductRPMSources(build, sources))
	spec, err := os.ReadFile(filepath.Join(root, "packaging/rpm/runtime/soda-runtime.spec"))
	require.NoError(t, err)
	for _, name := range []string{"manifest.json", "index.html", "app.css", "app.mjs", "native.mjs", "status.mjs", "stream.mjs"} {
		staged, readErr := os.ReadFile(filepath.Join(sources, "soda-tailscale-"+name))
		require.NoError(t, readErr)
		original, readErr := os.ReadFile(filepath.Join(root, "cockpit/soda-tailscale", name))
		require.NoError(t, readErr)
		require.Equal(t, original, staged)
		require.Contains(t, string(spec), "%{buildroot}%{_datadir}/cockpit/soda-tailscale/"+name)
	}
	for _, name := range []string{"soda-setup", "soda-local-access", "soda-tailnet.xml", "soda-projects-setup.mjs", "soda-projects-setup-protocol.mjs"} {
		_, statErr := os.Stat(filepath.Join(sources, name))
		require.ErrorIs(t, statErr, os.ErrNotExist)
		require.NotContains(t, string(spec), name)
	}
	require.Contains(t, specRequires(string(spec)), "cockpit-system")
	require.Contains(t, specRequires(string(spec)), "firewalld")
	require.Contains(t, string(spec), "%config(noreplace) %{_sysconfdir}/profile.d/soda-console-welcome.sh")
}

func TestNativeFirewallRemainsAvailableWithoutDefaultEnablement(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	container, err := os.ReadFile(filepath.Join(root, "packaging/bootc/Containerfile"))
	require.NoError(t, err)
	require.Contains(t, string(container), "systemctl disable firewalld.service")
	require.NotContains(t, string(container), "set-default-zone")
	require.NotContains(t, string(container), "mask firewalld")
	for _, line := range strings.Split(string(container), "\n") {
		if strings.Contains(line, "systemctl enable ") {
			require.NotContains(t, line, "firewalld")
		}
	}
	preset, err := os.ReadFile(filepath.Join(root, "packaging/rpm/runtime/sources/systemd/90-soda.preset"))
	require.NoError(t, err)
	require.Contains(t, string(preset), "disable firewalld.service")
	forwarding, err := os.ReadFile(filepath.Join(root, "packaging/rpm/runtime/sources/sysctl/60-soda-tailscale.conf"))
	require.NoError(t, err)
	require.Contains(t, string(forwarding), "net.ipv4.ip_forward = 1")
	require.Contains(t, string(forwarding), "net.ipv6.conf.all.forwarding = 1")
}
