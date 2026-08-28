// Package image builds Soda's pinned Fedora bootc OCI runtime image.
package image

import (
	"context"
	"encoding/hex"
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
	bootcBaseReference   = "quay.io/fedora/fedora-bootc@sha256:85677d47c03b2e1f8f9a3a19d838023ea154229817d579d4b4da5b87a21c9c1a"
	bootcPlatform        = "linux/arm64"
	bootcRuntimeNEVRA    = "bootc-0:1.16.10-1.fc44.aarch64"
	bootcBaseArchive     = "distro/base/fedora-bootc-44-aarch64/Fedora-bootc-44.20260826.0-aarch64.oci-archive.tar"
	builderBaseReference = "registry.fedoraproject.org/fedora@sha256:9c8b291e256262b91aac5b3da50ea323760d0a6b449c6d6ad5f01d9550d48d2a"
	sodaRegistry         = "registry.soda.local/soda/os"
	cosignVersion        = "v3.1.2"
	cosignArm64SHA256    = "90e7ae0b5dfd60f20816b52c012addf7fc055ebcc7bea4ce81c428ca8518c302"
)

var targetRPMs = []string{"soda-release", "soda-runtime", "soda-cockpit"}

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

// Builder owns no system state. It writes only disposable OCI build artifacts
// below Root/.artifacts; BuildImage never pushes or loads an image.
type Builder struct {
	Root             string
	Spec             config.DistroSpec
	RegistryCA       string
	SigningPublicKey string
	runner           process.Runner
}

func NewBuilderFromWorkingDirectory(specPath string, runner process.Runner) (*Builder, error) {
	root, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}
	return NewBuilder(root, specPath, runner)
}

func NewBuilder(root, specPath string, runner process.Runner) (*Builder, error) {
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("canonicalize workspace: %w", err)
	}
	if !filepath.IsAbs(specPath) {
		specPath = filepath.Join(canonicalRoot, specPath)
	}
	spec, err := config.LoadDistro(specPath)
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

// Check validates the pinned bootc runtime contract without accessing a
// registry, starting a container, or creating an artifact.
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
		validateReleaseToolLock(toolLock),
		b.validateBuildInputs(),
	)
}

func validateImageSpec(spec config.DistroSpec) error {
	if spec.Identity.Architecture != "aarch64" || spec.Base.Reference != bootcBaseReference || spec.Base.Platform != bootcPlatform || spec.Image.Registry != sodaRegistry || spec.Image.StateSchema != 2 || spec.Build.SourceDateEpoch < 0 {
		return errors.New("Soda image specification differs from the approved AArch64 bootc contract")
	}
	return nil
}

func validateRuntimePackageLock(lock packageLock, spec config.DistroSpec) error {
	if lock.SchemaVersion != 1 || lock.BaseReference != spec.Base.Reference || len(lock.Package) <= len(targetRPMs) {
		return errors.New("package lock does not bind the configured Fedora bootc base")
	}
	local, bootcLocked, err := classifyRuntimePackages(lock.Package)
	if err != nil {
		return err
	}
	if !bootcLocked || strings.Join(local, ",") != strings.Join(targetRPMs, ",") {
		return errors.New("package lock must contain the locked bootc package and exactly the three Soda RPM inputs")
	}
	return nil
}

func classifyRuntimePackages(packages []lockedPackage) ([]string, bool, error) {
	seen, local, bootcLocked := make(map[string]bool, len(packages)), make([]string, 0, len(targetRPMs)), false
	for _, item := range packages {
		if err := validateLockedPackage(item, seen); err != nil {
			return nil, false, err
		}
		if item.Source == "local-rpm" {
			local = append(local, item.Name)
		}
		bootcLocked = bootcLocked || item.Name == "bootc" && item.NEVRA == bootcRuntimeNEVRA
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

func validateReleaseToolLock(lock releaseToolLock) error {
	if lock.Version != cosignVersion || lock.checksum("linux", "arm64") != cosignArm64SHA256 {
		return errors.New("release tool lock must pin the approved Cosign v3.1.2 Linux/AArch64 binary")
	}
	return nil
}

func (b *Builder) validateBuildInputs() error {
	for _, path := range []string{"packaging/bootc/Containerfile", "packaging/builder/Containerfile", "distro/locks/builder-packages.toml", "packaging/rpm/soda-release.spec", "packaging/rpm/soda-runtime.spec", "packaging/rpm/soda-cockpit.spec", "packaging/sysusers.d/soda.conf", "packaging/release/policy.json", "packaging/release/registries.d.yaml", "distro/locks/release-tools.toml"} {
		if !isFile(b.path(path)) {
			return fmt.Errorf("required bootc build input %s is missing", path)
		}
	}
	return nil
}

// BuildImage emits a local OCI archive. It deliberately omits --push and
// --load: publication and local container storage are separate operations.
func (b *Builder) BuildImage(ctx context.Context) error {
	if err := b.BuildRPMs(ctx); err != nil {
		return err
	}
	baseTag, err := PrepareLocalBootcBase(ctx, b.Root, b.runner, b.Spec.Base.Reference)
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
	output := filepath.Join(images, "soda-os-"+b.Spec.Identity.Version+"-aarch64.oci.tar")
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
		"--provenance=false",
		"--output", "type=oci,dest=" + output + ",oci-mediatypes=true,rewrite-timestamp=true",
		".",
	}
	if err := b.runner.Run(ctx, process.Command{Dir: b.Root, Name: "docker", Args: args}); err != nil {
		return err
	}
	fmt.Printf("Built OCI archive %s from %s\n", output, b.Spec.Base.Reference)
	return nil
}

func (b *Builder) stageReleaseTrust() error {
	registryCA, err := os.ReadFile(b.RegistryCA)
	if err != nil {
		return fmt.Errorf("read registry-ca.crt: %w", err)
	}
	publicKey, err := os.ReadFile(b.SigningPublicKey)
	if err != nil {
		return fmt.Errorf("read cosign.pub: %w", err)
	}
	destination := b.artifactPath("bootc", "trust")
	if err := recreate(destination); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(destination, "registry-ca.crt"), registryCA, 0o644); err != nil {
		return fmt.Errorf("stage registry-ca.crt: %w", err)
	}
	return os.WriteFile(filepath.Join(destination, "cosign.pub"), publicKey, 0o644)
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
