package image

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

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
var sourceArchivePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

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
	if !forgejoVersionPattern.MatchString(lock.Version) || !validSourceArchive(lock.SourceArchive) || lock.URL == "" || !validSHA256(lock.SHA256) ||
		!validSHA256(lock.PatchSHA256) || lock.BuildTags == "" {
		return errors.New("Forgejo source lock is incomplete or invalid")
	}
	return nil
}

func validSourceArchive(value string) bool {
	return sourceArchivePattern.MatchString(value) && strings.HasSuffix(value, ".tar.gz")
}
