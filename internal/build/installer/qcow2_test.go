package installer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/config"
	"github.com/stretchr/testify/require"
)

func TestBuildQCOW2UsesTheExactArchiveReferenceAndFixedCompression(t *testing.T) {
	root := t.TempDir()
	archive, digest := writeTestOCIArchiveAt(t, filepath.Join(root, "runtime.oci.tar"))
	lock := filepath.Join(root, "image-builder.lock")
	require.NoError(t, os.WriteFile(lock, []byte(`version = "81.0.0"
commit = "3130fb87ee1f684b6e9d1909f354861c43d7a092"
reference = "ghcr.io/osbuild/image-builder@sha256:704dc05d6033799248a33c415f7f7253ec20b40f0b2bff03b06d8687179e058a"
platform = "linux/arm64"
`), 0o644))
	output := filepath.Join(root, "output")
	runner := &recordingRunner{}
	builder := NewBuilder(root, config.DistroSpec{
		Identity: config.IdentitySpec{Architecture: "aarch64", Hostname: "soda", Version: "0.5.0"},
		Base:     config.BaseSpec{Platform: "linux/arm64"},
		Platform: config.PlatformSpec{Architecture: config.PlatformArchitecture{Name: "aarch64", OCI: "arm64", Platform: "linux/arm64", Artifact: "aarch64", Installer: "aarch64"}},
	}, runner)
	builder.hostArchitecture = "arm64"
	_, err := builder.BuildQCOW2(context.Background(), QCOW2Options{ArchivePath: archive, ToolLock: lock, OutputDir: output})
	require.ErrorContains(t, err, "image-builder did not create")
	commands := commandOutput(runner)
	exactReference := Repository + "@" + digest
	require.Contains(t, commands, "containers-storage:"+exactReference)
	require.Contains(t, commands, "--bootc-ref "+exactReference)
	require.Contains(t, commands, "--bootc-default-fs ext4")
	require.Contains(t, commands, "--output-name SodaOS-0.5.0-aarch64 qcow2")
	require.NotContains(t, commands, "--bootc-pull-container")
	require.NotContains(t, commands, "--bootc-installer-payload-ref")
}

func TestBuildQCOW2RejectsExistingOutputsBeforeCreatingStorage(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, "output")
	require.NoError(t, os.MkdirAll(output, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(output, "SodaOS-0.5.0-aarch64.qcow2"), []byte("existing"), 0o644))
	archive, _ := writeTestOCIArchiveAt(t, filepath.Join(root, "runtime.oci.tar"))
	lock := filepath.Join(root, "image-builder.lock")
	require.NoError(t, os.WriteFile(lock, []byte(`version = "81.0.0"
commit = "3130fb87ee1f684b6e9d1909f354861c43d7a092"
reference = "ghcr.io/osbuild/image-builder@sha256:704dc05d6033799248a33c415f7f7253ec20b40f0b2bff03b06d8687179e058a"
platform = "linux/arm64"
`), 0o644))
	runner := &recordingRunner{}
	builder := NewBuilder(root, config.DistroSpec{
		Identity: config.IdentitySpec{Architecture: "aarch64", Hostname: "soda", Version: "0.5.0"},
		Base:     config.BaseSpec{Platform: "linux/arm64"},
		Platform: config.PlatformSpec{Architecture: config.PlatformArchitecture{Name: "aarch64", OCI: "arm64", Platform: "linux/arm64", Artifact: "aarch64", Installer: "aarch64"}},
	}, runner)
	builder.hostArchitecture = "arm64"
	_, err := builder.BuildQCOW2(context.Background(), QCOW2Options{ArchivePath: archive, ToolLock: lock, OutputDir: output})
	require.EqualError(t, err, "QCOW2 output \""+filepath.Join(output, "SodaOS-0.5.0-aarch64.qcow2")+"\" already exists")
	require.Empty(t, runner.Commands)
}

func commandOutput(runner *recordingRunner) string {
	commands := make([]string, 0, len(runner.Commands))
	for _, command := range runner.Commands {
		commands = append(commands, command.String())
	}
	return strings.Join(commands, "\n")
}
