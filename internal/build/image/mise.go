package image

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type miseSourceLock struct {
	Version string                     `toml:"version"`
	Asset   map[string]miseSourceAsset `toml:"asset"`
}

type miseSourceAsset struct {
	NEVRA  string `toml:"nevra"`
	File   string `toml:"file"`
	URL    string `toml:"url"`
	SHA256 string `toml:"sha256"`
}

func (b *Builder) stageMiseRPM(destination string) error {
	lock, err := readMiseSourceLock(b.path("distro/locks/mise-source.toml"))
	if err != nil {
		return err
	}
	asset, ok := lock.Asset[b.Spec.Platform.Architecture.Name]
	if !ok {
		return fmt.Errorf("mise source lock has no %s asset", b.Spec.Platform.Architecture.Name)
	}
	source := b.artifactPath("tools", asset.File)
	if err := verifyFileSHA256(source, asset.SHA256); err != nil {
		return fmt.Errorf("verify mise RPM; run just mise-rpm on matching-native %s: %w", b.Spec.Platform.Architecture.Name, err)
	}
	return copyFile(source, filepath.Join(destination, asset.File))
}

func readMiseSourceLock(path string) (miseSourceLock, error) {
	var lock miseSourceLock
	metadata, err := toml.DecodeFile(path, &lock)
	if err != nil {
		return miseSourceLock{}, fmt.Errorf("read mise source lock: %w", err)
	}
	if len(metadata.Undecoded()) != 0 {
		return miseSourceLock{}, errors.New("mise source lock is incomplete or invalid")
	}
	if err := lock.validate(); err != nil {
		return miseSourceLock{}, err
	}
	return lock, nil
}

func (lock miseSourceLock) validate() error {
	if !semanticVersionPattern.MatchString(lock.Version) || len(lock.Asset) != 2 {
		return errors.New("mise source lock is incomplete or invalid")
	}
	for _, architecture := range []string{"aarch64", "x86_64"} {
		asset, ok := lock.Asset[architecture]
		if !ok || !asset.valid(lock.Version, architecture) {
			return errors.New("mise source lock is incomplete or invalid")
		}
	}
	return nil
}

func (asset miseSourceAsset) valid(version, architecture string) bool {
	expectedFile := strings.Replace(asset.NEVRA, "-0:", "-", 1) + ".rpm"
	_, releaseAndArchitecture, found := strings.Cut(asset.NEVRA, ".fc")
	release, _, hasArchitecture := strings.Cut(releaseAndArchitecture, ".")
	return strings.HasPrefix(asset.NEVRA, "mise-0:"+version+"-") && strings.HasSuffix(asset.NEVRA, "."+architecture) &&
		asset.File == expectedFile && filepath.Base(asset.File) == asset.File && strings.HasSuffix(asset.URL, "/"+asset.File) &&
		found && hasArchitecture && release != "" && strings.Contains(asset.URL, "fedora-"+release+"-"+architecture+"/") && validSHA256(asset.SHA256)
}

func (lock miseSourceLock) runtimePackage(architecture string) (lockedPackage, error) {
	asset, ok := lock.Asset[architecture]
	if !ok {
		return lockedPackage{}, fmt.Errorf("mise source lock has no %s asset", architecture)
	}
	return lockedPackage{Name: "mise", NEVRA: asset.NEVRA, Source: "external-rpm", File: asset.File}, nil
}
