package config

import (
	"fmt"

	"github.com/BurntSushi/toml"
)

const (
	DefaultStateDir      = "/var/lib/soda"
	DefaultProjectsDir   = "/srv/soda/projects"
	DefaultToolchainsDir = "/opt/soda/toolchains"
	DefaultDaemonSocket  = "/run/soda/sodad.sock"
)

type DistroSpec struct {
	SchemaVersion uint32        `toml:"schema_version"`
	Identity      IdentitySpec  `toml:"identity"`
	Base          BaseSpec      `toml:"base"`
	Installer     InstallerSpec `toml:"installer"`
	Network       NetworkSpec   `toml:"network"`
	Paths         PathSpec      `toml:"paths"`
}

type IdentitySpec struct {
	Name         string `toml:"name"`
	ID           string `toml:"id"`
	Version      string `toml:"version"`
	Hostname     string `toml:"hostname"`
	Architecture string `toml:"architecture"`
}

type BaseSpec struct {
	Distribution           string `toml:"distribution"`
	InstallerSourceVersion string `toml:"installer_source_version"`
	PackageStream          string `toml:"package_stream"`
	SourceISO              string `toml:"source_iso"`
	SourceISOSHA256        string `toml:"source_iso_sha256"`
	ChecksumFile           string `toml:"checksum_file"`
	SignatureFile          string `toml:"signature_file"`
}

type InstallerSpec struct {
	ProfileID          string             `toml:"profile_id"`
	AnacondaGUINEVRA   string             `toml:"anaconda_gui_nevra"`
	VolumeID           string             `toml:"volume_id"`
	BootTimeoutSeconds uint16             `toml:"boot_timeout_seconds"`
	BrandingManifest   string             `toml:"branding_manifest"`
	UpstreamManifest   string             `toml:"upstream_manifest"`
	Payload            NetworkPayloadSpec `toml:"payload"`
}

type NetworkPayloadSpec struct {
	Mode                     string   `toml:"mode"`
	BaseOSMirrorlist         string   `toml:"baseos_mirrorlist"`
	AppStreamMirrorlist      string   `toml:"appstream_mirrorlist"`
	InstallWeakDependencies  bool     `toml:"install_weak_dependencies"`
	MaxISOSizeBytes          uint64   `toml:"max_iso_size_bytes"`
	Environment              string   `toml:"environment"`
	Packages                 []string `toml:"packages"`
	AutomatedExtraPackages   []string `toml:"automated_extra_packages"`
	AnacondaRequiredPackages []string `toml:"anaconda_required_packages"`
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

type ProfileSpec struct {
	SchemaVersion uint32          `toml:"schema_version"`
	Profile       ProfileIdentity `toml:"profile"`
	Tools         []ToolSpec      `toml:"tools"`
}

type ProfileIdentity struct {
	ID          string `toml:"id"`
	DisplayName string `toml:"display_name"`
}

type ToolSpec struct {
	ID       string   `toml:"id"`
	Resolver string   `toml:"resolver"`
	Channel  string   `toml:"channel"`
	BinPaths []string `toml:"bin_paths"`
}

func LoadDistro(path string) (DistroSpec, error) {
	var spec DistroSpec
	if _, err := toml.DecodeFile(path, &spec); err != nil {
		return DistroSpec{}, fmt.Errorf("decode distro specification %q: %w", path, err)
	}
	if spec.SchemaVersion != 3 {
		return DistroSpec{}, fmt.Errorf("unsupported distro schema version %d; expected 3", spec.SchemaVersion)
	}
	return spec, nil
}

func LoadProfile(path string) (ProfileSpec, error) {
	var spec ProfileSpec
	if _, err := toml.DecodeFile(path, &spec); err != nil {
		return ProfileSpec{}, fmt.Errorf("decode profile specification %q: %w", path, err)
	}
	if spec.SchemaVersion != 1 {
		return ProfileSpec{}, fmt.Errorf("unsupported profile schema version %d; expected 1", spec.SchemaVersion)
	}
	return spec, nil
}
