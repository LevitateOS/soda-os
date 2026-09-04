package installer

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"

	"github.com/BurntSushi/toml"
	"github.com/LevitateOS/soda-os/internal/config"
)

func (b *Builder) validateSelectedToolLock(path, operation string) error {
	if b.Spec.Platform.Installer.ToolLock != "" && filepath.Clean(path) != filepath.Clean(b.path(b.Spec.Platform.Installer.ToolLock)) {
		return fmt.Errorf("%s must use the selected platform image-builder lock", operation)
	}
	return nil
}

func (b *Builder) validateInstallerInputs() error {
	if b.Spec.Platform.Installer.PackageLock == "" {
		return nil
	}
	return ValidateInputLocks(b.Root, b.Spec)
}

func readToolLock(path string, platform config.PlatformSpec) (toolLock, error) {
	var lock toolLock
	metadata, err := toml.DecodeFile(path, &lock)
	if err != nil {
		return toolLock{}, fmt.Errorf("read image-builder lock: %w", err)
	}
	if len(metadata.Undecoded()) != 0 {
		return toolLock{}, errors.New("image-builder lock contains unknown fields")
	}
	if err := validateToolLock(lock, platform); err != nil {
		return toolLock{}, err
	}
	return lock, nil
}

func validateToolLock(lock toolLock, platform config.PlatformSpec) error {
	validReference := regexp.MustCompile(`^ghcr\.io/osbuild/image-builder@sha256:[0-9a-f]{64}$`).MatchString(lock.Reference)
	if !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(lock.Version) ||
		!regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(lock.Commit) || lock.Platform != platform.Architecture.Platform || !validReference {
		return errors.New("image-builder lock is incomplete or invalid for the selected platform")
	}
	return nil
}
