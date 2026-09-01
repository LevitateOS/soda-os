package config

import (
	"fmt"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const (
	DefaultToolchainsDir = "/opt/soda/toolchains"
	DefaultDaemonSocket  = "/run/soda/sodad.sock"
)

type DistroSpec struct {
	SchemaVersion uint32           `toml:"schema_version"`
	Identity      IdentitySpec     `toml:"identity"`
	Base          BaseSpec         `toml:"base"`
	Image         ImageSpec        `toml:"image"`
	Distribution  DistributionSpec `toml:"distribution"`
	Build         BuildSpec        `toml:"build"`
	Paths         PathSpec         `toml:"paths"`
	Platform      PlatformSpec     `toml:"-"`
}

type DistributionSpec struct {
	GitHubRepository string `toml:"github_repository" json:"github_repository"`
	IndexURL         string `toml:"index_url" json:"index_url"`
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
	StateSchema uint32 `toml:"state_schema"`
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

type PathSpec struct {
	ToolchainsDir string `toml:"toolchains_dir"`
	DaemonSocket  string `toml:"daemon_socket"`
}

func LoadDistro(path, architecture string) (DistroSpec, error) {
	var spec DistroSpec
	if _, err := toml.DecodeFile(path, &spec); err != nil {
		return DistroSpec{}, fmt.Errorf("decode distro specification %q: %w", path, err)
	}
	if spec.SchemaVersion != 2 {
		return DistroSpec{}, fmt.Errorf("unsupported distro schema version %d; expected 2", spec.SchemaVersion)
	}
	if _, ok := architectureContract[architecture]; !ok {
		return DistroSpec{}, fmt.Errorf("unsupported Soda architecture %q", architecture)
	}
	platformPath := filepath.Join(filepath.Dir(path), "platforms", architecture+".toml")
	if _, err := toml.DecodeFile(platformPath, &spec.Platform); err != nil {
		return DistroSpec{}, fmt.Errorf("decode platform specification %q: %w", platformPath, err)
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
		!validPlatformBase(spec.Base) || !validPlatformBuild(spec.Builder) ||
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
	return spec.Reference != "" && spec.Archive != "" && len(spec.ArchiveSHA256) == 64 &&
		spec.BootcNEVRA != "" && spec.RuntimePackageLock != ""
}

func validPlatformBuild(builder PlatformBuilder) bool {
	return builder.BaseReference != "" && builder.PackageLock != "" && builder.GoArchive != "" && len(builder.GoArchiveSHA256) == 64
}

func validPlatformInstaller(installer PlatformInstaller, release PlatformRelease, artifactArchitecture string) bool {
	return installer.PackageLock != "" && installer.ToolLock != "" && installer.ISOConfig != "" &&
		release.Channel == artifactArchitecture
}
