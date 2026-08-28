// Package installer builds and inspects Soda OS bootc installer media.
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
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
	imagebuild "github.com/LevitateOS/soda-os/internal/build/image"
	"github.com/LevitateOS/soda-os/internal/config"
	"github.com/LevitateOS/soda-os/internal/process"
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

// ValidateISO independently re-opens an already-built ISO, extracts its
// squashfs, and checks the exact kickstart and embedded containers-storage
// payload before release publication records its checksum.
func (b *Builder) ValidateISO(ctx context.Context, isoPath, reference, installerArchive, toolLockPath string) (string, error) {
	if !exactImagePattern.MatchString(reference) {
		return "", errors.New("installer payload must be an exact registry.soda.local/soda/os@sha256 reference")
	}
	if !regularFile(isoPath) || !regularFile(installerArchive) || !regularFile(toolLockPath) {
		return "", errors.New("ISO validation requires the ISO, installer environment archive, and image-builder lock")
	}
	var lock toolLock
	if _, err := toml.DecodeFile(toolLockPath, &lock); err != nil {
		return "", fmt.Errorf("read image-builder lock: %w", err)
	}
	if err := validateToolLock(lock); err != nil {
		return "", err
	}
	return b.inspectPublishedISO(ctx, isoPath, reference, installerArchive, lock)
}

func (b *Builder) inspectPublishedISO(ctx context.Context, isoPath, reference, installerArchive string, lock toolLock) (string, error) {
	workRoot := filepath.Join(b.Root, ".artifacts", "installer")
	if err := os.MkdirAll(workRoot, 0o755); err != nil {
		return "", err
	}
	inspectDir, err := os.MkdirTemp(workRoot, "publish-inspect-")
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
	installerTag := "localhost/soda-installer-inspect:" + b.Spec.Identity.Version
	if err := b.copyToStorage(ctx, lock, volumeName, installerArchive, installerTag); err != nil {
		return "", err
	}
	payloadTag := payloadStagingReference(reference)
	if err := b.inspectISO(ctx, isoInspectionInput{lock: lock, volumeName: volumeName, installerTag: installerTag, isoPath: isoPath, inspectDir: inspectDir, reference: reference, payloadTag: payloadTag}); err != nil {
		return "", err
	}
	digest, err := fileSHA256(isoPath)
	if err != nil {
		return "", fmt.Errorf("checksum installer ISO: %w", err)
	}
	return digest, nil
}

type Builder struct {
	Root   string
	Spec   config.DistroSpec
	runner process.Runner
}

func NewBuilder(root string, spec config.DistroSpec, runner process.Runner) *Builder {
	if runner == nil {
		runner = process.OSRunner{}
	}
	return &Builder{Root: root, Spec: spec, runner: runner}
}

func (b *Builder) Build(ctx context.Context, options Options) (string, error) {
	lock, err := b.validate(options)
	if err != nil {
		return "", err
	}
	if err := b.verifySignedImage(ctx, options); err != nil {
		return "", err
	}
	if err := verifyArchiveDigest(options.ArchivePath, options.ImageReference); err != nil {
		return "", err
	}
	baseTag, err := imagebuild.PrepareLocalBootcBase(ctx, b.Root, b.runner, b.Spec.Base.Reference)
	if err != nil {
		return "", err
	}
	workspace, err := b.prepareInstallerWorkspace(options)
	if err != nil {
		return "", err
	}
	volumeName := fmt.Sprintf("soda-installer-%s-%d", strings.TrimPrefix(options.ImageReference, Repository+"@sha256:")[:12], os.Getpid())
	if err := b.runner.Run(ctx, process.Command{Dir: b.Root, Name: "docker", Args: []string{"volume", "create", volumeName}}); err != nil {
		return "", fmt.Errorf("create disposable installer container storage: %w", err)
	}
	defer func() {
		_ = b.runner.Run(context.Background(), process.Command{Dir: b.Root, Name: "docker", Args: []string{"volume", "rm", "--force", volumeName}})
	}()

	installerArchive, installerTag, err := b.buildInstallerEnvironment(ctx, baseTag, workspace.work)
	if err != nil {
		return "", err
	}
	payloadTag := payloadStagingReference(options.ImageReference)
	for _, item := range []struct{ archive, reference string }{
		{installerArchive, installerTag},
		{options.ArchivePath, payloadTag},
	} {
		if err := b.copyToStorage(ctx, lock, volumeName, item.archive, item.reference); err != nil {
			return "", err
		}
	}

	return b.buildInstallerISO(ctx, isoBuildInput{lock: lock, volumeName: volumeName, installerTag: installerTag, workspace: workspace, reference: options.ImageReference, payloadTag: payloadTag})
}

type installerWorkspace struct{ work, context, inspect, output string }

type isoBuildInput struct {
	lock                     toolLock
	volumeName, installerTag string
	workspace                installerWorkspace
	reference, payloadTag    string
}

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
	if err := b.runner.Run(ctx, process.Command{Dir: b.Root, Name: "docker", Args: args}); err != nil {
		return "", "", fmt.Errorf("build installer environment: %w", err)
	}
	return archive, tag, nil
}

func (b *Builder) buildInstallerISO(ctx context.Context, input isoBuildInput) (string, error) {
	outputName := "SodaOS-" + b.Spec.Identity.Version + "-aarch64"
	for _, suffix := range []string{".iso", ".iso.sha256"} {
		if err := os.Remove(filepath.Join(input.workspace.output, outputName+suffix)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
	args := []string{"run", "--rm", "--platform", Platform, "--privileged",
		"--volume", input.volumeName + ":/var/lib/containers/storage",
		"--volume", input.workspace.output + ":/output", input.lock.Reference,
		"build", "--arch", "aarch64", "--bootc-ref", input.installerTag,
		"--bootc-installer-payload-ref", input.payloadTag,
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
	if err := b.inspectISO(ctx, isoInspectionInput{lock: input.lock, volumeName: input.volumeName, installerTag: input.installerTag, isoPath: isoPath, inspectDir: input.workspace.inspect, reference: input.reference, payloadTag: input.payloadTag}); err != nil {
		return "", err
	}
	digest, err := fileSHA256(isoPath)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(isoPath+".sha256", []byte(digest+"  "+filepath.Base(isoPath)+"\n"), 0o644); err != nil {
		return "", err
	}
	return isoPath, nil
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
	output, err := b.runner.Output(ctx, process.Command{Name: options.CosignPath, Args: []string{"version"}})
	if err != nil || !strings.Contains(output, "v3.1.2") {
		return errors.New("installer requires the pinned Cosign v3.1.2 tool")
	}
	if err := b.runner.Run(ctx, process.Command{Name: options.CosignPath, Args: []string{
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
	if err := b.runner.Run(ctx, process.Command{Dir: b.Root, Name: "docker", Args: args}); err != nil {
		return fmt.Errorf("copy %s into installer container storage: %w", reference, err)
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
		if slices.Contains(image.Names, payloadTag) && image.Digest == manifestDigest {
			return nil
		}
	}
	return errors.New("ISO container storage does not contain the staged Soda payload and exact manifest digest")
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
