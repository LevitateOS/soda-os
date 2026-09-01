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

const bunVersion = "1.4.0"

type bunSourceLock struct {
	Version       string                    `toml:"version"`
	LicenseSHA256 string                    `toml:"license_sha256"`
	Asset         map[string]bunSourceAsset `toml:"asset"`
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
	if _, err := toml.DecodeFile(lockPath, &lock); err != nil {
		return fmt.Errorf("read Bun source lock: %w", err)
	}
	asset, ok := lock.Asset[b.Spec.Platform.Architecture.Name]
	if lock.Version != bunVersion || !ok || !validBunAsset(asset) || !validSHA256(lock.LicenseSHA256) {
		return errors.New("Bun source lock differs from the selected architecture contract")
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
