package image

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
)

type teaSourceLock struct {
	Version       string `toml:"version"`
	Commit        string `toml:"commit"`
	SourceArchive string `toml:"source_archive"`
	SourceURL     string `toml:"source_url"`
	SourceSHA256  string `toml:"source_sha256"`
	LicenseURL    string `toml:"license_url"`
	LicenseSHA256 string `toml:"license_sha256"`
}

var semanticVersionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

func (b *Builder) stageTeaSource(sources string) error {
	lock, err := readTeaSourceLock(b.path("distro/locks/tea-source.toml"))
	if err != nil {
		return err
	}
	license := b.path("packaging/rpm/tea/sources/LICENSE")
	if err := verifyFileSHA256(license, lock.LicenseSHA256); err != nil {
		return fmt.Errorf("verify Tea license: %w", err)
	}
	binary := b.artifactPath("build", "tea")
	if !isFile(binary) {
		return errors.New("built Tea binary is missing")
	}
	if err := copyFile(binary, filepath.Join(sources, "tea")); err != nil {
		return fmt.Errorf("stage Tea source: %w", err)
	}
	return copyFile(license, filepath.Join(sources, "tea-LICENSE"))
}

func readTeaSourceLock(path string) (teaSourceLock, error) {
	var lock teaSourceLock
	metadata, err := toml.DecodeFile(path, &lock)
	if err != nil {
		return teaSourceLock{}, fmt.Errorf("read Tea source lock: %w", err)
	}
	if len(metadata.Undecoded()) != 0 {
		return teaSourceLock{}, errors.New("Tea source lock contains unknown fields")
	}
	if err := lock.validate(); err != nil {
		return teaSourceLock{}, err
	}
	return lock, nil
}

func (lock teaSourceLock) validate() error {
	valid := semanticVersionPattern.MatchString(lock.Version) && validGitCommit(lock.Commit) &&
		validInputFilename(lock.SourceArchive) && strings.HasSuffix(lock.SourceArchive, ".gz") && lock.SourceURL != "" && validSHA256(lock.SourceSHA256) &&
		lock.LicenseURL != "" && validSHA256(lock.LicenseSHA256)
	if !valid {
		return errors.New("Tea source lock is incomplete or invalid")
	}
	return nil
}

func validGitCommit(value string) bool {
	return len(value) == 40 && regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(value)
}
