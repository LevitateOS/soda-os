// Package image builds Soda's pinned Fedora bootc OCI runtime image.
package image

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
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
)

const (
	bootcBaseReference = "quay.io/fedora/fedora-bootc@sha256:85677d47c03b2e1f8f9a3a19d838023ea154229817d579d4b4da5b87a21c9c1a"
	bootcPlatform      = "linux/arm64"
	bootcRuntimeNEVRA  = "bootc-0:1.16.10-1.fc44.aarch64"
	sodaRegistry       = "registry.soda.local/soda/os"
	cosignVersion      = "v3.1.2"
	cosignArm64SHA256  = "90e7ae0b5dfd60f20816b52c012addf7fc055ebcc7bea4ce81c428ca8518c302"
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

type rpmInventory struct {
	SchemaVersion    uint32             `json:"schema_version"`
	BaseReference    string             `json:"base_reference"`
	Platform         string             `json:"platform"`
	SodaVersion      string             `json:"soda_version"`
	SourceRevision   string             `json:"source_revision"`
	SourceDateEpoch  int64              `json:"source_date_epoch"`
	DirectRPMPackage []rpmInventoryItem `json:"direct_rpm_packages"`
}

type rpmInventoryItem struct {
	Name   string `json:"name"`
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
}

// Builder owns no system state. It writes only disposable OCI build artifacts
// below Root/.artifacts; BuildImage never pushes or loads an image.
type Builder struct {
	Root             string
	Spec             config.DistroSpec
	RegistryCA       string
	SigningPublicKey string
	runner           Runner
}

func NewBuilderFromWorkingDirectory(specPath string, runner Runner) (*Builder, error) {
	root, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}
	return NewBuilder(root, specPath, runner)
}

func NewBuilder(root, specPath string, runner Runner) (*Builder, error) {
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
		runner = OSRunner{}
	}
	return &Builder{Root: canonicalRoot, Spec: spec, runner: runner}, nil
}

func (b *Builder) artifactPath(parts ...string) string {
	return filepath.Join(append([]string{b.Root, ".artifacts"}, parts...)...)
}

func (b *Builder) imageName() string {
	return b.Spec.Image.Registry + ":" + b.Spec.Identity.Version
}

// Check validates the pinned bootc runtime contract without accessing a
// registry, starting a container, or creating an artifact.
func (b *Builder) Check(_ context.Context) error {
	spec := b.Spec
	if spec.Identity.Architecture != "aarch64" {
		return errors.New("only AArch64 bootc image builds are supported")
	}
	if spec.Base.Reference != bootcBaseReference {
		return errors.New("Fedora bootc base reference differs from the approved digest")
	}
	if spec.Base.Platform != bootcPlatform {
		return fmt.Errorf("bootc platform must be %s", bootcPlatform)
	}
	if spec.Image.Registry != sodaRegistry {
		return fmt.Errorf("Soda image registry must be %s", sodaRegistry)
	}
	if spec.Image.StateSchema != 2 {
		return errors.New("Soda runtime state schema must be 2")
	}
	if spec.Build.SourceDateEpoch < 0 {
		return errors.New("SOURCE_DATE_EPOCH must be non-negative")
	}
	lock, err := b.packageLock()
	if err != nil {
		return err
	}
	if lock.SchemaVersion != 1 || lock.BaseReference != spec.Base.Reference {
		return errors.New("package lock does not bind the configured Fedora bootc base")
	}
	if len(lock.Package) <= len(targetRPMs) {
		return errors.New("package lock must contain Fedora dependencies and the three Soda RPM inputs")
	}
	seen := make(map[string]bool, len(lock.Package))
	var local []string
	bootcLocked := false
	for _, item := range lock.Package {
		if item.Name == "" || item.NEVRA == "" || seen[item.Name] {
			return errors.New("package lock contains an empty or duplicate package")
		}
		seen[item.Name] = true
		switch item.Source {
		case "fedora":
			if item.File != "" {
				return errors.New("Fedora package lock entries must not name local files")
			}
			if item.Name == "bootc" {
				if item.NEVRA != bootcRuntimeNEVRA {
					return fmt.Errorf("bootc package lock must pin %s", bootcRuntimeNEVRA)
				}
				bootcLocked = true
			}
		case "local-rpm":
			if item.File == "" || filepath.Base(item.File) != item.File {
				return errors.New("local package lock entries require a plain RPM filename")
			}
			local = append(local, item.Name)
		default:
			return fmt.Errorf("unsupported package source %q", item.Source)
		}
	}
	if !bootcLocked {
		return fmt.Errorf("package lock must include %s from Fedora", bootcRuntimeNEVRA)
	}
	toolLock, err := b.releaseToolLock()
	if err != nil {
		return err
	}
	if toolLock.Version != cosignVersion || toolLock.checksum("linux", "arm64") != cosignArm64SHA256 {
		return errors.New("release tool lock must pin the approved Cosign v3.1.2 Linux/AArch64 binary")
	}
	if strings.Join(local, ",") != strings.Join(targetRPMs, ",") {
		return errors.New("package lock must contain exactly the three Soda RPM build inputs")
	}
	for _, path := range []string{
		"packaging/bootc/Containerfile",
		"packaging/builder/Containerfile",
		"packaging/rpm/soda-release.spec",
		"packaging/rpm/soda-runtime.spec",
		"packaging/rpm/soda-cockpit.spec",
		"packaging/sysusers.d/soda.conf",
		"packaging/release/policy.json",
		"packaging/release/registries.d.yaml",
		"packaging/release/tools.lock",
	} {
		if !isFile(b.path(path)) {
			return fmt.Errorf("required bootc build input %s is missing", path)
		}
	}
	return nil
}

// BuildRPMs builds exactly the three direct Soda RPM inputs and records their
// names and SHA-256 values. Runtime dependencies resolve from the pinned base.
func (b *Builder) BuildRPMs(ctx context.Context) error {
	if err := b.Check(ctx); err != nil {
		return err
	}
	revision, err := b.sourceRevision(ctx)
	if err != nil {
		return err
	}
	if err := b.verifyRuntimeCosign(); err != nil {
		return err
	}
	if err := b.buildContainer(ctx); err != nil {
		return err
	}
	build := b.artifactPath("build")
	topdir := b.artifactPath("rpmbuild")
	rpms := b.artifactPath("rpms")
	for _, path := range []string{build, topdir, rpms} {
		if err := recreate(path); err != nil {
			return err
		}
	}
	for _, directory := range []string{"BUILD", "BUILDROOT", "RPMS", "SOURCES", "SPECS", "SRPMS"} {
		if err := os.MkdirAll(filepath.Join(topdir, directory), 0o755); err != nil {
			return err
		}
	}
	if err := b.buildGoBinaries(ctx, revision); err != nil {
		return err
	}
	if err := b.stageRPMSources(build, filepath.Join(topdir, "SOURCES")); err != nil {
		return err
	}
	for _, name := range targetRPMs {
		if err := b.rpmbuild(ctx, name); err != nil {
			return err
		}
		rpm, err := findSingleRPM(filepath.Join(topdir, "RPMS"), name)
		if err != nil {
			return err
		}
		if err := copyFile(rpm, filepath.Join(rpms, filepath.Base(rpm))); err != nil {
			return err
		}
	}
	if err := b.writeRPMInventory(rpms, revision); err != nil {
		return err
	}
	if err := b.writeLockedInstallInputs(rpms); err != nil {
		return err
	}
	fmt.Printf("Built locked Soda RPM inputs at %s\n", rpms)
	return nil
}

// BuildImage emits a local OCI archive. It deliberately omits --push and
// --load: publication and local container storage are separate operations.
func (b *Builder) BuildImage(ctx context.Context) error {
	if _, err := b.sourceRevision(ctx); err != nil {
		return err
	}
	if err := b.BuildRPMs(ctx); err != nil {
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
		"--file", "packaging/bootc/Containerfile",
		"--tag", b.imageName(),
		"--build-arg", "SODA_VERSION=" + b.Spec.Identity.Version,
		"--build-arg", "SODA_SOURCE_REVISION=" + revision,
		"--build-arg", "SOURCE_DATE_EPOCH=" + fmt.Sprint(b.Spec.Build.SourceDateEpoch),
		"--build-arg", "SODA_CREATED=" + created,
		"--provenance=false",
		"--output", "type=oci,dest=" + output + ",oci-mediatypes=true,rewrite-timestamp=true",
		".",
	}
	if err := b.runner.Run(ctx, Command{Dir: b.Root, Name: "docker", Args: args}); err != nil {
		return err
	}
	fmt.Printf("Built OCI archive %s from %s\n", output, b.Spec.Base.Reference)
	return nil
}

func (b *Builder) stageReleaseTrust() error {
	ca, err := os.ReadFile(b.RegistryCA)
	if err != nil {
		return fmt.Errorf("read registry CA build input: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(ca) {
		return errors.New("registry CA build input is not a PEM certificate")
	}
	publicKey, err := os.ReadFile(b.SigningPublicKey)
	if err != nil {
		return fmt.Errorf("read signing public key build input: %w", err)
	}
	block, rest := pem.Decode(publicKey)
	if block == nil || block.Type != "PUBLIC KEY" || len(strings.TrimSpace(string(rest))) != 0 {
		return errors.New("signing public key build input is not one PEM public key")
	}
	if _, err := x509.ParsePKIXPublicKey(block.Bytes); err != nil {
		return fmt.Errorf("parse signing public key build input: %w", err)
	}
	destination := b.artifactPath("bootc", "trust")
	if err := recreate(destination); err != nil {
		return err
	}
	if err := copyFile(b.RegistryCA, filepath.Join(destination, "registry-ca.crt")); err != nil {
		return err
	}
	if err := copyFile(b.SigningPublicKey, filepath.Join(destination, "cosign.pub")); err != nil {
		return err
	}
	return nil
}

func (b *Builder) sourceRevision(ctx context.Context) (string, error) {
	status, err := b.runner.Output(ctx, Command{Dir: b.Root, Name: "git", Args: []string{"status", "--porcelain=v1", "--untracked-files=all"}})
	if err != nil {
		return "", fmt.Errorf("inspect source worktree: %w", err)
	}
	if strings.TrimSpace(status) != "" {
		return "", errors.New("release artifact builds require a clean Git worktree; commit or remove tracked, staged, and untracked source changes")
	}
	revision, err := b.runner.Output(ctx, Command{Dir: b.Root, Name: "git", Args: []string{"rev-parse", "HEAD"}})
	if err != nil {
		return "", fmt.Errorf("resolve source revision: %w", err)
	}
	revision = strings.TrimSpace(revision)
	if len(revision) != 40 || !hexDigest(revision) {
		return "", fmt.Errorf("source revision %q is not a full Git commit ID", revision)
	}
	return revision, nil
}

func (b *Builder) buildGoBinaries(ctx context.Context, revision string) error {
	buildDate := time.Unix(b.Spec.Build.SourceDateEpoch, 0).UTC().Format(time.RFC3339)
	linkerFlags := strings.Join([]string{
		"-s", "-w", "-buildid=",
		"-X github.com/LevitateOS/soda-os/internal/version.Version=" + b.Spec.Identity.Version,
		"-X github.com/LevitateOS/soda-os/internal/version.Commit=" + revision,
		"-X github.com/LevitateOS/soda-os/internal/version.BuildDate=" + buildDate,
	}, " ")
	for _, target := range []struct{ output, pkg string }{
		{"sodad", "./cmd/sodad"},
		{"sodactl", "./cmd/sodactl"},
		{"soda-ssh", "./cmd/soda-ssh"},
		{"soda-cockpit", "./cockpit/cmd/soda-cockpit"},
		{"soda-authd", "./cockpit/cmd/soda-authd"},
	} {
		if err := b.docker(ctx, []string{"CGO_ENABLED=1", "SOURCE_DATE_EPOCH=" + fmt.Sprint(b.Spec.Build.SourceDateEpoch)}, "go", "build", "-buildvcs=false", "-trimpath", "-ldflags="+linkerFlags, "-o", "/src/.artifacts/build/"+target.output, target.pkg); err != nil {
			return err
		}
	}
	return nil
}

func (b *Builder) stageRPMSources(build, sources string) error {
	files := [][2]string{
		{filepath.Join(build, "sodad"), filepath.Join(sources, "sodad")},
		{filepath.Join(build, "sodactl"), filepath.Join(sources, "sodactl")},
		{b.artifactPath("tools", "cosign-linux-arm64"), filepath.Join(sources, "cosign")},
		{filepath.Join(build, "soda-ssh"), filepath.Join(sources, "soda-ssh")},
		{filepath.Join(build, "soda-cockpit"), filepath.Join(sources, "soda-cockpit")},
		{filepath.Join(build, "soda-authd"), filepath.Join(sources, "soda-authd")},
		{b.path("packaging/systemd/sodad.service"), filepath.Join(sources, "sodad.service")},
		{b.path("packaging/systemd/srv-soda-projects.mount"), filepath.Join(sources, "srv-soda-projects.mount")},
		{b.path("packaging/systemd/opt-soda-toolchains.mount"), filepath.Join(sources, "opt-soda-toolchains.mount")},
		{b.path("packaging/systemd/90-soda.preset"), filepath.Join(sources, "90-soda.preset")},
		{b.path("packaging/tmpfiles.d/soda.conf"), filepath.Join(sources, "soda.conf")},
		{b.path("packaging/sysusers.d/soda.conf"), filepath.Join(sources, "soda.sysusers")},
		{b.path("packaging/sshd/41-soda-project-accounts.conf"), filepath.Join(sources, "41-soda-project-accounts.conf")},
		{b.path("packaging/systemd/soda-cockpit.service"), filepath.Join(sources, "soda-cockpit.service")},
		{b.path("packaging/systemd/soda-authd.service"), filepath.Join(sources, "soda-authd.service")},
		{b.path("packaging/avahi/soda-cockpit.service"), filepath.Join(sources, "soda-cockpit.avahi.service")},
		{b.path("packaging/pam/soda-cockpit"), filepath.Join(sources, "soda-cockpit.pam")},
		{b.path("packaging/bootc/BASE_SYSTEM.md"), filepath.Join(sources, "BASE_SYSTEM.md")},
		{b.path("assets/branding/source/soda-symbol.svg"), filepath.Join(sources, "soda-symbol.svg")},
	}
	for _, size := range []string{"16", "24", "32", "48", "64", "128", "256", "512"} {
		files = append(files, [2]string{b.path("assets/branding/icons/hicolor/" + size + "x" + size + "/apps/soda-os.png"), filepath.Join(sources, "soda-os-"+size+".png")})
	}
	for _, pair := range files {
		if err := copyFile(pair[0], pair[1]); err != nil {
			return err
		}
	}
	return nil
}

func (b *Builder) writeRPMInventory(rpms, revision string) error {
	entries, err := os.ReadDir(rpms)
	if err != nil {
		return err
	}
	byName := make(map[string]string, len(targetRPMs))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".rpm") {
			continue
		}
		for _, name := range targetRPMs {
			if strings.HasPrefix(entry.Name(), name+"-") {
				if _, exists := byName[name]; exists {
					return fmt.Errorf("found more than one %s RPM", name)
				}
				byName[name] = entry.Name()
			}
		}
	}
	inventory := rpmInventory{SchemaVersion: 1, BaseReference: b.Spec.Base.Reference, Platform: b.Spec.Base.Platform, SodaVersion: b.Spec.Identity.Version, SourceRevision: revision, SourceDateEpoch: b.Spec.Build.SourceDateEpoch}
	for _, name := range targetRPMs {
		file, found := byName[name]
		if !found {
			return fmt.Errorf("locked RPM input %s is missing", name)
		}
		digest, err := sha256File(filepath.Join(rpms, file))
		if err != nil {
			return err
		}
		inventory.DirectRPMPackage = append(inventory.DirectRPMPackage, rpmInventoryItem{Name: name, File: file, SHA256: digest})
	}
	encoded, err := json.MarshalIndent(inventory, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(b.artifactPath("metadata"), 0o755); err != nil {
		return err
	}
	return os.WriteFile(b.artifactPath("metadata", "added-rpms.json"), append(encoded, '\n'), 0o644)
}

func (b *Builder) writeLockedInstallInputs(rpms string) error {
	lock, err := b.packageLock()
	if err != nil {
		return err
	}
	directory := b.artifactPath("bootc")
	if err := recreate(directory); err != nil {
		return err
	}
	var fedora, installed []string
	for _, item := range lock.Package {
		installed = append(installed, item.NEVRA)
		if item.Source == "fedora" {
			fedora = append(fedora, item.NEVRA)
			continue
		}
		if !isFile(filepath.Join(rpms, item.File)) {
			return fmt.Errorf("locked local RPM %s is missing", item.File)
		}
	}
	for path, lines := range map[string][]string{
		"fedora-packages.txt":   fedora,
		"expected-packages.txt": installed,
	} {
		if err := os.WriteFile(filepath.Join(directory, path), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
			return err
		}
	}
	return nil
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
	if _, err := toml.DecodeFile(b.path("packaging/release/tools.lock"), &lock); err != nil {
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

func (b *Builder) verifyRuntimeCosign() error {
	path := b.artifactPath("tools", "cosign-linux-arm64")
	if !isFile(path) {
		return errors.New("pinned Linux/AArch64 Cosign input is missing; run just release-tools")
	}
	digest, err := sha256File(path)
	if err != nil {
		return err
	}
	if digest != cosignArm64SHA256 {
		return fmt.Errorf("Linux/AArch64 Cosign SHA-256 %s differs from pinned %s", digest, cosignArm64SHA256)
	}
	return nil
}

func (b *Builder) buildContainer(ctx context.Context) error {
	return b.runner.Run(ctx, Command{Dir: b.Root, Name: "docker", Args: []string{"build", "--quiet", "--platform", b.Spec.Base.Platform, "--file", "packaging/builder/Containerfile", "--tag", "soda-os-rpm-builder:" + b.Spec.Identity.Version, "."}})
}

func (b *Builder) docker(ctx context.Context, environment []string, name string, args ...string) error {
	return b.runner.Run(ctx, b.dockerCommand(environment, name, args...))
}

func (b *Builder) dockerCommand(environment []string, name string, args ...string) Command {
	dockerArgs := []string{"run", "--rm", "--platform", b.Spec.Base.Platform, "--volume", b.Root + ":/src", "--workdir", "/src"}
	for _, pair := range environment {
		dockerArgs = append(dockerArgs, "--env", pair)
	}
	dockerArgs = append(dockerArgs, "soda-os-rpm-builder:"+b.Spec.Identity.Version, name)
	dockerArgs = append(dockerArgs, args...)
	return Command{Dir: b.Root, Name: "docker", Args: dockerArgs}
}

func (b *Builder) rpmbuild(ctx context.Context, name string) error {
	epoch := fmt.Sprint(b.Spec.Build.SourceDateEpoch)
	return b.docker(ctx, []string{"SOURCE_DATE_EPOCH=" + epoch}, "rpmbuild", "-bb",
		"--define", "_topdir /src/.artifacts/rpmbuild",
		"--define", "_source_date_epoch "+epoch,
		"--define", "use_source_date_epoch_as_buildtime 1",
		"--define", "_buildhost soda-builder",
		"packaging/rpm/"+name+".spec")
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

func hexDigest(value string) bool {
	for _, character := range value {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return false
		}
	}
	return true
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

func sha256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
