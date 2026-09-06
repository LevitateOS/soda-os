package scripts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrepareNativeImageSequenceAndActualPath(t *testing.T) {
	for _, arch := range []string{"aarch64", "x86_64"} {
		t.Run(arch, func(t *testing.T) {
			fixture := prepareNativeScriptTest(t, arch, "Linux")
			output := runScriptOK(t, fixture.command(t, "prepare-native-image.sh", arch))
			commands := readFile(t, fixture.log)
			requireOrder(t, commands, "docker info", "docker buildx ls", "just check", "just forgejo-source", "just github-runner "+arch,
				"go run ./cmd/soda-image --architecture "+arch+" oci --output-dir .artifacts/images/"+arch+"/"+testRevision,
				"skopeo inspect --raw", "docker load", "docker image inspect", "docker run", "go run ./cmd/soda-image --architecture "+arch+" publish --archive "+fixture.root+"/.artifacts/images/actual returned archive.oci.tar")
			require.Equal(t, 1, strings.Count(commands, "just check"))
			require.Equal(t, 1, strings.Count(commands, " oci --output-dir "))
			for _, forbidden := range []string{"origin/main", "git fetch", "just rpm", "just iso", "qcow2", "./cmd/soda-release", "record-sign", "git push"} {
				require.NotContains(t, commands, forbidden)
			}
			require.Contains(t, commands, "--entrypoint /bin/sh "+testImageID)
			require.Contains(t, output, "dev-"+arch)
			require.Contains(t, output, "No installer was built and no installed update was tested")
		})
	}
}

func TestPrepareNativeImageUsesContainerdManifestID(t *testing.T) {
	fixture := prepareNativeScriptTest(t, "aarch64", "Linux")
	cmd := fixture.command(t, "prepare-native-image.sh", fixture.arch)
	cmd.Env = append(cmd.Env, "SODA_TEST_IMAGE_STORE=containerd")
	runScriptOK(t, cmd)
	commands := readFile(t, fixture.log)
	require.Contains(t, commands, "--entrypoint /bin/sh "+testDigest)
	require.NotContains(t, commands, "--entrypoint /bin/sh ghcr.io/")
}

func TestPrepareNativeImageCanBeSourcedWithoutPublication(t *testing.T) {
	fixture := prepareNativeScriptTest(t, "aarch64", "Linux")
	writeTestFile(t, filepath.Join(fixture.root, "scripts", "source-only.sh"), "#!/usr/bin/env bash\nsource scripts/prepare-native-image.sh\n", 0o755)
	runScriptOK(t, fixture.command(t, "source-only.sh"))
	_, err := os.Stat(fixture.log)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestPrepareNativeImagePrepublicationFailures(t *testing.T) {
	for _, failure := range []string{"dirty", "git-status", "status-after-check", "check", "fetch", "oci", "missing-archive", "metadata", "version", "config-digest", "manifest-digest", "image-id", "runtime", "drift-check", "drift-oci"} {
		t.Run(failure, func(t *testing.T) {
			fixture := prepareNativeScriptTest(t, "aarch64", "Linux")
			cmd := fixture.command(t, "prepare-native-image.sh", fixture.arch)
			cmd.Env = append(cmd.Env, "SODA_TEST_FAIL="+failure)
			runScriptFails(t, cmd, "")
			require.NotContains(t, readFile(t, fixture.log), " publish --archive ")
		})
	}
}

func TestPrepareNativeImageReportsPartialPublicationWithoutRetry(t *testing.T) {
	fixture := prepareNativeScriptTest(t, "aarch64", "Linux")
	cmd := fixture.command(t, "prepare-native-image.sh", fixture.arch)
	cmd.Env = append(cmd.Env, "SODA_TEST_FAIL=publish")
	output := runScriptFails(t, cmd, "partially complete")
	require.Contains(t, output, "development tag may have advanced")
	require.NotContains(t, output, "Native OCI publication complete")
	require.Equal(t, 1, strings.Count(readFile(t, fixture.log), " publish --archive "))
}

func TestPrepareNativeImageDarwinUsesLinuxChecks(t *testing.T) {
	fixture := prepareNativeScriptTest(t, "aarch64", "Darwin")
	runScriptOK(t, fixture.command(t, "prepare-native-image.sh", fixture.arch))
	commands := readFile(t, fixture.log)
	requireOrder(t, commands, "docker build --platform linux/arm64", "git clone --no-local --no-checkout", " oci --output-dir ")
	require.NotContains(t, commands, "\njust check\n")
}
