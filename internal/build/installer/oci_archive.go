package installer

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/v1/layout"
)

func verifyArchiveDigest(path, expectedReference, architecture string) error {
	directory, err := os.MkdirTemp("", "soda-installer-oci-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(directory)
	if err := extractRuntimeOCIArchive(path, directory); err != nil {
		return err
	}
	index, err := layout.ImageIndexFromPath(directory)
	if err != nil {
		return fmt.Errorf("read runtime OCI layout: %w", err)
	}
	manifest, err := index.IndexManifest()
	if err != nil {
		return err
	}
	if len(manifest.Manifests) != 1 {
		return errors.New("runtime OCI archive must contain exactly one manifest")
	}
	descriptor := manifest.Manifests[0]
	if descriptor.Platform == nil || descriptor.Platform.OS != "linux" || descriptor.Platform.Architecture != architecture {
		return fmt.Errorf("runtime OCI archive manifest must be linux/%s", architecture)
	}
	expectedDigest := strings.TrimPrefix(expectedReference, Repository+"@")
	if descriptor.Digest.String() != expectedDigest {
		return fmt.Errorf("runtime OCI archive digest %s differs from exact payload %s", descriptor.Digest, expectedDigest)
	}
	return nil
}

func extractRuntimeOCIArchive(path, directory string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open runtime OCI archive: %w", err)
	}
	defer file.Close()
	reader := tar.NewReader(file)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read runtime OCI archive: %w", err)
		}
		if err := extractRuntimeOCIEntry(reader, header, directory); err != nil {
			return err
		}
	}
}

func extractRuntimeOCIEntry(reader *tar.Reader, header *tar.Header, directory string) error {
	clean := filepath.Clean(header.Name)
	if clean == "." {
		return nil
	}
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("runtime OCI archive contains unsafe path %q", header.Name)
	}
	target := filepath.Join(directory, clean)
	switch header.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(target, 0o755)
	case tar.TypeReg, tar.TypeRegA:
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(output, reader)
		closeErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	default:
		return fmt.Errorf("runtime OCI archive contains unsupported entry %q", header.Name)
	}
}
