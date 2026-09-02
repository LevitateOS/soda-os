package image

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

const teaVersion = "0.15.1"

type teaSourceLock struct {
	Version                string                    `toml:"version"`
	LicenseURL             string                    `toml:"license_url"`
	LicenseSHA256          string                    `toml:"license_sha256"`
	ChecksumManifestURL    string                    `toml:"checksum_manifest_url"`
	ChecksumManifestSHA256 string                    `toml:"checksum_manifest_sha256"`
	Asset                  map[string]teaSourceAsset `toml:"asset"`
}

type teaSourceAsset struct {
	Archive string `toml:"archive"`
	URL     string `toml:"url"`
	SHA256  string `toml:"sha256"`
}

func (b *Builder) stageTeaSource(sources string) error {
	lock, asset, err := readTeaSourceLock(b.path("distro/locks/tea-source.toml"), b.Spec.Platform.Architecture.Name)
	if err != nil {
		return err
	}
	license := b.path("packaging/rpm/tea/sources/LICENSE")
	if err := verifyFileSHA256(license, lock.LicenseSHA256); err != nil {
		return fmt.Errorf("verify Tea license: %w", err)
	}
	archive := b.artifactPath("tools", asset.Archive)
	if err := verifyFileSHA256(archive, asset.SHA256); err != nil {
		return fmt.Errorf("verify Tea source; run just tea-source on matching-native %s: %w", b.Spec.Platform.Architecture.Name, err)
	}
	if err := copyFile(archive, filepath.Join(sources, "tea.xz")); err != nil {
		return fmt.Errorf("stage Tea source: %w", err)
	}
	return copyFile(license, filepath.Join(sources, "tea-LICENSE"))
}

func readTeaSourceLock(path, architecture string) (teaSourceLock, teaSourceAsset, error) {
	var lock teaSourceLock
	metadata, err := toml.DecodeFile(path, &lock)
	if err != nil {
		return teaSourceLock{}, teaSourceAsset{}, fmt.Errorf("read Tea source lock: %w", err)
	}
	if len(metadata.Undecoded()) != 0 {
		return teaSourceLock{}, teaSourceAsset{}, errors.New("Tea source lock contains unknown fields")
	}
	asset, err := lock.assetFor(architecture)
	return lock, asset, err
}

func (lock teaSourceLock) assetFor(architecture string) (teaSourceAsset, error) {
	asset, selected := lock.Asset[architecture]
	if lock.Version != teaVersion || lock.LicenseURL == "" || !validSHA256(lock.LicenseSHA256) {
		return teaSourceAsset{}, errors.New("Tea source lock differs from the selected architecture contract")
	}
	if lock.ChecksumManifestURL == "" || !validSHA256(lock.ChecksumManifestSHA256) {
		return teaSourceAsset{}, errors.New("Tea source lock differs from the selected architecture contract")
	}
	if !validTeaAssets(lock.Asset) || !selected {
		return teaSourceAsset{}, errors.New("Tea source lock differs from the selected architecture contract")
	}
	return asset, nil
}

func validTeaAssets(assets map[string]teaSourceAsset) bool {
	if len(assets) != 2 {
		return false
	}
	for _, architecture := range []string{"aarch64", "x86_64"} {
		asset, present := assets[architecture]
		if !present || !validTeaAsset(asset) {
			return false
		}
	}
	return true
}

func validTeaAsset(asset teaSourceAsset) bool {
	return filepath.Base(asset.Archive) == asset.Archive && strings.HasSuffix(asset.Archive, ".xz") &&
		asset.URL != "" && validSHA256(asset.SHA256)
}
