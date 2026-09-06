package oci

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
)

// Image is an in-memory projection of native OCI identities and installed RPM
// inventory. It is not a release record or a serialization/approval contract.
type Image struct {
	Digest             string
	ConfigDigest       string
	Platform           string
	Architecture       string
	Version            string
	Revision           string
	BaseReference      string
	RPMInventorySHA256 string
}

func (image Image) Reference() string { return Repository + "@" + image.Digest }

var revisionPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

func inspectImage(img v1.Image, architecture string) (Image, error) {
	config, err := img.ConfigFile()
	if err != nil {
		return Image{}, err
	}
	if config.OS != "linux" || config.Architecture != architecture {
		return Image{}, fmt.Errorf("OCI configuration must be linux/%s", architecture)
	}
	labels := config.Config.Labels
	image := Image{
		Platform: "linux/" + architecture, Architecture: architecture,
		Version: labels["org.opencontainers.image.version"], Revision: labels["org.opencontainers.image.revision"],
		BaseReference: labels["org.opencontainers.image.base.name"],
	}
	if !revisionPattern.MatchString(image.Revision) || strings.TrimSpace(image.Version) == "" {
		return Image{}, errors.New("OCI image requires a version and full source revision label")
	}
	if _, err := name.NewDigest(image.BaseReference); err != nil {
		return Image{}, fmt.Errorf("OCI image requires an exact base reference: %w", err)
	}
	digest, err := img.Digest()
	if err != nil {
		return Image{}, err
	}
	configDigest, err := img.ConfigName()
	if err != nil {
		return Image{}, err
	}
	image.Digest, image.ConfigDigest = digest.String(), configDigest.String()
	image.RPMInventorySHA256, err = inspectRPMInventory(img)
	return image, err
}

func inspectRPMInventory(img v1.Image) (string, error) {
	// Flattening honors OCI whiteouts and replacements instead of reading a
	// removed inventory from a lower layer.
	stream := mutate.Extract(img)
	defer stream.Close()
	reader := tar.NewReader(stream)
	files := map[string][]byte{}
	for len(files) < 2 {
		header, err := reader.Next()
		if err != nil {
			return "", fmt.Errorf("read installed RPM inventory: %w", err)
		}
		clean := strings.TrimPrefix(path.Clean(header.Name), "/")
		switch clean {
		case "usr/share/soda/rpm-inventory.txt", "usr/share/soda/rpm-inventory.sha256":
		default:
			continue
		}
		// archive/tar normalizes legacy TypeRegA entries to TypeReg.
		if header.Typeflag != tar.TypeReg {
			return "", errors.New("installed RPM inventory and sidecar must be regular files")
		}
		files[clean], err = io.ReadAll(reader)
		if err != nil {
			return "", err
		}
	}
	return inventoryDigest(files)
}

func inventoryDigest(files map[string][]byte) (string, error) {
	inventory := files["usr/share/soda/rpm-inventory.txt"]
	if len(inventory) == 0 {
		return "", errors.New("installed RPM inventory is missing or empty")
	}
	digest := sha256.Sum256(inventory)
	expected := hex.EncodeToString(digest[:])
	fields := strings.Fields(string(files["usr/share/soda/rpm-inventory.sha256"]))
	if len(fields) != 2 || fields[0] != expected || fields[1] != "rpm-inventory.txt" {
		return "", errors.New("installed RPM inventory does not match its image sidecar")
	}
	return expected, nil
}
