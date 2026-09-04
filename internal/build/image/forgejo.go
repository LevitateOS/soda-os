package image

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"

	"github.com/BurntSushi/toml"
)

type forgejoSourceLock struct {
	Version       string `toml:"version"`
	SourceArchive string `toml:"source_archive"`
	URL           string `toml:"url"`
	SHA256        string `toml:"sha256"`
	PatchSHA256   string `toml:"patch_sha256"`
	BuildTags     string `toml:"build_tags"`
}

var forgejoVersionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

func readForgejoSourceLock(path string) (forgejoSourceLock, error) {
	var lock forgejoSourceLock
	metadata, err := toml.DecodeFile(path, &lock)
	if err != nil {
		return forgejoSourceLock{}, fmt.Errorf("read Forgejo source lock: %w", err)
	}
	if len(metadata.Undecoded()) != 0 {
		return forgejoSourceLock{}, errors.New("Forgejo source lock contains unknown fields")
	}
	if err := lock.validate(); err != nil {
		return forgejoSourceLock{}, err
	}
	return lock, nil
}

func (lock forgejoSourceLock) validate() error {
	if !forgejoVersionPattern.MatchString(lock.Version) || filepath.Base(lock.SourceArchive) != lock.SourceArchive ||
		lock.SourceArchive != "forgejo-src-"+lock.Version+".tar.gz" ||
		lock.URL != "https://codeberg.org/forgejo/forgejo/releases/download/v"+lock.Version+"/"+lock.SourceArchive || !validSHA256(lock.SHA256) ||
		!validSHA256(lock.PatchSHA256) || lock.BuildTags == "" {
		return errors.New("Forgejo source lock is incomplete or invalid")
	}
	return nil
}
