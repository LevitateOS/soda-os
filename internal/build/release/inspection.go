package release

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
)

func (p *Publisher) inspect(img v1.Image, exactReference string) (Record, error) {
	configFile, err := img.ConfigFile()
	if err != nil {
		return Record{}, fmt.Errorf("inspect image configuration: %w", err)
	}
	revision, err := p.inspectImageIdentity(configFile)
	if err != nil {
		return Record{}, err
	}
	inventoryDigest, err := inspectRPMInventory(img)
	if err != nil {
		return Record{}, err
	}
	return Record{SchemaVersion: 4, SodaVersion: p.spec.Identity.Version, SourceRevision: revision, Platform: p.spec.Base.Platform, Channel: p.spec.Platform.Release.Channel, FedoraBaseReference: p.spec.Base.Reference, SodaImageReference: exactReference, ArtifactChecksums: ArtifactChecksums{RPMInventorySHA256: inventoryDigest}}, nil
}

func (p *Publisher) inspectImageIdentity(configFile *v1.ConfigFile) (string, error) {
	if configFile.OS != "linux" || configFile.Architecture != p.spec.Platform.Architecture.OCI {
		return "", fmt.Errorf("release image platform is %s/%s, expected %s", configFile.OS, configFile.Architecture, p.spec.Base.Platform)
	}
	labels := configFile.Config.Labels
	revision := labels["org.opencontainers.image.revision"]
	if len(revision) != 40 || !hexadecimal(revision) {
		return "", errors.New("release image has no full source revision label")
	}
	if labels["org.opencontainers.image.version"] != p.spec.Identity.Version {
		return "", errors.New("release image version label differs from the Soda specification")
	}
	if labels["org.opencontainers.image.base.name"] != p.spec.Base.Reference {
		return "", errors.New("release image Fedora base label differs from the Soda specification")
	}
	return revision, nil
}

func inspectRPMInventory(img v1.Image) (string, error) {
	inventory, err := imageFile(img, "usr/share/soda/rpm-inventory.txt")
	if err != nil {
		return "", fmt.Errorf("read installed RPM inventory: %w", err)
	}
	sidecar, err := imageFile(img, "usr/share/soda/rpm-inventory.sha256")
	if err != nil {
		return "", fmt.Errorf("read installed RPM inventory sidecar: %w", err)
	}
	inventoryDigest := sha256Hex(inventory)
	fields := strings.Fields(string(sidecar))
	if len(fields) != 2 || fields[0] != inventoryDigest || fields[1] != "rpm-inventory.txt" {
		return "", errors.New("installed RPM inventory does not match its image sidecar")
	}
	return inventoryDigest, nil
}

func imageFromOCIArchive(path, architecture string) (v1.Image, func(), error) {
	if !regularFile(path) {
		return nil, func() {}, fmt.Errorf("OCI archive %q is not a regular file", path)
	}
	directory, err := os.MkdirTemp("", "soda-oci-layout-")
	if err != nil {
		return nil, func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(directory) }
	if err := extractOCIArchive(path, directory); err != nil {
		cleanup()
		return nil, func() {}, err
	}
	index, err := layout.ImageIndexFromPath(directory)
	if err != nil {
		cleanup()
		return nil, func() {}, fmt.Errorf("read OCI archive: %w", err)
	}
	image, err := platformImage(index, architecture)
	if err != nil {
		cleanup()
		return nil, func() {}, err
	}
	return image, cleanup, nil
}

func platformImage(index v1.ImageIndex, architecture string) (v1.Image, error) {
	manifest, err := index.IndexManifest()
	if err != nil {
		return nil, err
	}
	if len(manifest.Manifests) != 1 {
		return nil, errors.New("OCI archive must contain exactly one manifest")
	}
	selected := &manifest.Manifests[0]
	if selected.Platform == nil || selected.Platform.OS != "linux" || selected.Platform.Architecture != architecture {
		return nil, fmt.Errorf("OCI archive manifest must be linux/%s", architecture)
	}
	img, err := index.Image(selected.Digest)
	if err != nil {
		return nil, fmt.Errorf("read platform image: %w", err)
	}
	return img, nil
}

func extractOCIArchive(path, directory string) error {
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
		if err := writeOCIArchiveEntry(reader, header, directory); err != nil {
			return err
		}
	}
}

func writeOCIArchiveEntry(reader *tar.Reader, header *tar.Header, directory string) error {
	clean := filepath.Clean(header.Name)
	if clean == "." {
		return nil
	}
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
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
		return fmt.Errorf("OCI archive contains unsupported entry %q", header.Name)
	}
}

func imageFile(img v1.Image, target string) ([]byte, error) {
	layers, err := img.Layers()
	if err != nil {
		return nil, err
	}
	for index := len(layers) - 1; index >= 0; index-- {
		contents, found, readErr := imageFileInLayer(layers[index], target)
		if readErr != nil {
			return nil, readErr
		}
		if found {
			return contents, nil
		}
	}
	return nil, fmt.Errorf("image file /%s is missing", target)
}

func imageFileInLayer(layer v1.Layer, target string) ([]byte, bool, error) {
	stream, err := layer.Uncompressed()
	if err != nil {
		return nil, false, err
	}
	defer stream.Close()
	reader := tar.NewReader(stream)
	for {
		header, nextErr := reader.Next()
		if errors.Is(nextErr, io.EOF) {
			return nil, false, nil
		}
		if nextErr != nil {
			return nil, false, nextErr
		}
		if strings.TrimPrefix(filepath.Clean(header.Name), "/") == target {
			contents, readErr := io.ReadAll(reader)
			return contents, true, readErr
		}
	}
}

func sha256Hex(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

func regularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func inspectQCOW2Artifacts(rawPath, compressedPath string) (string, string, error) {
	if filepath.Ext(rawPath) != ".qcow2" || filepath.Ext(compressedPath) != ".zst" || strings.TrimSuffix(compressedPath, ".zst") != rawPath {
		return "", "", errors.New("release record QCOW2 paths must be a raw .qcow2 and its .qcow2.zst download")
	}
	for _, item := range []struct{ label, path string }{{"raw QCOW2", rawPath}, {"compressed QCOW2", compressedPath}, {"compressed QCOW2 checksum", compressedPath + ".sha256"}} {
		info, err := os.Lstat(item.path)
		if err != nil || !info.Mode().IsRegular() {
			return "", "", fmt.Errorf("%s %q must be a regular non-symlink file", item.label, item.path)
		}
	}
	rawChecksum, err := fileSHA256(rawPath)
	if err != nil {
		return "", "", fmt.Errorf("checksum raw QCOW2: %w", err)
	}
	compressedChecksum, err := fileSHA256(compressedPath)
	if err != nil {
		return "", "", fmt.Errorf("checksum compressed QCOW2: %w", err)
	}
	if err := validateSidecar(compressedPath+".sha256", compressedChecksum, filepath.Base(compressedPath)); err != nil {
		return "", "", fmt.Errorf("validate compressed QCOW2 checksum: %w", err)
	}
	return rawChecksum, compressedChecksum, nil
}

func hexadecimal(value string) bool {
	_, err := hex.DecodeString(value)
	return err == nil
}
