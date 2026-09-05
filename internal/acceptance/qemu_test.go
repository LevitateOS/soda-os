package acceptance

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQEMUInstallUsesOnlyCandidateISOAndTargetDisk(t *testing.T) {
	config := VMConfig{
		Architecture: "aarch64", Mode: "install", Disk: "/tmp/installed.qcow2",
		ISO: "/tmp/candidate.iso", Directory: "/tmp/evidence", Host: "127.0.0.1",
		SSHPort: 2222, CockpitPort: 19090, ForgejoPort: 13000,
	}
	arguments := strings.Join(qemuCommonArgs(config), " ")
	require.Contains(t, arguments, "file=/tmp/candidate.iso,media=cdrom,format=raw,readonly=on,if=virtio")
	require.Equal(t, 2, strings.Count(arguments, "-drive"))
	require.NotContains(t, arguments, "OEMDRV")
	require.NotContains(t, arguments, "cidata")
	require.NotContains(t, arguments, "config-2")
	require.Contains(t, arguments, "hostfwd=tcp:127.0.0.1:13000-:30000")
}

func TestGuestChecksDoNotRestoreRejectedOrchestration(t *testing.T) {
	checks := coreGuestChecks + qcow2GuestChecks + workspaceBoundaryChecks + stableManifestScript
	require.NotContains(t, checks, "toolset-commands")
	require.NotContains(t, checks, "tea-token")
	require.NotContains(t, checks, "configdrive")
	require.NotContains(t, checks, "nocloud")
	// Manual Tea and gh authentication is checked as an absence of retained config.
	require.Contains(t, checks, ".config/tea/config.yml")
	require.Contains(t, checks, ".config/gh/hosts.yml")
}

func TestInstalledChecksUseNativeTailscaleState(t *testing.T) {
	require.Contains(t, tailscaleAccessCheck, "tailscale status --json")
	require.NotContains(t, tailscaleAccessCheck, "soda-setup")
	require.NotContains(t, qcow2GuestChecks, "soda-setup")
	require.NotContains(t, localAccessCheck, "connection.zone")
}

func TestReusableQCOW2KeepsTheInteractiveConsoleVisible(t *testing.T) {
	arguments := strings.Join(qemuDisplayArgsFor(VMConfig{Architecture: "x86_64", Mode: "qcow2"}, "linux"), " ")
	require.Contains(t, arguments, "-display gtk")
	require.Equal(t, []string{"-display", "none"}, qemuDisplayArgs(VMConfig{Architecture: "x86_64", Mode: "installed"}))
}

func TestLinuxAArch64GraphicalFlowHasKeyboardAndPointer(t *testing.T) {
	arguments := qemuDisplayArgsFor(VMConfig{Architecture: "aarch64", Mode: "install"}, "linux")
	require.Contains(t, arguments, "qemu-xhci")
	require.Contains(t, arguments, "usb-kbd")
	require.Contains(t, arguments, "usb-tablet")
}

func TestDarwinAArch64GraphicalFlowUsesCocoaAndShortQMPSocket(t *testing.T) {
	config := VMConfig{Architecture: "aarch64", Mode: "install", Directory: "/a/very/long/acceptance/evidence/path/that/must/not/become/a/darwin/unix/socket/path"}
	arguments := qemuDisplayArgsFor(config, "darwin")
	require.Contains(t, arguments, "cocoa")
	require.NotContains(t, arguments, "gtk")
	require.Less(t, len(qmpSocketFor(config, "darwin")), 104)
}
