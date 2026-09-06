// Package oci inspects a single-platform Soda OCI archive without registry or
// release state. Callers own matching-native authorization and subsequent work.
package oci

import (
	"archive/tar"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/validate"
)

const Repository = "ghcr.io/levitateos/soda-os"

// Inspect returns facts from verified local bytes, not the caller's checkout.
// Version and base are validated independently of any current distribution spec.
func Inspect(path, architecture string) (Image, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return Image{}, fmt.Errorf("OCI archive %q must be a regular non-symlink file", path)
	}
	directory, err := os.MkdirTemp("", "soda-oci-layout-")
	if err != nil {
		return Image{}, err
	}
	defer os.RemoveAll(directory)
	if err := extractArchive(path, directory); err != nil {
		return Image{}, err
	}
	index, err := layout.ImageIndexFromPath(directory)
	if err != nil {
		return Image{}, fmt.Errorf("read OCI archive: %w", err)
	}
	img, err := platformImage(index, architecture)
	if err != nil {
		return Image{}, err
	}
	if err := validate.Image(img); err != nil {
		return Image{}, fmt.Errorf("verify OCI image integrity: %w", err)
	}
	return inspectImage(img, architecture)
}

func platformImage(index v1.ImageIndex, architecture string) (v1.Image, error) {
	manifest, err := index.IndexManifest()
	if err != nil {
		return nil, err
	}
	if len(manifest.Manifests) != 1 {
		return nil, errors.New("OCI archive must contain exactly one manifest")
	}
	selected := manifest.Manifests[0]
	if selected.Platform == nil || selected.Platform.OS != "linux" || selected.Platform.Architecture != architecture {
		return nil, fmt.Errorf("OCI archive manifest must be linux/%s", architecture)
	}
	img, err := index.Image(selected.Digest)
	if err != nil {
		return nil, fmt.Errorf("read platform image: %w", err)
	}
	return img, nil
}

func extractArchive(path, directory string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	reader := tar.NewReader(file)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read OCI archive: %w", err)
		}
		if err := writeArchiveEntry(reader, header, directory); err != nil {
			return err
		}
	}
}

func writeArchiveEntry(reader *tar.Reader, header *tar.Header, directory string) error {
	clean := filepath.Clean(header.Name)
	if !filepath.IsLocal(clean) {
		return fmt.Errorf("OCI archive contains unsafe path %q", header.Name)
	}
	target := filepath.Join(directory, clean)
	switch header.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(target, 0o755)
	case tar.TypeReg, tar.TypeRegA:
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		digest := sha256.New()
		_, copyErr := io.Copy(io.MultiWriter(output, digest), reader)
		closeErr := output.Close()
		if strings.HasPrefix(clean, "blobs/sha256/") && filepath.Base(clean) != fmt.Sprintf("%x", digest.Sum(nil)) {
			return fmt.Errorf("OCI blob integrity differs from path %s", clean)
		}
		return errors.Join(copyErr, closeErr)
	default:
		return fmt.Errorf("OCI archive contains unsupported entry %q", header.Name)
	}
}
