package installer

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/LevitateOS/soda-os/internal/config"
)

// ValidateInputLocks validates the selected architecture's installer inputs
// before any artifact workspace is created.
func ValidateInputLocks(root string, spec config.DistroSpec) error {
	packagePath := resolveInputPath(root, spec.Platform.Installer.PackageLock)
	if _, err := readPackageLock(packagePath, spec.Platform); err != nil {
		return err
	}
	if spec.Platform.Installer.ToolLock == "" {
		return nil
	}
	_, err := readToolLock(resolveInputPath(root, spec.Platform.Installer.ToolLock), spec.Platform)
	return err
}

func resolveInputPath(root, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}

func readPackageLock(path string, platform config.PlatformSpec) (packageLock, error) {
	var lock packageLock
	metadata, err := toml.DecodeFile(path, &lock)
	if err != nil {
		return packageLock{}, fmt.Errorf("read installer package lock: %w", err)
	}
	if len(metadata.Undecoded()) != 0 {
		return packageLock{}, errors.New("installer package lock contains unknown fields")
	}
	if err := validatePackageLock(lock, platform); err != nil {
		return packageLock{}, err
	}
	return lock, nil
}

func validatePackageLock(lock packageLock, platform config.PlatformSpec) error {
	if lock.SchemaVersion != 1 || lock.Platform != platform.Architecture.Platform || len(lock.Packages) == 0 || len(lock.BootPackages) == 0 || lock.EFIVendor == "" {
		return errors.New("installer package lock differs from the selected platform contract")
	}
	if !validInstallerPackages(lock.Packages, platform.Architecture.Name) || !validBootPackages(lock.BootPackages) {
		return errors.New("installer package lock has an invalid, duplicate, or cross-platform package")
	}
	return nil
}

func validInstallerPackages(packages []string, architecture string) bool {
	seen := make(map[string]bool, len(packages))
	for _, nevra := range packages {
		if nevra == "" || seen[nevra] || !(strings.HasSuffix(nevra, "."+architecture) || strings.HasSuffix(nevra, ".noarch")) {
			return false
		}
		seen[nevra] = true
	}
	return true
}

func validBootPackages(packages []string) bool {
	for _, name := range packages {
		if name == "" || strings.ContainsAny(name, "\n\r \t") {
			return false
		}
	}
	return true
}
