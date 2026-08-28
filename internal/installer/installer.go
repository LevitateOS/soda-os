// Package installer builds and inspects Soda OS bootc installer media.
package installer

import (
	"archive/tar"
	"bytes"
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
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/LevitateOS/soda-os/internal/config"
	imagebuild "github.com/LevitateOS/soda-os/internal/image"
	"github.com/google/go-containerregistry/pkg/v1/layout"
)

const (
	Repository = "registry.soda.local/soda/os"
	Platform   = "linux/arm64"
)

var exactImagePattern = regexp.MustCompile(`^registry\.soda\.local/soda/os@sha256:[0-9a-f]{64}$`)

type toolLock struct {
	Version   string `toml:"version"`
	Commit    string `toml:"commit"`
	Reference string `toml:"reference"`
	Platform  string `toml:"platform"`
}

// Options are explicit local build inputs. ArchivePath must contain the same
// image digest named by ImageReference; inspection proves that exact payload
// was copied into the finished ISO before the checksum is recorded.
type Options struct {
	ImageReference string
	ArchivePath    string
	RegistryCA     string
	PublicKey      string
	CosignPath     string
	ToolLock       string
	OutputDir      string
}

// Provenance binds an ISO checksum to the exact immutable payload found in it.
type Provenance struct {
	SchemaVersion          uint32 `json:"schema_version"`
	ISOPath                string `json:"iso_path"`
	ISOSHA256              string `json:"iso_sha256"`
	EmbeddedImageReference string `json:"embedded_image_reference"`
	Platform               string `json:"platform"`
	Filesystem             string `json:"filesystem"`
	ImageBuilderVersion    string `json:"image_builder_version"`
	ImageBuilderReference  string `json:"image_builder_reference"`
}

type Result struct {
	ISOPath string
}

// ValidateISO independently re-opens an already-built ISO, extracts its
// squashfs, and checks the exact kickstart and embedded containers-storage
// payload before release publication records its checksum.
func (b *Builder) ValidateISO(ctx context.Context, isoPath, reference, installerArchive, toolLockPath string) (Provenance, error) {
	if !exactImagePattern.MatchString(reference) {
		return Provenance{}, errors.New("installer payload must be an exact registry.soda.local/soda/os@sha256 reference")
	}
	if !regularFile(isoPath) || !regularFile(installerArchive) || !regularFile(toolLockPath) {
		return Provenance{}, errors.New("ISO validation requires the ISO, installer environment archive, and image-builder lock")
	}
	var lock toolLock
	if _, err := toml.DecodeFile(toolLockPath, &lock); err != nil {
		return Provenance{}, fmt.Errorf("read image-builder lock: %w", err)
	}
	if err := validateToolLock(lock); err != nil {
		return Provenance{}, err
	}
	return b.inspectPublishedISO(ctx, isoPath, reference, installerArchive, lock)
}

func (b *Builder) inspectPublishedISO(ctx context.Context, isoPath, reference, installerArchive string, lock toolLock) (Provenance, error) {
	workRoot := filepath.Join(b.Root, ".artifacts", "installer")
	if err := os.MkdirAll(workRoot, 0o755); err != nil {
		return Provenance{}, err
	}
	inspectDir, err := os.MkdirTemp(workRoot, "publish-inspect-")
	if err != nil {
		return Provenance{}, err
	}
	defer os.RemoveAll(inspectDir)
	volumeName := fmt.Sprintf("soda-iso-inspect-%s-%d", strings.TrimPrefix(reference, Repository+"@sha256:")[:12], os.Getpid())
	if err := b.runner.Run(ctx, imagebuild.Command{Dir: b.Root, Name: "docker", Args: []string{"volume", "create", volumeName}}); err != nil {
		return Provenance{}, fmt.Errorf("create disposable ISO inspection storage: %w", err)
	}
	defer func() {
		_ = b.runner.Run(context.Background(), imagebuild.Command{Dir: b.Root, Name: "docker", Args: []string{"volume", "rm", "--force", volumeName}})
	}()
	installerTag := "localhost/soda-installer-inspect:" + b.Spec.Identity.Version
	if err := b.copyToStorage(ctx, lock, volumeName, installerArchive, installerTag); err != nil {
		return Provenance{}, err
	}
	payloadTag := payloadStagingReference(reference)
	if err := b.inspectISO(ctx, lock, volumeName, installerTag, isoPath, inspectDir, reference, payloadTag); err != nil {
		return Provenance{}, err
	}
	digest, err := fileSHA256(isoPath)
	if err != nil {
		return Provenance{}, fmt.Errorf("checksum installer ISO: %w", err)
	}
	return Provenance{SchemaVersion: 1, ISOPath: filepath.Base(isoPath), ISOSHA256: digest, EmbeddedImageReference: reference, Platform: Platform, Filesystem: "ext4", ImageBuilderVersion: lock.Version, ImageBuilderReference: lock.Reference}, nil
}

type Builder struct {
	Root   string
	Spec   config.DistroSpec
	runner imagebuild.Runner
}

func NewBuilder(root string, spec config.DistroSpec, runner imagebuild.Runner) *Builder {
	if runner == nil {
		runner = imagebuild.OSRunner{}
	}
	return &Builder{Root: root, Spec: spec, runner: runner}
}

func (b *Builder) Build(ctx context.Context, options Options) (Result, error) {
	lock, err := b.validate(options)
	if err != nil {
		return Result{}, err
	}
	if err := b.verifySignedImage(ctx, options); err != nil {
		return Result{}, err
	}
	if err := verifyArchiveDigest(options.ArchivePath, options.ImageReference); err != nil {
		return Result{}, err
	}
	baseTag, err := imagebuild.PrepareLocalBootcBase(ctx, b.Root, b.runner, b.Spec.Base.Reference)
	if err != nil {
		return Result{}, err
	}
	workspace, err := b.prepareInstallerWorkspace(options)
	if err != nil {
		return Result{}, err
	}
	volumeName := fmt.Sprintf("soda-installer-%s-%d", strings.TrimPrefix(options.ImageReference, Repository+"@sha256:")[:12], os.Getpid())
	if err := b.runner.Run(ctx, imagebuild.Command{Dir: b.Root, Name: "docker", Args: []string{"volume", "create", volumeName}}); err != nil {
		return Result{}, fmt.Errorf("create disposable installer container storage: %w", err)
	}
	defer func() {
		_ = b.runner.Run(context.Background(), imagebuild.Command{Dir: b.Root, Name: "docker", Args: []string{"volume", "rm", "--force", volumeName}})
	}()

	installerArchive, installerTag, err := b.buildInstallerEnvironment(ctx, baseTag, workspace.work)
	if err != nil {
		return Result{}, err
	}
	payloadTag := payloadStagingReference(options.ImageReference)
	for _, item := range []struct{ archive, reference string }{
		{installerArchive, installerTag},
		{options.ArchivePath, payloadTag},
	} {
		if err := b.copyToStorage(ctx, lock, volumeName, item.archive, item.reference); err != nil {
			return Result{}, err
		}
	}

	return b.buildInstallerISO(ctx, lock, volumeName, installerTag, workspace, options.ImageReference, payloadTag)
}

type installerWorkspace struct{ work, context, inspect, output string }

func (b *Builder) prepareInstallerWorkspace(options Options) (installerWorkspace, error) {
	work := filepath.Join(b.Root, ".artifacts", "installer")
	output := options.OutputDir
	if output == "" {
		output = filepath.Join(b.Root, ".artifacts", "images")
	}
	workspace := installerWorkspace{work, filepath.Join(work, "context"), filepath.Join(work, "inspect"), output}
	for _, path := range []string{workspace.context, workspace.inspect} {
		if err := os.RemoveAll(path); err != nil {
			return installerWorkspace{}, err
		}
		if err := os.MkdirAll(path, 0o755); err != nil {
			return installerWorkspace{}, err
		}
	}
	if err := os.MkdirAll(workspace.output, 0o755); err != nil {
		return installerWorkspace{}, err
	}
	if err := os.WriteFile(filepath.Join(workspace.context, "interactive-defaults.ks"), []byte(kickstart(options.ImageReference, b.Spec.Identity.Hostname)), 0o644); err != nil {
		return installerWorkspace{}, err
	}
	return workspace, nil
}

func (b *Builder) buildInstallerEnvironment(ctx context.Context, baseTag, work string) (string, string, error) {
	archive := filepath.Join(work, "soda-installer-environment.oci.tar")
	if err := os.Remove(archive); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", "", err
	}
	tag := "localhost/soda-installer:" + b.Spec.Identity.Version
	args := []string{"buildx", "build", "--platform", Platform, "--build-context", "fedora-base=docker-image://" + baseTag, "--file", "packaging/installer/Containerfile", "--tag", tag, "--provenance=false", "--output", "type=oci,dest=" + archive + ",oci-mediatypes=true", "."}
	if err := b.runner.Run(ctx, imagebuild.Command{Dir: b.Root, Name: "docker", Args: args}); err != nil {
		return "", "", fmt.Errorf("build installer environment: %w", err)
	}
	return archive, tag, nil
}

func (b *Builder) buildInstallerISO(ctx context.Context, lock toolLock, volumeName, installerTag string, workspace installerWorkspace, reference, payloadTag string) (Result, error) {
	outputName := "SodaOS-" + b.Spec.Identity.Version + "-aarch64"
	for _, suffix := range []string{".iso", ".iso.sha256"} {
		if err := os.Remove(filepath.Join(workspace.output, outputName+suffix)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return Result{}, err
		}
	}
	args := b.containerArgs(lock, volumeName, workspace.output)
	args = append(args,
		"build", "--arch", "aarch64", "--bootc-ref", installerTag,
		"--bootc-installer-payload-ref", payloadTag,
		"--bootc-default-fs", "ext4", "--output-dir", "/output",
		"--output-name", outputName, "bootc-generic-iso",
	)
	if err := b.runner.Run(ctx, imagebuild.Command{Dir: b.Root, Name: "docker", Args: args}); err != nil {
		return Result{}, fmt.Errorf("build bootc-generic-iso: %w", err)
	}
	isoPath := filepath.Join(workspace.output, outputName+".iso")
	if !regularFile(isoPath) {
		return Result{}, fmt.Errorf("image-builder did not create %s", isoPath)
	}
	if err := b.inspectISO(ctx, lock, volumeName, installerTag, isoPath, workspace.inspect, reference, payloadTag); err != nil {
		return Result{}, err
	}
	digest, err := fileSHA256(isoPath)
	if err != nil {
		return Result{}, err
	}
	if err := os.WriteFile(isoPath+".sha256", []byte(digest+"  "+filepath.Base(isoPath)+"\n"), 0o644); err != nil {
		return Result{}, err
	}
	return Result{ISOPath: isoPath}, nil
}

func (b *Builder) validate(options Options) (toolLock, error) {
	if !exactImagePattern.MatchString(options.ImageReference) {
		return toolLock{}, errors.New("installer payload must be an exact registry.soda.local/soda/os@sha256 reference")
	}
	for label, path := range map[string]string{
		"runtime OCI archive": options.ArchivePath, "registry CA": options.RegistryCA,
		"public signing key": options.PublicKey, "Cosign executable": options.CosignPath,
		"image-builder lock": options.ToolLock,
	} {
		if !regularFile(path) {
			return toolLock{}, fmt.Errorf("%s %q is not a regular file", label, path)
		}
	}
	if b.Spec.Identity.Architecture != "aarch64" || b.Spec.Base.Platform != Platform {
		return toolLock{}, errors.New("installer builds support only Soda AArch64")
	}
	if b.Spec.Identity.Hostname != "soda" {
		return toolLock{}, errors.New("installer default hostname must be soda")
	}
	var lock toolLock
	if _, err := toml.DecodeFile(options.ToolLock, &lock); err != nil {
		return toolLock{}, fmt.Errorf("read image-builder lock: %w", err)
	}
	if err := validateToolLock(lock); err != nil {
		return toolLock{}, err
	}
	return lock, nil
}

func validateToolLock(lock toolLock) error {
	if lock.Version != "81.0.0" || lock.Commit != "3130fb87ee1f684b6e9d1909f354861c43d7a092" ||
		lock.Reference != "ghcr.io/osbuild/image-builder@sha256:704dc05d6033799248a33c415f7f7253ec20b40f0b2bff03b06d8687179e058a" || lock.Platform != Platform {
		return errors.New("image-builder lock differs from the reviewed AArch64 tool")
	}
	return nil
}

func (b *Builder) verifySignedImage(ctx context.Context, options Options) error {
	output, err := b.runner.Output(ctx, imagebuild.Command{Name: options.CosignPath, Args: []string{"version"}})
	if err != nil || !strings.Contains(output, "v3.1.2") {
		return errors.New("installer requires the pinned Cosign v3.1.2 tool")
	}
	if err := b.runner.Run(ctx, imagebuild.Command{Name: options.CosignPath, Args: []string{
		"verify", "--key", options.PublicKey, "--registry-cacert", options.RegistryCA,
		"--insecure-ignore-tlog=true", options.ImageReference,
	}}); err != nil {
		return fmt.Errorf("verify signed installer payload: %w", err)
	}
	return nil
}

func (b *Builder) copyToStorage(ctx context.Context, lock toolLock, volumeName, archive, reference string) error {
	args := append([]string{"run", "--rm", "--platform", Platform, "--privileged", "--entrypoint", "skopeo",
		"--volume", volumeName + ":/var/lib/containers/storage",
		"--volume", archive + ":/input/image.oci.tar:ro", lock.Reference},
		"copy", "oci-archive:/input/image.oci.tar", "containers-storage:"+reference)
	if err := b.runner.Run(ctx, imagebuild.Command{Dir: b.Root, Name: "docker", Args: args}); err != nil {
		return fmt.Errorf("copy %s into installer container storage: %w", reference, err)
	}
	return nil
}

func (b *Builder) containerArgs(lock toolLock, volumeName, outputDir string) []string {
	return []string{"run", "--rm", "--platform", Platform, "--privileged",
		"--volume", volumeName + ":/var/lib/containers/storage",
		"--volume", outputDir + ":/output", lock.Reference}
}

func (b *Builder) inspectISO(ctx context.Context, lock toolLock, volumeName, installerTag, isoPath, inspectDir, reference, payloadTag string) error {
	outer := []string{"run", "--rm", "--platform", Platform, "--privileged", "--entrypoint", "podman",
		"--volume", volumeName + ":/var/lib/containers/storage", "--volume", isoPath + ":/input/soda.iso:ro",
		"--volume", inspectDir + ":/inspect", lock.Reference,
		"run", "--rm", "--privileged", "--security-opt", "label=disable",
		"--volume", "/input/soda.iso:/input/soda.iso:ro", "--volume", "/inspect:/inspect", installerTag}
	args := append(append([]string{}, outer...), "xorriso",
		"-osirrox", "on", "-indev", "/input/soda.iso", "-extract", "/LiveOS/squashfs.img", "/inspect/squashfs.img")
	if err := b.runner.Run(ctx, imagebuild.Command{Dir: b.Root, Name: "docker", Args: args}); err != nil {
		return fmt.Errorf("extract installer squashfs: %w", err)
	}
	args = append(append([]string{}, outer...), "unsquashfs",
		"-f", "-d", "/inspect/root", "/inspect/squashfs.img",
		"usr/share/anaconda/interactive-defaults.ks", "etc/anaconda/conf.d/90-soda-storage.conf",
		"usr/lib/image-builder/bootc/iso.yaml", "var/lib/containers/storage/overlay-images/images.json")
	if err := b.runner.Run(ctx, imagebuild.Command{Dir: b.Root, Name: "docker", Args: args}); err != nil {
		return fmt.Errorf("inspect installer squashfs: %w", err)
	}
	return b.validateExtractedISO(inspectDir, reference, payloadTag)
}

func (b *Builder) validateExtractedISO(inspectDir, reference, payloadTag string) error {
	kickstartBytes, err := os.ReadFile(filepath.Join(inspectDir, "root", "usr/share/anaconda/interactive-defaults.ks"))
	if err != nil {
		return fmt.Errorf("read ISO kickstart: %w", err)
	}
	expected := kickstart(reference, b.Spec.Identity.Hostname)
	if string(kickstartBytes) != expected {
		return errors.New("ISO kickstart differs from exact Soda payload contract")
	}
	storageMetadata, err := os.ReadFile(filepath.Join(inspectDir, "root", "var/lib/containers/storage/overlay-images/images.json"))
	if err != nil {
		return fmt.Errorf("read embedded container storage metadata: %w", err)
	}
	if err := validateEmbeddedPayload(storageMetadata, payloadTag, reference); err != nil {
		return err
	}
	for _, file := range []struct {
		actual, expected, label string
		validate                func([]byte, []byte) error
	}{
		{"usr/lib/image-builder/bootc/iso.yaml", "iso.yaml", "ISO configuration", validateISOConfig},
		{"etc/anaconda/conf.d/90-soda-storage.conf", "soda-storage.conf", "installer storage configuration", validateStorageConfig},
	} {
		actual, err := os.ReadFile(filepath.Join(inspectDir, "root", file.actual))
		if err != nil {
			return fmt.Errorf("read %s: %w", file.label, err)
		}
		expected, err := os.ReadFile(filepath.Join(b.Root, "packaging", "installer", file.expected))
		if err != nil {
			return fmt.Errorf("read expected %s: %w", file.label, err)
		}
		if err := file.validate(actual, expected); err != nil {
			return err
		}
	}
	return nil
}

func payloadStagingReference(reference string) string {
	return Repository + ":payload-" + strings.TrimPrefix(reference, Repository+"@sha256:")
}

func validateEmbeddedPayload(metadata []byte, payloadTag, reference string) error {
	var images []struct {
		Names  []string `json:"names"`
		Digest string   `json:"digest"`
	}
	if err := json.Unmarshal(metadata, &images); err != nil {
		return fmt.Errorf("decode embedded container storage metadata: %w", err)
	}
	manifestDigest := "sha256:" + strings.TrimPrefix(reference, Repository+"@sha256:")
	for _, image := range images {
		if containsString(image.Names, payloadTag) && image.Digest == manifestDigest {
			return nil
		}
	}
	return errors.New("ISO container storage does not contain the staged Soda payload and exact manifest digest")
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func validateISOConfig(actual, expected []byte) error {
	if !bytes.Equal(actual, expected) {
		return errors.New("ISO boot configuration differs from the Soda installer contract")
	}
	return nil
}

func validateStorageConfig(actual, expected []byte) error {
	if !bytes.Equal(actual, expected) {
		return errors.New("ISO storage configuration differs from the Soda ext4 root-only contract")
	}
	return nil
}

func kickstart(reference, hostname string) string {
	return "# Soda OS stock interactive Anaconda defaults.\n" +
		"text\n" +
		"network --bootproto=dhcp --device=link --activate --onboot=on --hostname=" + hostname + "\n" +
		"bootc --source-imgref=\"containers-storage:" + reference + "\" --target-imgref=\"" + reference + "\"\n"
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

func verifyArchiveDigest(path, expectedReference string) error {
	directory, err := os.MkdirTemp("", "soda-installer-oci-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(directory)
	if err := extractRuntimeOCIArchive(path, directory); err != nil {
		return err
	}
	index, err := layout.ImageIndexFromPath(directory)
	if err != nil {
		return fmt.Errorf("read runtime OCI layout: %w", err)
	}
	manifest, err := index.IndexManifest()
	if err != nil {
		return err
	}
	if len(manifest.Manifests) != 1 {
		return errors.New("runtime OCI archive must contain exactly one manifest")
	}
	descriptor := manifest.Manifests[0]
	if descriptor.Platform == nil || descriptor.Platform.OS != "linux" || descriptor.Platform.Architecture != "arm64" {
		return errors.New("runtime OCI archive manifest must be linux/arm64")
	}
	expectedDigest := strings.TrimPrefix(expectedReference, Repository+"@")
	if descriptor.Digest.String() != expectedDigest {
		return fmt.Errorf("runtime OCI archive digest %s differs from exact payload %s", descriptor.Digest, expectedDigest)
	}
	return nil
}

func extractRuntimeOCIArchive(path, directory string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open runtime OCI archive: %w", err)
	}
	defer file.Close()
	reader := tar.NewReader(file)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read runtime OCI archive: %w", err)
		}
		if err := extractRuntimeOCIEntry(reader, header, directory); err != nil {
			return err
		}
	}
}

func extractRuntimeOCIEntry(reader *tar.Reader, header *tar.Header, directory string) error {
	clean := filepath.Clean(header.Name)
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("runtime OCI archive contains unsafe path %q", header.Name)
	}
	target := filepath.Join(directory, clean)
	switch header.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(target, 0o755)
	case tar.TypeReg, tar.TypeRegA:
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(output, reader)
		closeErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	default:
		return fmt.Errorf("runtime OCI archive contains unsupported entry %q", header.Name)
	}
}
