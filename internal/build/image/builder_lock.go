package image

import (
	"errors"
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/LevitateOS/soda-os/internal/config"
)

type builderPackageLock struct {
	SchemaVersion   uint32          `toml:"schema_version"`
	BaseReference   string          `toml:"base_reference"`
	Platform        string          `toml:"platform"`
	InventorySHA256 string          `toml:"inventory_sha256"`
	Package         []lockedPackage `toml:"package"`
}

func readBuilderPackageLock(path string, platform config.PlatformSpec) (builderPackageLock, error) {
	var lock builderPackageLock
	metadata, err := toml.DecodeFile(path, &lock)
	if err != nil {
		return builderPackageLock{}, fmt.Errorf("parse builder package lock: %w", err)
	}
	if len(metadata.Undecoded()) != 0 {
		return builderPackageLock{}, errors.New("builder package lock contains unknown fields")
	}
	if err := validateBuilderPackageLock(lock, platform); err != nil {
		return builderPackageLock{}, err
	}
	return lock, nil
}

func validateBuilderPackageLock(lock builderPackageLock, platform config.PlatformSpec) error {
	if !validBuilderPackageLockIdentity(lock, platform) {
		return errors.New("builder package lock differs from the selected platform contract")
	}
	seen := make(map[string]bool, len(lock.Package))
	for _, item := range lock.Package {
		if item.Name == "" || item.NEVRA == "" || seen[item.Name] || !nativeNEVRA(item.NEVRA, platform.Architecture.Name) {
			return errors.New("builder package lock has an invalid, duplicate, or cross-platform package")
		}
		seen[item.Name] = true
	}
	return nil
}

func validBuilderPackageLockIdentity(lock builderPackageLock, platform config.PlatformSpec) bool {
	return lock.SchemaVersion == 1 && lock.BaseReference == platform.Builder.BaseReference && lock.Platform == platform.Architecture.Platform && validSHA256(lock.InventorySHA256) && len(lock.Package) != 0
}

func nativeNEVRA(nevra, architecture string) bool {
	return strings.HasSuffix(nevra, "."+architecture) || strings.HasSuffix(nevra, ".noarch")
}
