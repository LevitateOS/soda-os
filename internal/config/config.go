package config

import (
	"fmt"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const (
	DefaultStateDir      = "/var/lib/soda"
	DefaultProjectsDir   = "/srv/soda/projects"
	DefaultToolchainsDir = "/opt/soda/toolchains"
	DefaultDaemonSocket  = "/run/soda/sodad.sock"
)

type DistroSpec struct {
	SchemaVersion uint32       `toml:"schema_version"`
	Identity      IdentitySpec `toml:"identity"`
	Base          BaseSpec     `toml:"base"`
	Image         ImageSpec    `toml:"image"`
	Build         BuildSpec    `toml:"build"`
	Network       NetworkSpec  `toml:"network"`
	Paths         PathSpec     `toml:"paths"`
	Platform      PlatformSpec `toml:"-"`
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
	SchemaVersion            uint32 `toml:"schema_version"`
	Architecture             string `toml:"architecture"`
	OCIArchitecture          string `toml:"oci_architecture"`
	OCIPlatform              string `toml:"oci_platform"`
	ArtifactArchitecture     string `toml:"artifact_architecture"`
	InstallerArchitecture    string `toml:"installer_architecture"`
	BaseReference            string `toml:"base_reference"`
	BaseArchive              string `toml:"base_archive"`
	BaseArchiveSHA256        string `toml:"base_archive_sha256"`
	BootcNEVRA               string `toml:"bootc_nevra"`
	RuntimePackageLock       string `toml:"runtime_package_lock"`
	BuilderBaseReference     string `toml:"builder_base_reference"`
	BuilderPackageLock       string `toml:"builder_package_lock"`
	TargetCosignArchitecture string `toml:"target_cosign_architecture"`
	TargetCosignSHA256       string `toml:"target_cosign_sha256"`
	InstallerPackageLock     string `toml:"installer_package_lock"`
	InstallerToolLock        string `toml:"installer_tool_lock"`
	ReleaseChannel           string `toml:"release_channel"`
}

type BuildSpec struct {
	SourceDateEpoch int64 `toml:"source_date_epoch"`
}

type NetworkSpec struct {
	CockpitPort uint16 `toml:"cockpit_port"`
	MDNSName    string `toml:"mdns_name"`
}

type PathSpec struct {
	StateDir      string `toml:"state_dir"`
	ProjectsDir   string `toml:"projects_dir"`
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
	spec.Identity.Architecture = spec.Platform.Architecture
	spec.Base = BaseSpec{Reference: spec.Platform.BaseReference, Platform: spec.Platform.OCIPlatform}
	spec.Image.PackageLock = spec.Platform.RuntimePackageLock
	return spec, nil
}

var architectureContract = map[string]struct{ oci, artifact, installer string }{
	"aarch64": {oci: "arm64", artifact: "aarch64", installer: "aarch64"},
	"x86_64":  {oci: "amd64", artifact: "x86_64", installer: "x86_64"},
}

func validatePlatformSpec(spec PlatformSpec, requested string) error {
	expected := architectureContract[requested]
	if spec.SchemaVersion != 1 || spec.Architecture != requested || spec.OCIArchitecture != expected.oci ||
		spec.OCIPlatform != "linux/"+expected.oci || spec.ArtifactArchitecture != expected.artifact ||
		spec.InstallerArchitecture != expected.installer || spec.BaseReference == "" || spec.BaseArchive == "" ||
		len(spec.BaseArchiveSHA256) != 64 || spec.BootcNEVRA == "" || spec.RuntimePackageLock == "" ||
		spec.BuilderBaseReference == "" || spec.BuilderPackageLock == "" || spec.TargetCosignArchitecture != expected.oci || len(spec.TargetCosignSHA256) != 64 ||
		spec.InstallerPackageLock == "" || spec.InstallerToolLock == "" || spec.ReleaseChannel != expected.artifact {
		return fmt.Errorf("platform specification for %s differs from the Soda architecture contract", requested)
	}
	return nil
}
