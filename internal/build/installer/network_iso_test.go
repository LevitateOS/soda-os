package installer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/config"
	"github.com/stretchr/testify/require"
)

func TestBuildDoesNotDeleteRuntimeArchiveFromOutputDirectory(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, ".artifacts", "images")
	require.NoError(t, os.MkdirAll(output, 0o755))
	archive, digest := writeTestOCIArchiveAt(t, filepath.Join(output, "runtime.oci.tar"))
	lock := filepath.Join(root, "image-builder.lock")
	require.NoError(t, os.WriteFile(lock, []byte(`version = "81.0.0"
commit = "3130fb87ee1f684b6e9d1909f354861c43d7a092"
reference = "ghcr.io/osbuild/image-builder@sha256:704dc05d6033799248a33c415f7f7253ec20b40f0b2bff03b06d8687179e058a"
platform = "linux/arm64"
`), 0o644))
	exactReference := Repository + "@" + digest
	options := Options{ArchivePath: archive, ToolLock: lock, OutputDir: output}
	runner := &recordingRunner{}
	packageLock := filepath.Join(root, "installer-packages.toml")
	require.NoError(t, os.WriteFile(packageLock, []byte("schema_version = 1\nplatform = \"linux/arm64\"\npackages = [\"anaconda\"]\nboot_packages = [\"shim-aa64\"]\nefi_vendor = \"fedora\"\n"), 0o644))
	isoConfig := filepath.Join(root, "iso.yaml")
	require.NoError(t, os.WriteFile(isoConfig, []byte("test ISO config\n"), 0o644))
	platform := config.PlatformSpec{
		Architecture: config.PlatformArchitecture{Name: "aarch64", OCI: "arm64", Platform: "linux/arm64", Artifact: "aarch64", Installer: "aarch64"},
		Base:         config.PlatformBase{Reference: "quay.io/fedora/fedora-bootc@sha256:950a52fa1244db4d7fe2673af57fd6784a605a83bec3cd2d716ed8c00ebd366d", Archive: "unused.oci.tar", ArchiveSHA256: strings.Repeat("a", 64)},
		Builder:      config.PlatformBuilder{BaseReference: "registry.fedoraproject.org/fedora@sha256:9c8b291e256262b91aac5b3da50ea323760d0a6b449c6d6ad5f01d9550d48d2a"},
		Installer:    config.PlatformInstaller{PackageLock: packageLock, ISOConfig: isoConfig},
	}
	builder := NewBuilder(root, config.DistroSpec{Identity: config.IdentitySpec{Architecture: "aarch64", Hostname: "soda", Version: "0.2.0"}, Base: config.BaseSpec{Reference: platform.Base.Reference, Platform: platform.Architecture.Platform}, Platform: platform}, runner)
	builder.hostArchitecture = "arm64"
	_, err := builder.Build(context.Background(), options)
	require.ErrorContains(t, err, "image-builder did not create")
	require.FileExists(t, archive)
	volumeName := fmt.Sprintf("soda-installer-%s-%d", strings.TrimPrefix(exactReference, Repository+"@sha256:")[:12], os.Getpid())
	commands := commandOutput(runner)
	require.Contains(t, commands, "docker volume create "+volumeName)
	require.Contains(t, commands, "docker volume rm --force "+volumeName)
	require.NotContains(t, commands, "containers-storage:"+exactReference)
	require.Contains(t, commands, "--build-context installer-base=docker-image://registry.fedoraproject.org/fedora@sha256:9c8b291e256262b91aac5b3da50ea323760d0a6b449c6d6ad5f01d9550d48d2a")
	require.NotContains(t, commands, "--bootc-installer-payload-ref")
	require.NotContains(t, commands, "--bootc-pull-container")
	require.NotContains(t, commands, root+"/.artifacts/installer/containers-storage:/var/lib/containers/storage")
	require.Contains(t, commands, volumeName+":/var/lib/containers/storage")
	require.Contains(t, commands, "--tmpdir /var/lib/containers/storage copy")
}
