package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type DistroSpec struct {
	SchemaVersion uint32           `toml:"schema_version"`
	Identity      IdentitySpec     `toml:"identity"`
	Base          BaseSpec         `toml:"base"`
	Image         ImageSpec        `toml:"image"`
	Distribution  DistributionSpec `toml:"distribution"`
	Build         BuildSpec        `toml:"build"`
	Platform      PlatformSpec     `toml:"-"`
}

type DistributionSpec struct {
	GitHubRepository string `toml:"github_repository" json:"github_repository"`
}

type IdentitySpec struct {
	Name         string `toml:"name"`
	ID           string `toml:"id"`
	Version      string `toml:"version"`
	Hostname     string `toml:"hostname"`
	Architecture string `toml:"architecture"`
}

type BaseSpec struct {
	Reference string `toml:"reference"`
	Platform  string `toml:"platform"`
}

type ImageSpec struct {
	Registry    string `toml:"registry"`
	PackageLock string `toml:"package_lock"`
}

type PlatformSpec struct {
	SchemaVersion uint32               `toml:"schema_version"`
	Architecture  PlatformArchitecture `toml:"architecture"`
	Base          PlatformBase         `toml:"base"`
	Builder       PlatformBuilder      `toml:"builder"`
	Installer     PlatformInstaller    `toml:"installer"`
	Release       PlatformRelease      `toml:"release"`
}

type PlatformArchitecture struct {
	Name      string `toml:"name"`
	OCI       string `toml:"oci"`
	Platform  string `toml:"platform"`
	Artifact  string `toml:"artifact"`
	Installer string `toml:"installer"`
}

type PlatformBase struct {
	Reference          string `toml:"reference"`
	Archive            string `toml:"archive"`
	ArchiveSHA256      string `toml:"archive_sha256"`
	BootcNEVRA         string `toml:"bootc_nevra"`
	RuntimePackageLock string `toml:"runtime_package_lock"`
}

type PlatformBuilder struct {
	BaseReference   string `toml:"base_reference"`
	PackageLock     string `toml:"package_lock"`
	GoVersion       string `toml:"go_version"`
	GoURL           string `toml:"go_url"`
	GoArchive       string `toml:"go_archive"`
	GoArchiveSHA256 string `toml:"go_archive_sha256"`
}

type PlatformInstaller struct {
	PackageLock string `toml:"package_lock"`
	ToolLock    string `toml:"tool_lock"`
	ISOConfig   string `toml:"iso_config"`
}

type PlatformRelease struct {
	Channel string `toml:"channel"`
}

type BuildSpec struct {
	SourceDateEpoch int64 `toml:"source_date_epoch"`
}

func LoadDistro(path, architecture string) (DistroSpec, error) {
	var spec DistroSpec
	metadata, err := toml.DecodeFile(path, &spec)
	if err != nil {
		return DistroSpec{}, fmt.Errorf("decode distro specification %q: %w", path, err)
	}
	if len(metadata.Undecoded()) != 0 {
		return DistroSpec{}, fmt.Errorf("distro specification %q contains unknown fields", path)
	}
	if spec.SchemaVersion != 2 {
		return DistroSpec{}, fmt.Errorf("unsupported distro schema version %d; expected 2", spec.SchemaVersion)
	}
	if _, ok := architectureContract[architecture]; !ok {
		return DistroSpec{}, fmt.Errorf("unsupported Soda architecture %q", architecture)
	}
	platformPath := filepath.Join(filepath.Dir(path), "platforms", architecture+".toml")
	metadata, err = toml.DecodeFile(platformPath, &spec.Platform)
	if err != nil {
		return DistroSpec{}, fmt.Errorf("decode platform specification %q: %w", platformPath, err)
	}
	if len(metadata.Undecoded()) != 0 {
		return DistroSpec{}, fmt.Errorf("platform specification %q contains unknown fields", platformPath)
	}
	if err := validatePlatformSpec(spec.Platform, architecture); err != nil {
		return DistroSpec{}, err
	}
	spec.Identity.Architecture = spec.Platform.Architecture.Name
	spec.Base = BaseSpec{Reference: spec.Platform.Base.Reference, Platform: spec.Platform.Architecture.Platform}
	spec.Image.PackageLock = spec.Platform.Base.RuntimePackageLock
	return spec, nil
}

var architectureContract = map[string]struct{ oci, artifact, installer string }{
	"aarch64": {oci: "arm64", artifact: "aarch64", installer: "aarch64"},
	"x86_64":  {oci: "amd64", artifact: "x86_64", installer: "x86_64"},
}

func RequireNativeHostArchitecture(architecture, hostArchitecture string) error {
	expected, ok := architectureContract[architecture]
	if !ok {
		return fmt.Errorf("unsupported Soda architecture %q", architecture)
	}
	if hostArchitecture != expected.oci {
		return fmt.Errorf("Soda %s artifact operations require a native %s host; running on %s", architecture, expected.oci, hostArchitecture)
	}
	return nil
}

func validatePlatformSpec(spec PlatformSpec, requested string) error {
	expected := architectureContract[requested]
	if spec.SchemaVersion != 1 || !validPlatformArchitecture(spec.Architecture, requested, expected) ||
		!validPlatformBase(spec.Base) || !validPlatformBuild(spec.Builder, expected.oci) ||
		!validPlatformInstaller(spec.Installer, spec.Release, expected.artifact) {
		return fmt.Errorf("platform specification for %s differs from the Soda architecture contract", requested)
	}
	return nil
}

func validPlatformArchitecture(spec PlatformArchitecture, requested string, expected struct{ oci, artifact, installer string }) bool {
	return spec.Name == requested && spec.OCI == expected.oci && spec.Platform == "linux/"+expected.oci &&
		spec.Artifact == expected.artifact && spec.Installer == expected.installer
}

func validPlatformBase(spec PlatformBase) bool {
	return digestReference(spec.Reference) && filepath.Base(spec.Archive) != "." && strings.HasSuffix(spec.Archive, ".tar") &&
		validSHA256(spec.ArchiveSHA256) && spec.BootcNEVRA != "" && spec.RuntimePackageLock != ""
}

func validPlatformBuild(builder PlatformBuilder, _ string) bool {
	return digestReference(builder.BaseReference) && builder.PackageLock != "" && builder.GoVersion != "" && builder.GoURL != "" &&
		builder.GoArchive != "" && validSHA256(builder.GoArchiveSHA256)
}

func validPlatformInstaller(installer PlatformInstaller, release PlatformRelease, artifactArchitecture string) bool {
	return installer.PackageLock != "" && installer.ToolLock != "" && installer.ISOConfig != "" &&
		release.Channel == artifactArchitecture
}

func digestReference(value string) bool {
	_, digest, found := strings.Cut(value, "@sha256:")
	return found && validSHA256(digest)
}

func validSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return false
		}
	}
	return true
}
