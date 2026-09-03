package release

import (
	"archive/tar"
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/config"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/stretchr/testify/require"
)

type fakeISOValidator struct {
	calls int
	err   error
}

func (v *fakeISOValidator) ValidateISO(_ context.Context, isoPath, _ string, _ string, _ string) (string, error) {
	v.calls++
	if v.err != nil {
		return "", v.err
	}
	contents, err := os.ReadFile(isoPath)
	if err != nil {
		return "", err
	}
	return sha256Hex(contents), nil
}

func testSpec() config.DistroSpec {
	return config.DistroSpec{
		Identity: config.IdentitySpec{Version: "0.2.0"},
		Base:     config.BaseSpec{Reference: "quay.io/fedora/fedora-bootc@sha256:" + strings.Repeat("b", 64), Platform: "linux/arm64"},
		Image:    config.ImageSpec{Registry: Repository},
		Platform: config.PlatformSpec{
			Architecture: config.PlatformArchitecture{Name: "aarch64", OCI: "arm64", Platform: "linux/arm64", Artifact: "aarch64"},
			Base:         config.PlatformBase{RuntimePackageLock: "runtime.lock"},
			Release:      config.PlatformRelease{Channel: "aarch64"},
		},
	}
}

func matchingTestImage(t *testing.T) v1.Image {
	return testImageWithSidecar(t, sha256Hex([]byte("rpm inventory\n")))
}

func testImageWithSidecar(t *testing.T, sidecarDigest string) v1.Image {
	t.Helper()
	inventory := []byte("rpm inventory\n")
	var layer bytes.Buffer
	writer := tar.NewWriter(&layer)
	for name, contents := range map[string][]byte{
		"usr/share/soda/rpm-inventory.txt":    inventory,
		"usr/share/soda/rpm-inventory.sha256": []byte(sidecarDigest + "  rpm-inventory.txt\n"),
	} {
		require.NoError(t, writer.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(contents))}))
		_, err := writer.Write(contents)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	image, err := mutate.AppendLayers(empty.Image, static.NewLayer(layer.Bytes(), types.OCILayer))
	require.NoError(t, err)
	configFile, err := image.ConfigFile()
	require.NoError(t, err)
	configFile.OS = "linux"
	configFile.Architecture = "arm64"
	configFile.Config.Labels = map[string]string{
		"org.opencontainers.image.version":   "0.2.0",
		"org.opencontainers.image.revision":  testRevision,
		"org.opencontainers.image.base.name": testSpec().Base.Reference,
	}
	image, err = mutate.ConfigFile(image, configFile)
	require.NoError(t, err)
	return image
}

func writeOCIArchive(t *testing.T, image v1.Image) string {
	t.Helper()
	index := mutate.AppendManifests(empty.Index, mutate.IndexAddendum{Add: image, Descriptor: v1.Descriptor{Platform: &v1.Platform{OS: "linux", Architecture: "arm64"}}})
	return writeIndexArchive(t, index)
}

func writeIndexArchive(t *testing.T, index v1.ImageIndex) string {
	t.Helper()
	directory := filepath.Join(t.TempDir(), "layout")
	_, err := layout.Write(directory, index)
	require.NoError(t, err)
	archive := filepath.Join(t.TempDir(), "image.oci.tar")
	require.NoError(t, exec.Command("tar", "-cf", archive, "-C", directory, ".").Run())
	return archive
}
