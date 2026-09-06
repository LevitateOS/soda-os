package oci

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/build/oci/ocitest"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/stretchr/testify/require"
)

func fixtureConfig(architecture string) *v1.ConfigFile {
	return &v1.ConfigFile{OS: "linux", Architecture: architecture, Config: v1.Config{Labels: map[string]string{
		"org.opencontainers.image.version":   "0.6.3",
		"org.opencontainers.image.revision":  strings.Repeat("a", 40),
		"org.opencontainers.image.base.name": "quay.io/fedora/fedora-bootc@sha256:" + strings.Repeat("b", 64),
	}}}
}

func TestInspectReturnsNativeIdentitiesForBothPlatforms(t *testing.T) {
	for _, architecture := range []string{"arm64", "amd64"} {
		img := ocitest.Image(t, fixtureConfig(architecture))
		archive := ocitest.Archive(t, img, architecture)
		facts, err := Inspect(archive, architecture)
		require.NoError(t, err)
		digest, err := img.Digest()
		require.NoError(t, err)
		configDigest, err := img.ConfigName()
		require.NoError(t, err)
		require.Equal(t, digest.String(), facts.Digest)
		require.Equal(t, configDigest.String(), facts.ConfigDigest)
		require.Equal(t, Repository+"@"+facts.Digest, facts.Reference())
		require.Equal(t, "linux/"+architecture, facts.Platform)
		require.Equal(t, "0.6.3", facts.Version)
		require.Equal(t, strings.Repeat("a", 40), facts.Revision)
		require.Len(t, facts.RPMInventorySHA256, 64)
	}
}

func TestInspectRejectsIdentityAndPlatformMismatch(t *testing.T) {
	for _, field := range []string{"version", "revision", "base.name", "architecture"} {
		config := fixtureConfig("amd64")
		if field == "architecture" {
			config.Architecture = "arm64"
		} else {
			config.Config.Labels["org.opencontainers.image."+field] = ""
		}
		_, err := Inspect(ocitest.Archive(t, ocitest.Image(t, config), "amd64"), "amd64")
		require.Error(t, err, field)
	}
	img := ocitest.Image(t, fixtureConfig("amd64"))
	_, err := Inspect(ocitest.Archive(t, img, "arm64"), "amd64")
	require.ErrorContains(t, err, "manifest must be linux/amd64")
	index := mutate.AppendManifests(empty.Index, mutate.IndexAddendum{Add: img}, mutate.IndexAddendum{Add: img})
	_, err = Inspect(ocitest.IndexArchive(t, index), "amd64")
	require.ErrorContains(t, err, "exactly one manifest")
}

func TestInspectRejectsCorruptBlobsAndSymlinks(t *testing.T) {
	archive := ocitest.Archive(t, ocitest.Image(t, fixtureConfig("amd64")), "amd64")
	link := filepath.Join(t.TempDir(), "link.tar")
	require.NoError(t, os.Symlink(archive, link))
	_, err := Inspect(link, "amd64")
	require.ErrorContains(t, err, "regular non-symlink")
	contents, err := os.ReadFile(archive)
	require.NoError(t, err)
	changed := bytes.ReplaceAll(contents, []byte("0.6.3"), []byte("9.9.9"))
	require.NotEqual(t, contents, changed)
	require.NoError(t, os.WriteFile(archive, changed, 0o600))
	_, err = Inspect(archive, "amd64")
	require.ErrorContains(t, err, "integrity")
}

func TestInspectRejectsUnsafeArchiveEntries(t *testing.T) {
	for _, header := range []*tar.Header{
		{Name: "../escape", Typeflag: tar.TypeReg},
		{Name: "/absolute", Typeflag: tar.TypeReg},
		{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "elsewhere"},
	} {
		var archive bytes.Buffer
		writer := tar.NewWriter(&archive)
		require.NoError(t, writer.WriteHeader(header))
		require.NoError(t, writer.Close())
		file := filepath.Join(t.TempDir(), "bad.tar")
		require.NoError(t, os.WriteFile(file, archive.Bytes(), 0o600))
		_, err := Inspect(file, "amd64")
		require.Error(t, err)
	}
}

func TestInventoryHonorsWhiteoutsAndRejectsSidecarMismatch(t *testing.T) {
	for _, entry := range []string{"usr/share/soda/.wh.rpm-inventory.txt", "usr/share/soda/rpm-inventory.sha256"} {
		var layer bytes.Buffer
		writer := tar.NewWriter(&layer)
		require.NoError(t, writer.WriteHeader(&tar.Header{Name: entry, Typeflag: tar.TypeReg}))
		require.NoError(t, writer.Close())
		extra, err := tarball.LayerFromReader(bytes.NewReader(layer.Bytes()))
		require.NoError(t, err)
		img, err := mutate.AppendLayers(ocitest.Image(t, fixtureConfig("amd64")), extra)
		require.NoError(t, err)
		_, err = Inspect(ocitest.Archive(t, img, "amd64"), "amd64")
		require.ErrorContains(t, err, "inventory")
	}
}
