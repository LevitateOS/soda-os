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

func TestNativeRunnerBinariesAndSourcesAreBuilt(t *testing.T) {
	runner := &recordingRunner{}
	builder := &Builder{Root: "/workspace/soda", runner: runner, Spec: config.DistroSpec{
		Identity: config.IdentitySpec{Version: "0.5.0"},
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
	for _, expected := range []string{
		"-o /src/.artifacts/build/soda-runners ./cmd/soda-runners",
		"-o /src/.artifacts/build/soda-runner-helper ./cmd/soda-runner-helper",
		"-o /src/.artifacts/build/soda-runner-launch ./cmd/soda-runner-launch",
	} {
		require.Contains(t, joined, expected)
	}
}

func TestRunnerPackageOwnsOnlyFocusedLocalComposition(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	contents, err := os.ReadFile(filepath.Join(root, "packaging", "rpm", "runners", "soda-runners.spec"))
	require.NoError(t, err)
	spec := string(contents)
	for _, expected := range []string{
		"%{_libexecdir}/soda/soda-runners",
		"%{_libexecdir}/soda/soda-runner-helper",
		"%{_libexecdir}/soda/soda-runner-launch",
		"%{_datadir}/cockpit/soda-runners/",
		"%{_unitdir}/soda-runner@.service",
		"%{_prefix}/lib/soda/github-actions-runner/",
	} {
		require.Contains(t, spec, expected)
	}
	for _, absent := range []string{"project_id", "workflow", "database", "queue", "scheduler", "external runner"} {
		require.NotContains(t, strings.ToLower(spec), absent)
	}
	dependencies := specRequires(spec)
	for _, dependency := range []string{"forgejo-runner", "libicu", "policycoreutils", "polkit", "shadow-utils", "systemd"} {
		require.Contains(t, dependencies, dependency)
	}
}

func TestRunnerServiceUsesDedicatedUnprivilegedHardenedAccount(t *testing.T) {
	path := filepath.Join("..", "..", "..", "packaging", "rpm", "runners", "sources", "systemd", "soda-runner@.service")
	contents, err := os.ReadFile(path)
	require.NoError(t, err)
	text := string(contents)
	for _, line := range []string{
		"User=soda-runner-%i", "Group=soda-runners", "NoNewPrivileges=true", "CapabilityBoundingSet=",
		"ProtectHome=true", "ProtectSystem=strict", "PrivateTmp=true",
		"ReadWritePaths=/var/lib/soda/runners/%i/state", "RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6",
	} {
		require.Contains(t, text, line)
	}
	require.NotContains(t, text, "sudo")
	require.NotContains(t, text, "/home/")
}

func TestRunnerPolkitPolicyBindsOnlyTheFixedHelper(t *testing.T) {
	path := filepath.Join("..", "..", "..", "packaging", "rpm", "runners", "sources", "polkit", "org.sodaos.runners.policy")
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
	require.Equal(t, "org.sodaos.runners.manage", policy.Actions[0].ID)
	require.Equal(t, "/usr/libexec/soda/soda-runner-helper", policy.Actions[0].Annotations[0].Value)
	require.NotContains(t, string(contents), "/bin/sh")
	require.NotContains(t, string(contents), "auth_admin_keep")
}

func TestSiblingRuntimeLocksContainNativeRunnerInputs(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	require.NoError(t, err)
	for _, architecture := range []string{"aarch64", "x86_64"} {
		t.Run(architecture, func(t *testing.T) {
			builder, buildErr := NewBuilder(root, filepath.Join(root, "distro", "soda.toml"), architecture, nil)
			require.NoError(t, buildErr)
			lock, lockErr := builder.packageLock()
			require.NoError(t, lockErr)
			packages := map[string]lockedPackage{}
			for _, item := range lock.Package {
				packages[item.Name] = item
			}
			require.Equal(t, "forgejo-runner-0:12.13.2-1.fc44."+architecture, packages["forgejo-runner"].NEVRA)
			require.Equal(t, "libicu-0:77.1-3.fc44."+architecture, packages["libicu"].NEVRA)
			require.Equal(t, "local-rpm", packages["soda-runners"].Source)
		})
	}
}
