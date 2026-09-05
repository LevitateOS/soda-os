package scripts

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNativeCandidateRejectsUnverifiedBackend(t *testing.T) {
	for _, arch := range []string{"aarch64", "x86_64"} {
		for _, backend := range []string{"daemon", "daemon-os", "remote", "driver", "endpoint", "stopped", "multiple", "missing", "unnamed"} {
			t.Run(arch+"/"+backend, func(t *testing.T) {
				fixture := prepareNativeScriptTest(t, arch, "Linux")
				cmd := fixture.command(t, "prepare-native-iso-candidate.sh", arch)
				cmd.Env = append(cmd.Env, "SODA_TEST_DOCKER_CASE="+backend)
				runScriptFails(t, cmd, "check-native:")
				commands := readFile(t, fixture.log)
				for _, forbidden := range []string{"docker build --", "docker run", "just ", "skopeo "} {
					require.NotContains(t, commands, forbidden)
				}
			})
		}
	}
}

func TestNativeCandidateHonorsExplicitBackendSelection(t *testing.T) {
	for _, selection := range []string{"DOCKER_HOST=tcp://other:2375", "BUILDX_BUILDER=other"} {
		t.Run(selection, func(t *testing.T) {
			fixture := prepareNativeScriptTest(t, "aarch64", "Linux")
			cmd := fixture.command(t, "prepare-native-iso-candidate.sh", fixture.arch)
			cmd.Env = append(cmd.Env, selection)
			runScriptFails(t, cmd, "check-native:")
			if strings.Contains(readFile(t, fixture.log), "just ") {
				t.Fatal("unverified override reached artifact preparation")
			}
		})
	}
}

func TestNativeCandidateRejectsSiblingHardware(t *testing.T) {
	for _, host := range []string{"aarch64", "x86_64"} {
		fixture := prepareNativeScriptTest(t, host, "Linux")
		other := "aarch64"
		if host == other {
			other = "x86_64"
		}
		runScriptFails(t, fixture.command(t, "prepare-native-iso-candidate.sh", other), "matching native hardware")
		if strings.Contains(readFile(t, fixture.log), "docker ") {
			t.Fatal("wrong host contacted Docker")
		}
	}
}

func TestCheckNativeLinuxRunsCompleteGateDirectly(t *testing.T) {
	for _, arch := range []string{"aarch64", "x86_64"} {
		fixture := prepareNativeScriptTest(t, arch, "Linux")
		runScriptOK(t, fixture.command(t, "check-native.sh", arch))
		commands := readFile(t, fixture.log)
		if strings.Count(commands, "just check") != 1 || strings.Contains(commands, "docker ") {
			t.Fatalf("Linux gate should run unchanged without Docker:\n%s", commands)
		}
	}
}

func TestCheckNativeDarwinIsolation(t *testing.T) {
	fixture := prepareNativeScriptTest(t, "aarch64", "Darwin")
	writeStub(t, fixture.bin, "uname", `case "$1" in -m) echo arm64 ;; -s) echo Darwin ;; *) exit 2 ;; esac`)
	runScriptOK(t, fixture.command(t, "check-native.sh", fixture.arch))
	commands := readFile(t, fixture.log)
	for _, required := range []string{
		"--platform linux/arm64", "--build-arg GO_VERSION=1.26.7", "--file tools/check/Containerfile",
		"target=/source,readonly", "git clone --no-local --no-checkout", "git checkout --detach",
		"just check", "test \"$(uname -s)\" = Linux", testRevision,
	} {
		if !strings.Contains(commands, required) {
			t.Fatalf("missing check isolation %q:\n%s", required, commands)
		}
	}
	if strings.Count(commands, "--mount ") != 1 {
		t.Fatalf("expected only the read-only source bind mount:\n%s", commands)
	}
	for _, forbidden := range []string{"--privileged", "docker.sock", "node_modules", "git worktree", "go test ./internal/config"} {
		if strings.Contains(commands, forbidden) {
			t.Fatalf("unexpected check shortcut or host sharing %q:\n%s", forbidden, commands)
		}
	}
}

func TestCheckNativePropagatesFailures(t *testing.T) {
	for _, failure := range []string{"dirty", "check-image", "check-container"} {
		fixture := prepareNativeScriptTest(t, "aarch64", "Darwin")
		cmd := fixture.command(t, "check-native.sh", fixture.arch)
		cmd.Env = append(cmd.Env, "SODA_TEST_FAIL="+failure)
		runScriptFails(t, cmd, "")
	}
	for _, failure := range []string{"check", "drift-check", "status-after-check"} {
		fixture := prepareNativeScriptTest(t, "aarch64", "Linux")
		cmd := fixture.command(t, "check-native.sh", fixture.arch)
		cmd.Env = append(cmd.Env, "SODA_TEST_FAIL="+failure)
		runScriptFails(t, cmd, "")
	}
}

func TestCheckContainerUsesExistingToolchainInputs(t *testing.T) {
	contents := readFile(t, "../tools/check/Containerfile")
	for _, required := range []string{"ARG GO_VERSION", "golang:${GO_VERSION}", "scripts/install-cockpit-toolchain.sh", "cockpit/.node-version", "librsvg2-bin", "git jq just", "USER check", "--uid 1000", "/home/check/.local/share/vite-plus/bin", "vp env install"} {
		if !strings.Contains(contents, required) {
			t.Fatalf("check environment does not reuse %q", required)
		}
	}
}
