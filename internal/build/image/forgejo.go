package image

import (
	"errors"
	"fmt"
	"path/filepath"
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
	return validInputFilename(value) && strings.HasSuffix(value, ".tar.gz")
}

func validInputFilename(value string) bool {
	return sourceArchivePattern.MatchString(value)
}

func (b *Builder) stageForgejoBranding(sources string) error {
	custom := "packaging/rpm/forgejo/sources/custom/"
	files := [][2]string{
		{"assets/branding/theme/palette.css", "soda-palette.css"},
		{"assets/branding/source/soda-symbol.svg", "soda-forgejo-logo.svg"},
		{"assets/branding/forgejo/logo.png", "soda-forgejo-logo.png"},
		{"assets/branding/forgejo/favicon.png", "soda-forgejo-favicon.png"},
		{"assets/branding/forgejo/apple-touch-icon.png", "soda-forgejo-apple-touch-icon.png"},
		{custom + "templates/home.tmpl", "soda-forgejo-home.tmpl"},
		{custom + "templates/custom/header.tmpl", "soda-forgejo-header.tmpl"},
	}
	for _, name := range []string{"theme-soda-light.css", "theme-soda-dark.css", "theme-soda-auto.css", "soda-controls.css"} {
		files = append(files, [2]string{custom + "public/assets/css/" + name, name})
	}
	for _, file := range files {
		if err := copyFile(b.path(file[0]), filepath.Join(sources, file[1])); err != nil {
			return err
		}
	}
	return nil
}
