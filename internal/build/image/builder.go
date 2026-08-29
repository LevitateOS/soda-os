package image

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/LevitateOS/soda-os/internal/config"
	"github.com/LevitateOS/soda-os/internal/process"
)

const (
	sodaRegistry  = "ghcr.io/levitateos/soda-os"
	cosignVersion = "v3.1.2"
)

var targetRPMs = []string{"soda-release", "soda-runtime", "soda-cockpit", "soda-forgejo"}

type packageLock struct {
	SchemaVersion uint32          `toml:"schema_version"`
	BaseReference string          `toml:"base_reference"`
	Package       []lockedPackage `toml:"package"`
}

type lockedPackage struct {
	Name   string `toml:"name"`
	NEVRA  string `toml:"nevra"`
	Source string `toml:"source"`
	File   string `toml:"file"`
}

type releaseToolLock struct {
	Version string              `toml:"version"`
	Binary  []releaseToolBinary `toml:"binary"`
}

type releaseToolBinary struct {
	OS     string `toml:"os"`
	Arch   string `toml:"arch"`
	SHA256 string `toml:"sha256"`
}

type Builder struct {
	Root             string
	Spec             config.DistroSpec
	SigningPublicKey string
	runner           process.Runner
}

func NewBuilderFromWorkingDirectory(specPath, architecture string, runner process.Runner) (*Builder, error) {
	root, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}
	return NewBuilder(root, specPath, architecture, runner)
}

func NewBuilder(root, specPath, architecture string, runner process.Runner) (*Builder, error) {
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("canonicalize workspace: %w", err)
	}
	if !filepath.IsAbs(specPath) {
		specPath = filepath.Join(canonicalRoot, specPath)
	}
	spec, err := config.LoadDistro(specPath, architecture)
	if err != nil {
		return nil, err
	}
	if runner == nil {
		runner = process.OSRunner{}
	}
	return &Builder{Root: canonicalRoot, Spec: spec, runner: runner}, nil
}

func (b *Builder) artifactPath(parts ...string) string {
	return filepath.Join(append([]string{b.Root, ".artifacts"}, parts...)...)
}

func (b *Builder) Check(_ context.Context) error {
	spec := b.Spec
	lock, err := b.packageLock()
	if err != nil {
		return err
	}
	toolLock, err := b.releaseToolLock()
	if err != nil {
		return err
	}
	return errors.Join(
		validateImageSpec(spec),
		validateRuntimePackageLock(lock, spec),
		validateReleaseToolLock(toolLock, spec.Platform),
		b.validateBuildInputs(),
	)
}

func validateImageSpec(spec config.DistroSpec) error {
	if spec.Identity.Architecture != spec.Platform.Architecture.Name || spec.Base.Reference != spec.Platform.Base.Reference || spec.Base.Platform != spec.Platform.Architecture.Platform || spec.Image.Registry != sodaRegistry || spec.Image.StateSchema != 3 || spec.Build.SourceDateEpoch < 0 {
		return errors.New("Soda image specification differs from the selected architecture contract")
	}
	return nil
}

func validateRuntimePackageLock(lock packageLock, spec config.DistroSpec) error {
	if lock.SchemaVersion != 1 || lock.BaseReference != spec.Base.Reference || len(lock.Package) <= len(targetRPMs) {
		return errors.New("package lock does not bind the configured Fedora bootc base")
	}
	local, bootcLocked, err := classifyRuntimePackages(lock.Package, spec.Platform.Base.BootcNEVRA)
	if err != nil {
		return err
	}
	if !bootcLocked || strings.Join(local, ",") != strings.Join(targetRPMs, ",") {
		return errors.New("package lock must contain the locked bootc package and exactly the Soda RPM inputs")
	}
	return nil
}

func classifyRuntimePackages(packages []lockedPackage, bootcNEVRA string) ([]string, bool, error) {
	seen, local, bootcLocked := make(map[string]bool, len(packages)), make([]string, 0, len(targetRPMs)), false
	for _, item := range packages {
		if err := validateLockedPackage(item, seen); err != nil {
			return nil, false, err
		}
		if item.Source == "local-rpm" {
			local = append(local, item.Name)
		}
		bootcLocked = bootcLocked || item.Name == "bootc" && item.NEVRA == bootcNEVRA
	}
	return local, bootcLocked, nil
}

func validateLockedPackage(item lockedPackage, seen map[string]bool) error {
	if item.Name == "" || item.NEVRA == "" || seen[item.Name] {
		return errors.New("package lock contains an empty or duplicate package")
	}
	seen[item.Name] = true
	if item.Source == "fedora" && item.File == "" {
		return nil
	}
	if item.Source == "local-rpm" && item.File != "" && filepath.Base(item.File) == item.File {
		return nil
	}
	return fmt.Errorf("package lock entry %s has an unsupported source or file", item.Name)
}

func validateReleaseToolLock(lock releaseToolLock, platform config.PlatformSpec) error {
	if lock.Version != cosignVersion || lock.checksum("linux", platform.Cosign.Architecture) != platform.Cosign.SHA256 {
		return fmt.Errorf("release tool lock must pin the approved Cosign %s Linux/%s binary", cosignVersion, platform.Cosign.Architecture)
	}
	return nil
}

func (b *Builder) validateBuildInputs() error {
	for _, path := range []string{"packaging/bootc/Containerfile", "packaging/builder/Containerfile", b.Spec.Platform.Builder.PackageLock, b.Spec.Platform.Installer.PackageLock, b.Spec.Platform.Installer.ToolLock, b.Spec.Platform.Installer.ISOConfig, "packaging/rpm/release/soda-release.spec", "packaging/rpm/runtime/soda-runtime.spec", "packaging/rpm/cockpit/soda-cockpit.spec", "packaging/rpm/forgejo/soda-forgejo.spec", "packaging/rpm/runtime/sources/sysusers/soda.conf", "packaging/bootc/trust/policy.json", "packaging/bootc/trust/registries.d.yaml", "distro/locks/release-tools.toml", "distro/locks/forgejo-source.toml"} {
		if !isFile(b.path(path)) {
			return fmt.Errorf("required bootc build input %s is missing", path)
		}
	}
	return nil
}

func (b *Builder) BuildImage(ctx context.Context) error {
	if err := b.BuildRPMs(ctx); err != nil {
		return err
	}
	baseTag, err := PrepareLocalBootcBase(ctx, b.Root, b.runner, b.Spec.Platform)
	if err != nil {
		return err
	}
	if err := b.stageReleaseTrust(); err != nil {
		return err
	}
	revision, err := b.sourceRevision(ctx)
	if err != nil {
		return err
	}
	images := b.artifactPath("images")
	if err := os.MkdirAll(images, 0o755); err != nil {
		return err
	}
	output := filepath.Join(images, "soda-os-"+b.Spec.Identity.Version+"-"+b.Spec.Platform.Architecture.Artifact+".oci.tar")
	if err := os.Remove(output); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	created := time.Unix(b.Spec.Build.SourceDateEpoch, 0).UTC().Format(time.RFC3339)
	args := []string{
		"buildx", "build", "--platform", b.Spec.Base.Platform,
		"--build-context", "fedora-base=docker-image://" + baseTag,
		"--file", "packaging/bootc/Containerfile",
		"--tag", b.Spec.Image.Registry + ":" + b.Spec.Identity.Version,
		"--build-arg", "SODA_VERSION=" + b.Spec.Identity.Version,
		"--build-arg", "SODA_SOURCE_REVISION=" + revision,
		"--build-arg", "SOURCE_DATE_EPOCH=" + fmt.Sprint(b.Spec.Build.SourceDateEpoch),
		"--build-arg", "SODA_CREATED=" + created,
		"--build-arg", "FEDORA_BASE_REFERENCE=" + b.Spec.Base.Reference,
		"--build-arg", "BOOTC_NEVRA=" + b.Spec.Platform.Base.BootcNEVRA,
		"--provenance=false",
		"--output", "type=oci,dest=" + output + ",oci-mediatypes=true,rewrite-timestamp=true",
		".",
	}
	if err := b.runner.Run(ctx, process.Command{Dir: b.Root, Name: "docker", Args: args}); err != nil {
		return err
	}
	if err := b.lintImage(ctx, output); err != nil {
		return err
	}
	fmt.Printf("Built OCI archive %s from %s\n", output, b.Spec.Base.Reference)
	return nil
}

func (b *Builder) lintImage(ctx context.Context, archive string) error {
	if err := b.runner.Run(ctx, process.Command{Dir: b.Root, Name: "docker", Args: []string{"load", "--input", archive}}); err != nil {
		return fmt.Errorf("load Soda image for bootc lint: %w", err)
	}
	reference := b.Spec.Image.Registry + ":" + b.Spec.Identity.Version
	if err := b.runner.Run(ctx, process.Command{Dir: b.Root, Name: "docker", Args: []string{"run", "--rm", "--platform", b.Spec.Base.Platform, "--entrypoint", "bootc", reference, "container", "lint"}}); err != nil {
		return fmt.Errorf("bootc container lint: %w", err)
	}
	return nil
}

func (b *Builder) stageReleaseTrust() error {
	publicKey, err := os.ReadFile(b.SigningPublicKey)
	if err != nil {
		return fmt.Errorf("read cosign.pub: %w", err)
	}
	destination := b.artifactPath("bootc", "trust")
	if err := recreate(destination); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(destination, "cosign.pub"), publicKey, 0o644); err != nil {
		return err
	}
	distribution, err := json.Marshal(b.Spec.Distribution)
	if err != nil {
		return fmt.Errorf("encode release distribution: %w", err)
	}
	return os.WriteFile(filepath.Join(destination, "distribution.json"), append(distribution, '\n'), 0o644)
}

func (b *Builder) sourceRevision(ctx context.Context) (string, error) {
	status, err := b.runner.Output(ctx, process.Command{Dir: b.Root, Name: "git", Args: []string{"status", "--porcelain=v1", "--untracked-files=all"}})
	if err != nil {
		return "", fmt.Errorf("inspect source worktree: %w", err)
	}
	if strings.TrimSpace(status) != "" {
		return "", errors.New("release artifact builds require a clean Git worktree; commit or remove tracked, staged, and untracked source changes")
	}
	revision, err := b.runner.Output(ctx, process.Command{Dir: b.Root, Name: "git", Args: []string{"rev-parse", "HEAD"}})
	if err != nil {
		return "", fmt.Errorf("resolve source revision: %w", err)
	}
	revision = strings.TrimSpace(revision)
	if len(revision) != 40 {
		return "", fmt.Errorf("source revision %q is not a full Git commit ID", revision)
	}
	if _, err := hex.DecodeString(revision); err != nil || revision != strings.ToLower(revision) {
		return "", fmt.Errorf("source revision %q is not a full Git commit ID", revision)
	}
	return revision, nil
}

func (b *Builder) packageLock() (packageLock, error) {
	var lock packageLock
	if _, err := toml.DecodeFile(b.path(b.Spec.Image.PackageLock), &lock); err != nil {
		return packageLock{}, fmt.Errorf("parse package lock: %w", err)
	}
	return lock, nil
}

func (b *Builder) releaseToolLock() (releaseToolLock, error) {
	var lock releaseToolLock
	if _, err := toml.DecodeFile(b.path("distro/locks/release-tools.toml"), &lock); err != nil {
		return releaseToolLock{}, fmt.Errorf("parse release tool lock: %w", err)
	}
	return lock, nil
}

func (l releaseToolLock) checksum(osName, architecture string) string {
	for _, binary := range l.Binary {
		if binary.OS == osName && binary.Arch == architecture {
			return binary.SHA256
		}
	}
	return ""
}

func (b *Builder) path(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(b.Root, path)
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func recreate(path string) error {
	if err := os.RemoveAll(path); err != nil {
		return err
	}
	return os.MkdirAll(path, 0o755)
}

func copyFile(source, destination string) error {
	in, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("copy %s: %w", source, err)
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func findSingleRPM(root, name string) (string, error) {
	var matches []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".rpm") && strings.HasPrefix(entry.Name(), name+"-") {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(matches) != 1 {
		return "", fmt.Errorf("expected one %s RPM, found %d", name, len(matches))
	}
	return matches[0], nil
}
