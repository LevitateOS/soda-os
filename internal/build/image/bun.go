package image

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type bunSourceLock struct {
	Version                string                    `toml:"version"`
	LicenseURL             string                    `toml:"license_url"`
	LicenseUpstreamSHA256  string                    `toml:"license_upstream_sha256"`
	LicenseSHA256          string                    `toml:"license_sha256"`
	ChecksumManifestURL    string                    `toml:"checksum_manifest_url"`
	ChecksumManifestSHA256 string                    `toml:"checksum_manifest_sha256"`
	Asset                  map[string]bunSourceAsset `toml:"asset"`
}

type bunSourceAsset struct {
	Archive string `toml:"archive"`
	Member  string `toml:"member"`
	URL     string `toml:"url"`
	SHA256  string `toml:"sha256"`
}

func (b *Builder) stageBunSource(sources string) error {
	var lock bunSourceLock
	lockPath := b.path("distro/locks/bun-source.toml")
	metadata, err := toml.DecodeFile(lockPath, &lock)
	if err != nil {
		return fmt.Errorf("read Bun source lock: %w", err)
	}
	if len(metadata.Undecoded()) != 0 {
		return errors.New("Bun source lock contains unknown fields")
	}
	asset, ok := lock.Asset[b.Spec.Platform.Architecture.Name]
	if !validBunSourceLock(lock, b.Spec.Platform.Architecture.Name) || !ok || !validBunAsset(asset) {
		return errors.New("Bun source lock is incomplete or invalid for the selected architecture")
	}
	license := b.path("packaging/rpm/bun/sources/LICENSE.md")
	if err := verifyFileSHA256(license, lock.LicenseSHA256); err != nil {
		return fmt.Errorf("verify Bun license: %w", err)
	}
	archive := b.artifactPath("tools", asset.Archive)
	if err := verifyFileSHA256(archive, asset.SHA256); err != nil {
		return fmt.Errorf("verify Bun source; run just bun-source on matching-native %s: %w", b.Spec.Platform.Architecture.Name, err)
	}
	if err := extractZipMember(archive, asset.Member, filepath.Join(sources, "bun")); err != nil {
		return fmt.Errorf("extract Bun source: %w", err)
	}
	return copyFile(license, filepath.Join(sources, "LICENSE.md"))
}

func validBunSourceLock(lock bunSourceLock, architecture string) bool {
	if !semanticVersionPattern.MatchString(lock.Version) || lock.LicenseURL == "" || lock.ChecksumManifestURL == "" ||
		!validSHA256(lock.LicenseUpstreamSHA256) || !validSHA256(lock.LicenseSHA256) || !validSHA256(lock.ChecksumManifestSHA256) {
		return false
	}
	if len(lock.Asset) != 2 || !validBunAsset(lock.Asset["x86_64"]) || !validBunAsset(lock.Asset["aarch64"]) {
		return false
	}
	_, ok := lock.Asset[architecture]
	return ok
}

func validBunAsset(asset bunSourceAsset) bool {
	return filepath.Base(asset.Archive) == asset.Archive && asset.Archive != "" && asset.URL != "" &&
		asset.Member != "" && !strings.HasPrefix(asset.Member, "/") && !strings.Contains(asset.Member, "..") &&
		validSHA256(asset.SHA256)
}

func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func verifyFileSHA256(path, expected string) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(contents)
	if hex.EncodeToString(digest[:]) != expected {
		return errors.New("SHA-256 checksum mismatch")
	}
	return nil
}

func extractZipMember(archive, member, destination string) error {
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer reader.Close()
	for _, item := range reader.File {
		if item.Name != member {
			continue
		}
		source, err := item.Open()
		if err != nil {
			return err
		}
		defer source.Close()
		target, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(target, source)
		closeErr := target.Close()
		return errors.Join(copyErr, closeErr)
	}
	return fmt.Errorf("archive does not contain %s", member)
}
