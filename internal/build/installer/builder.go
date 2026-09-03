package installer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/LevitateOS/soda-os/internal/config"
	"github.com/LevitateOS/soda-os/internal/process"
)

const (
	Repository = "ghcr.io/levitateos/soda-os"
)

var exactImagePattern = regexp.MustCompile(`^ghcr\.io/levitateos/soda-os@sha256:[0-9a-f]{64}$`)

type toolLock struct {
	Version   string `toml:"version"`
	Commit    string `toml:"commit"`
	Reference string `toml:"reference"`
	Platform  string `toml:"platform"`
}

type packageLock struct {
	SchemaVersion uint32   `toml:"schema_version"`
	Platform      string   `toml:"platform"`
	Packages      []string `toml:"packages"`
	BootPackages  []string `toml:"boot_packages"`
	EFIVendor     string   `toml:"efi_vendor"`
}

type Options struct {
	ArchivePath string
	ToolLock    string
	OutputDir   string
}

func (b *Builder) ValidateISO(ctx context.Context, isoPath, reference, installerArchive, toolLockPath string) (string, error) {
	if err := b.requireNativeHost(); err != nil {
		return "", err
	}
	if !exactImagePattern.MatchString(reference) {
		return "", errors.New("installer payload must be an exact ghcr.io/levitateos/soda-os@sha256 reference")
	}
	if !regularFile(isoPath) || !regularFile(installerArchive) || !regularFile(toolLockPath) {
		return "", errors.New("ISO validation requires the ISO, installer environment archive, and image-builder lock")
	}
	lock, err := readToolLock(toolLockPath, b.Spec.Platform)
	if err != nil {
		return "", err
	}
	return b.inspectISOArtifact(ctx, isoPath, reference, installerArchive, lock)
}

func (b *Builder) inspectISOArtifact(ctx context.Context, isoPath, reference, installerArchive string, lock toolLock) (string, error) {
	workRoot := filepath.Join(b.Root, ".artifacts", "installer")
	if err := os.MkdirAll(workRoot, 0o755); err != nil {
		return "", err
	}
	inspectDir, err := os.MkdirTemp(workRoot, "iso-inspect-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(inspectDir)
	volumeName := fmt.Sprintf("soda-iso-inspect-%s-%d", strings.TrimPrefix(reference, Repository+"@sha256:")[:12], os.Getpid())
	if err := b.runner.Run(ctx, process.Command{Dir: b.Root, Name: "docker", Args: []string{"volume", "create", volumeName}}); err != nil {
		return "", fmt.Errorf("create disposable ISO inspection storage: %w", err)
	}
	defer func() {
		_ = b.runner.Run(context.Background(), process.Command{Dir: b.Root, Name: "docker", Args: []string{"volume", "rm", "--force", volumeName}})
	}()
	installerTag := "localhost/soda-installer-inspect:" + b.Spec.Identity.Version + "-" + b.Spec.Platform.Architecture.Artifact
	if err := b.copyToStorage(ctx, lock, volumeName, installerArchive, installerTag); err != nil {
		return "", err
	}
	if err := b.inspectISO(ctx, isoInspectionInput{lock: lock, volumeName: volumeName, installerTag: installerTag, isoPath: isoPath, inspectDir: inspectDir, reference: reference}); err != nil {
		return "", err
	}
	digest, err := fileSHA256(isoPath)
	if err != nil {
		return "", fmt.Errorf("checksum installer ISO: %w", err)
	}
	return digest, nil
}

type Builder struct {
	Root             string
	Spec             config.DistroSpec
	hostArchitecture string
	runner           process.Runner
}

func NewBuilder(root string, spec config.DistroSpec, runner process.Runner) *Builder {
	if runner == nil {
		runner = process.OSRunner{}
	}
	return &Builder{Root: root, Spec: spec, hostArchitecture: runtime.GOARCH, runner: runner}
}

func (b *Builder) requireNativeHost() error {
	hostArchitecture := b.hostArchitecture
	if hostArchitecture == "" {
		hostArchitecture = runtime.GOARCH
	}
	return config.RequireNativeHostArchitecture(b.Spec.Platform.Architecture.Name, hostArchitecture)
}

func (b *Builder) Build(ctx context.Context, options Options) (string, error) {
	if err := b.requireNativeHost(); err != nil {
		return "", err
	}
	lock, err := b.validate(options)
	if err != nil {
		return "", err
	}
	reference, err := archiveReference(options.ArchivePath, b.Spec.Platform.Architecture.OCI)
	if err != nil {
		return "", err
	}
	workspace, err := b.prepareInstallerWorkspace(options, reference)
	if err != nil {
		return "", err
	}
	volumeName := fmt.Sprintf("soda-installer-%s-%d", strings.TrimPrefix(reference, Repository+"@sha256:")[:12], os.Getpid())
	if err := b.runner.Run(ctx, process.Command{Dir: b.Root, Name: "docker", Args: []string{"volume", "create", volumeName}}); err != nil {
		return "", fmt.Errorf("create disposable installer container storage: %w", err)
	}
	defer func() {
		_ = b.runner.Run(context.Background(), process.Command{Dir: b.Root, Name: "docker", Args: []string{"volume", "rm", "--force", volumeName}})
	}()

	installerArchive, installerTag, err := b.buildInstallerEnvironment(ctx, workspace.work)
	if err != nil {
		return "", err
	}
	if err := b.copyToStorage(ctx, lock, volumeName, installerArchive, installerTag); err != nil {
		return "", err
	}

	return b.buildInstallerISO(ctx, isoBuildInput{lock: lock, volumeName: volumeName, installerTag: installerTag, workspace: workspace, reference: reference})
}

type installerWorkspace struct{ work, context, inspect, output string }

type isoBuildInput struct {
	lock                     toolLock
	volumeName, installerTag string
	workspace                installerWorkspace
	reference                string
}

func (b *Builder) prepareInstallerWorkspace(options Options, reference string) (installerWorkspace, error) {
	work := filepath.Join(b.Root, ".artifacts", "installer")
	output := options.OutputDir
	if output == "" {
		output = filepath.Join(b.Root, ".artifacts", "images")
	}
	workspace := installerWorkspace{work, filepath.Join(work, "context"), filepath.Join(work, "inspect"), output}
	if err := resetInstallerWorkspace(workspace); err != nil {
		return installerWorkspace{}, err
	}
	if err := b.stageInstallerConfiguration(workspace.context, reference); err != nil {
		return installerWorkspace{}, err
	}
	return workspace, nil
}

func resetInstallerWorkspace(workspace installerWorkspace) error {
	for _, path := range []string{workspace.context, workspace.inspect} {
		if err := os.RemoveAll(path); err != nil {
			return err
		}
		if err := os.MkdirAll(path, 0o755); err != nil {
			return err
		}
	}
	return os.MkdirAll(workspace.output, 0o755)
}

func (b *Builder) stageInstallerConfiguration(destination, reference string) error {
	if err := os.WriteFile(filepath.Join(destination, "interactive-defaults.ks"), []byte(kickstart(reference, b.Spec.Identity.Hostname)), 0o644); err != nil {
		return err
	}
	if err := b.stageInstallerPackageLock(destination); err != nil {
		return err
	}
	isoConfig := b.Spec.Platform.Installer.ISOConfig
	if !filepath.IsAbs(isoConfig) {
		isoConfig = filepath.Join(b.Root, isoConfig)
	}
	if err := copyFile(isoConfig, filepath.Join(destination, "iso.yaml")); err != nil {
		return fmt.Errorf("stage installer ISO configuration: %w", err)
	}
	return nil
}

func copyFile(source, destination string) error {
	contents, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return os.WriteFile(destination, contents, 0o644)
}

func (b *Builder) stageInstallerPackageLock(destination string) error {
	var lock packageLock
	lockPath := b.Spec.Platform.Installer.PackageLock
	if !filepath.IsAbs(lockPath) {
		lockPath = filepath.Join(b.Root, lockPath)
	}
	if _, err := toml.DecodeFile(lockPath, &lock); err != nil {
		return fmt.Errorf("read installer package lock: %w", err)
	}
	if lock.SchemaVersion != 1 || lock.Platform != b.Spec.Base.Platform || len(lock.Packages) == 0 || len(lock.BootPackages) == 0 || lock.EFIVendor == "" {
		return errors.New("installer package lock differs from the selected platform contract")
	}
	for name, values := range map[string][]string{"installer-packages.txt": lock.Packages, "installer-boot-packages.txt": lock.BootPackages} {
		if err := os.WriteFile(filepath.Join(destination, name), []byte(strings.Join(values, "\n")+"\n"), 0o644); err != nil {
			return err
		}
	}
	return os.WriteFile(filepath.Join(destination, "installer-efi-vendor.txt"), []byte(lock.EFIVendor+"\n"), 0o644)
}

func (b *Builder) buildInstallerEnvironment(ctx context.Context, work string) (string, string, error) {
	archive := filepath.Join(work, "soda-installer-environment-"+b.Spec.Platform.Architecture.Artifact+".oci.tar")
	if err := os.Remove(archive); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", "", err
	}
	tag := "localhost/soda-installer:" + b.Spec.Identity.Version + "-" + b.Spec.Platform.Architecture.Artifact
	args := []string{"buildx", "build", "--platform", b.Spec.Base.Platform, "--build-context", "installer-base=docker-image://" + b.Spec.Platform.Builder.BaseReference, "--file", "packaging/installer/Containerfile", "--tag", tag, "--provenance=false", "--output", "type=oci,dest=" + archive + ",oci-mediatypes=true", "."}
	if err := b.runner.Run(ctx, process.Command{Dir: b.Root, Name: "docker", Args: args}); err != nil {
		return "", "", fmt.Errorf("build installer environment: %w", err)
	}
	return archive, tag, nil
}

func (b *Builder) buildInstallerISO(ctx context.Context, input isoBuildInput) (string, error) {
	outputName := "SodaOS-" + b.Spec.Identity.Version + "-" + b.Spec.Platform.Architecture.Artifact
	for _, suffix := range []string{".iso", ".iso.sha256"} {
		if err := os.Remove(filepath.Join(input.workspace.output, outputName+suffix)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
	args := []string{"run", "--rm", "--platform", b.Spec.Base.Platform, "--privileged",
		"--volume", input.volumeName + ":/var/lib/containers/storage",
		"--volume", input.workspace.output + ":/output", input.lock.Reference,
		"build", "--arch", b.Spec.Platform.Architecture.Installer, "--bootc-ref", input.installerTag,
		"--bootc-default-fs", "ext4", "--output-dir", "/output",
		"--output-name", outputName, "bootc-generic-iso",
	}
	if err := b.runner.Run(ctx, process.Command{Dir: b.Root, Name: "docker", Args: args}); err != nil {
		return "", fmt.Errorf("build bootc-generic-iso: %w", err)
	}
	isoPath := filepath.Join(input.workspace.output, outputName+".iso")
	if !regularFile(isoPath) {
		return "", fmt.Errorf("image-builder did not create %s", isoPath)
	}
	if err := b.inspectISO(ctx, isoInspectionInput{lock: input.lock, volumeName: input.volumeName, installerTag: input.installerTag, isoPath: isoPath, inspectDir: input.workspace.inspect, reference: input.reference}); err != nil {
		return "", err
	}
	digest, err := fileSHA256(isoPath)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(isoPath+".sha256", []byte(digest+"  "+filepath.Base(isoPath)+"\n"), 0o644); err != nil {
		return "", err
	}
	if err := os.Chmod(isoPath+".sha256", 0o644); err != nil {
		return "", err
	}
	return isoPath, nil
}

func (b *Builder) validate(options Options) (toolLock, error) {
	for label, path := range map[string]string{
		"runtime OCI archive": options.ArchivePath,
		"image-builder lock":  options.ToolLock,
	} {
		if !regularFile(path) {
			return toolLock{}, fmt.Errorf("%s %q is not a regular file", label, path)
		}
	}
	if b.Spec.Identity.Architecture != b.Spec.Platform.Architecture.Name || b.Spec.Base.Platform != b.Spec.Platform.Architecture.Platform {
		return toolLock{}, errors.New("installer architecture differs from the selected Soda platform")
	}
	if b.Spec.Identity.Hostname != "soda" {
		return toolLock{}, errors.New("installer default hostname must be soda")
	}
	return readToolLock(options.ToolLock, b.Spec.Platform)
}

func readToolLock(path string, platform config.PlatformSpec) (toolLock, error) {
	var lock toolLock
	metadata, err := toml.DecodeFile(path, &lock)
	if err != nil {
		return toolLock{}, fmt.Errorf("read image-builder lock: %w", err)
	}
	if len(metadata.Undecoded()) != 0 {
		return toolLock{}, errors.New("image-builder lock contains unknown fields")
	}
	if err := validateToolLock(lock, platform); err != nil {
		return toolLock{}, err
	}
	return lock, nil
}

func validateToolLock(lock toolLock, platform config.PlatformSpec) error {
	validReference := regexp.MustCompile(`^ghcr\.io/osbuild/image-builder@sha256:[0-9a-f]{64}$`).MatchString(lock.Reference)
	if !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(lock.Version) ||
		!regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(lock.Commit) || lock.Platform != platform.Architecture.Platform || !validReference {
		return errors.New("image-builder lock is incomplete or invalid for the selected platform")
	}
	return nil
}

func (b *Builder) copyToStorage(ctx context.Context, lock toolLock, volumeName, archive, reference string) error {
	args := append([]string{"run", "--rm", "--platform", b.Spec.Base.Platform, "--privileged", "--entrypoint", "skopeo",
		"--volume", volumeName + ":/var/lib/containers/storage",
		"--volume", archive + ":/input/image.oci.tar:ro", lock.Reference},
		"--tmpdir", "/var/lib/containers/storage", "copy", "oci-archive:/input/image.oci.tar", "containers-storage:"+reference)
	if err := b.runner.Run(ctx, process.Command{Dir: b.Root, Name: "docker", Args: args}); err != nil {
		return fmt.Errorf("copy %s into installer container storage: %w", reference, err)
	}
	return nil
}

func validateNoEmbeddedPayload(metadata []byte, reference string) error {
	var images []struct {
		Names  []string `json:"names"`
		Digest string   `json:"digest"`
	}
	if err := json.Unmarshal(metadata, &images); err != nil {
		return fmt.Errorf("decode ISO container storage metadata: %w", err)
	}
	manifestDigest := "sha256:" + strings.TrimPrefix(reference, Repository+"@sha256:")
	for _, image := range images {
		if image.Digest == manifestDigest {
			return errors.New("ISO embeds the Soda runtime payload instead of using the exact remote image reference")
		}
		for _, name := range image.Names {
			if name == reference {
				return errors.New("ISO embeds the Soda runtime payload instead of using the exact remote image reference")
			}
		}
	}
	return nil
}

func kickstart(reference, hostname string) string {
	return "# Soda OS stock interactive Anaconda defaults.\n" +
		"graphical\n" +
		"network --bootproto=dhcp --device=link --activate --onboot=on --hostname=" + hostname + "\n" +
		"rootpw --lock\n" +
		"firstboot --disable\n" +
		"bootc --source-imgref=\"docker://" + reference + "\" --target-imgref=\"" + reference + "\"\n"
}

func regularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func fileSHA256(path string) (string, error) {
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
