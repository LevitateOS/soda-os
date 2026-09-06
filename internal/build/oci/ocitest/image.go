// Package ocitest makes tiny synthetic OCI fixtures for inspector, publisher,
// installer and acceptance source tests. It never builds or runs a native image.
package ocitest

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/stretchr/testify/require"
)

func Image(t *testing.T, config *v1.ConfigFile) v1.Image {
	t.Helper()
	inventory := []byte("soda-release-0:0.6.3-1.fc44.noarch\n")
	var layer bytes.Buffer
	writer := tar.NewWriter(&layer)
	for _, entry := range []struct {
		name     string
		contents []byte
	}{
		{"usr/share/soda/rpm-inventory.txt", inventory},
		{"usr/share/soda/rpm-inventory.sha256", fmt.Appendf(nil, "%x  rpm-inventory.txt\n", sha256.Sum256(inventory))},
	} {
		require.NoError(t, writer.WriteHeader(&tar.Header{Name: entry.name, Mode: 0o644, Size: int64(len(entry.contents))}))
		_, err := writer.Write(entry.contents)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	compressed, err := tarball.LayerFromReader(bytes.NewReader(layer.Bytes()), tarball.WithMediaType(types.OCILayer))
	require.NoError(t, err)
	img, err := mutate.AppendLayers(empty.Image, compressed)
	require.NoError(t, err)
	original, err := img.ConfigFile()
	require.NoError(t, err)
	config.RootFS = original.RootFS
	config.History = original.History
	img, err = mutate.ConfigFile(img, config)
	require.NoError(t, err)
	return img
}

func Archive(t *testing.T, img v1.Image, architecture string) string {
	t.Helper()
	index := mutate.AppendManifests(empty.Index, mutate.IndexAddendum{Add: img, Descriptor: v1.Descriptor{Platform: &v1.Platform{OS: "linux", Architecture: architecture}}})
	return IndexArchive(t, index)
}

func IndexArchive(t *testing.T, index v1.ImageIndex) string {
	t.Helper()
	directory := t.TempDir()
	_, err := layout.Write(directory, index)
	require.NoError(t, err)
	archive := filepath.Join(t.TempDir(), "fixture.oci.tar")
	output, err := os.Create(archive)
	require.NoError(t, err)
	writer := tar.NewWriter(output)
	require.NoError(t, filepath.WalkDir(directory, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		info, err := entry.Info()
		require.NoError(t, err)
		name, err := filepath.Rel(directory, path)
		require.NoError(t, err)
		require.NoError(t, writer.WriteHeader(&tar.Header{Name: filepath.ToSlash(name), Mode: 0o644, Size: info.Size()}))
		file, err := os.Open(path)
		require.NoError(t, err)
		_, copyErr := io.Copy(writer, file)
		require.NoError(t, file.Close())
		return copyErr
	}))
	require.NoError(t, writer.Close())
	require.NoError(t, output.Close())
	return archive
}
