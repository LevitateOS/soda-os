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
		SSHPort: 2222, CockpitPort: 19090,
	}
	arguments := strings.Join(qemuCommonArgs(config), " ")
	require.Contains(t, arguments, "file=/tmp/candidate.iso,media=cdrom,format=raw,readonly=on,if=virtio")
	require.Equal(t, 2, strings.Count(arguments, "-drive"))
	require.NotContains(t, arguments, "OEMDRV")
	require.NotContains(t, arguments, "cidata")
	require.NotContains(t, arguments, "config-2")
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

func TestReusableQCOW2KeepsTheInteractiveConsoleVisible(t *testing.T) {
	arguments := strings.Join(qemuDisplayArgs(VMConfig{Architecture: "x86_64", Mode: "qcow2"}), " ")
	require.Contains(t, arguments, "-display gtk")
	require.Equal(t, []string{"-display", "none"}, qemuDisplayArgs(VMConfig{Architecture: "x86_64", Mode: "installed"}))
}
