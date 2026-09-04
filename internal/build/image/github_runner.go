package image

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/BurntSushi/toml"
)

var (
	githubRunnerVersionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)
	githubRunnerSHA256Pattern  = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

type githubRunnerSourceLock struct {
	Version    string              `toml:"version"`
	ReleaseURL string              `toml:"release_url"`
	Asset      []githubRunnerAsset `toml:"asset"`
}

type githubRunnerAsset struct {
	Architecture string `toml:"architecture"`
	Archive      string `toml:"archive"`
	URL          string `toml:"url"`
	SHA256       string `toml:"sha256"`
}

func readGitHubRunnerSourceLock(path string) (githubRunnerSourceLock, error) {
	var lock githubRunnerSourceLock
	metadata, err := toml.DecodeFile(path, &lock)
	if err != nil {
		return githubRunnerSourceLock{}, fmt.Errorf("parse GitHub runner source lock: %w", err)
	}
	if undecoded := metadata.Undecoded(); len(undecoded) != 0 {
		return githubRunnerSourceLock{}, fmt.Errorf("GitHub runner source lock has unknown key %s", undecoded[0])
	}
	if err = lock.validate(); err != nil {
		return githubRunnerSourceLock{}, err
	}
	return lock, nil
}

func (lock githubRunnerSourceLock) validate() error {
	if !githubRunnerVersionPattern.MatchString(lock.Version) || lock.ReleaseURL == "" || len(lock.Asset) != 2 {
		return errors.New("GitHub runner source lock does not bind one release for both Soda architectures")
	}
	seen := map[string]bool{}
	for _, asset := range lock.Asset {
		if err := lock.validateAsset(asset, seen); err != nil {
			return err
		}
	}
	return nil
}

func (lock githubRunnerSourceLock) validateAsset(asset githubRunnerAsset, seen map[string]bool) error {
	if seen[asset.Architecture] || (asset.Architecture != "aarch64" && asset.Architecture != "x86_64") {
		return errors.New("GitHub runner source lock has an invalid or duplicate architecture")
	}
	seen[asset.Architecture] = true
	if !validSourceArchive(asset.Archive) || asset.URL == "" || !githubRunnerSHA256Pattern.MatchString(asset.SHA256) {
		return fmt.Errorf("GitHub runner source lock has invalid %s asset metadata", asset.Architecture)
	}
	return nil
}

func (lock githubRunnerSourceLock) asset(architecture string) (githubRunnerAsset, error) {
	for _, asset := range lock.Asset {
		if asset.Architecture == architecture {
			return asset, nil
		}
	}
	return githubRunnerAsset{}, fmt.Errorf("GitHub runner source lock is missing %s", architecture)
}
