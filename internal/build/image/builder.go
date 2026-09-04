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
	"runtime"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/LevitateOS/soda-os/internal/config"
	"github.com/LevitateOS/soda-os/internal/process"
)

const sodaRegistry = "ghcr.io/levitateos/soda-os"

var builtRPMs = []string{"soda-release", "soda-runtime", "soda-projects", "soda-forgejo", "soda-tea"}
var externalRPMs = []string{"mise"}

var requiredStockCockpitPackages = []string{
	"cockpit-bridge",
	"cockpit-networkmanager",
	"cockpit-storaged",
	"cockpit-system",
	"cockpit-ws",
	"cockpit-ws-selinux",
}

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

type runtimePackageSources struct {
	Built       []string
	External    []string
	BootcLocked bool
}

type Builder struct {
	Root             string
	Spec             config.DistroSpec
	hostArchitecture string
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
	return &Builder{Root: canonicalRoot, Spec: spec, hostArchitecture: runtime.GOARCH, runner: runner}, nil
}

func (b *Builder) requireNativeHost() error {
	hostArchitecture := b.hostArchitecture
	if hostArchitecture == "" {
		hostArchitecture = runtime.GOARCH
	}
	return config.RequireNativeHostArchitecture(b.Spec.Platform.Architecture.Name, hostArchitecture)
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
	return errors.Join(
		validateImageSpec(spec),
		validateRuntimePackageLock(lock, spec),
		b.validateMiseRuntimeInput(lock),
		validateStockCockpitLockClosure(lock),
		b.validateBuildInputs(),
	)
}

func (b *Builder) validateMiseRuntimeInput(runtime packageLock) error {
	lock, err := readMiseSourceLock(b.path("distro/locks/mise-source.toml"))
	if err != nil {
		return err
	}
	expected, err := lock.runtimePackage(b.Spec.Platform.Architecture.Name)
	if err != nil {
		return err
	}
	for _, item := range runtime.Package {
		if item.Name == "mise" {
			if item != expected {
				return errors.New("runtime package lock differs from the reviewed mise input")
			}
			return nil
		}
	}
	return errors.New("runtime package lock is missing the reviewed mise input")
}

func validateImageSpec(spec config.DistroSpec) error {
	if spec.Identity.Architecture != spec.Platform.Architecture.Name || spec.Base.Reference != spec.Platform.Base.Reference || spec.Base.Platform != spec.Platform.Architecture.Platform || spec.Image.Registry != sodaRegistry || spec.Build.SourceDateEpoch < 0 {
		return errors.New("Soda image specification differs from the selected architecture contract")
	}
	return nil
}

func validateRuntimePackageLock(lock packageLock, spec config.DistroSpec) error {
	if lock.SchemaVersion != 1 || lock.BaseReference != spec.Base.Reference || len(lock.Package) <= len(builtRPMs)+len(externalRPMs) {
		return errors.New("package lock does not bind the configured Fedora bootc base")
	}
	sources, err := classifyRuntimePackages(lock.Package, spec.Platform.Base.BootcNEVRA)
	if err != nil {
		return err
	}
	if !sources.BootcLocked || strings.Join(sources.Built, ",") != strings.Join(builtRPMs, ",") || strings.Join(sources.External, ",") != strings.Join(externalRPMs, ",") {
		return errors.New("package lock must contain the locked bootc package, Soda RPM inputs, and reviewed external RPM inputs")
	}
	return nil
}

func validateStockCockpitLockClosure(lock packageLock) error {
	locked := make(map[string]bool, len(lock.Package))
	for _, item := range lock.Package {
		locked[item.Name] = true
	}
	missing := make([]string, 0, len(requiredStockCockpitPackages))
	for _, name := range requiredStockCockpitPackages {
		if !locked[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) != 0 {
		return fmt.Errorf("runtime package lock requires matching-native resolution for: %s", strings.Join(missing, ", "))
	}
	return nil
}

func classifyRuntimePackages(packages []lockedPackage, bootcNEVRA string) (runtimePackageSources, error) {
	seen := make(map[string]bool, len(packages))
	sources := runtimePackageSources{Built: make([]string, 0, len(builtRPMs)), External: make([]string, 0, len(externalRPMs))}
	for _, item := range packages {
		if err := validateLockedPackage(item, seen); err != nil {
			return runtimePackageSources{}, err
		}
		if item.Source == "local-rpm" {
			sources.Built = append(sources.Built, item.Name)
		}
		if item.Source == "external-rpm" {
			sources.External = append(sources.External, item.Name)
		}
		sources.BootcLocked = sources.BootcLocked || item.Name == "bootc" && item.NEVRA == bootcNEVRA
	}
	return sources, nil
}

func validateLockedPackage(item lockedPackage, seen map[string]bool) error {
	if item.Name == "" || item.NEVRA == "" || seen[item.Name] {
		return errors.New("package lock contains an empty or duplicate package")
	}
	seen[item.Name] = true
	if item.Source == "fedora" && item.File == "" {
		return nil
	}
	if (item.Source == "local-rpm" || item.Source == "external-rpm") && item.File != "" && filepath.Base(item.File) == item.File {
		return nil
	}
	return fmt.Errorf("package lock entry %s has an unsupported source or file", item.Name)
}

func (b *Builder) validateBuildInputs() error {
	for _, path := range []string{"packaging/bootc/Containerfile", "packaging/builder/Containerfile", b.Spec.Platform.Builder.PackageLock, b.Spec.Platform.Installer.PackageLock, b.Spec.Platform.Installer.ToolLock, b.Spec.Platform.Installer.ISOConfig, "packaging/rpm/release/soda-release.spec", "packaging/rpm/runtime/soda-runtime.spec", "packaging/rpm/projects/soda-projects.spec", "packaging/rpm/forgejo/soda-forgejo.spec", "packaging/rpm/forgejo/sources/patches/0001-pam-do-not-retain-password.patch", "packaging/rpm/tea/soda-tea.spec", "packaging/rpm/tea/sources/LICENSE", "distro/locks/forgejo-source.toml", "distro/locks/mise-source.toml", "distro/locks/tea-source.toml"} {
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
		"--build-context", "rpm-inputs=" + b.artifactPath("rpms"),
		"--build-context", "lock-inputs=" + b.artifactPath("bootc"),
		"--file", "packaging/bootc/Containerfile",
		"--tag", b.Spec.Image.Registry + ":" + b.Spec.Identity.Version,
		"--build-arg", "SODA_VERSION=" + b.Spec.Identity.Version,
		"--build-arg", "SODA_HOSTNAME=" + b.Spec.Identity.Hostname,
		"--build-arg", "SODA_SOURCE_REVISION=" + revision,
		"--build-arg", "SOURCE_DATE_EPOCH=" + fmt.Sprint(b.Spec.Build.SourceDateEpoch),
		"--build-arg", "SODA_CREATED=" + created,
		"--build-arg", "FEDORA_BASE_REFERENCE=" + b.Spec.Base.Reference,
		"--build-arg", "BOOTC_NEVRA=" + b.Spec.Platform.Base.BootcNEVRA,
		"--provenance=false",
		"--output", "type=oci,dest=" + output + ",oci-mediatypes=true,rewrite-timestamp=true",
		"packaging/bootc",
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
