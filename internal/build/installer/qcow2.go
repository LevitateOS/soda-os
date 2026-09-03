package installer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LevitateOS/soda-os/internal/process"
)

// QCOW2Options identifies one local Soda OCI archive and the Image Builder
// contract used to turn it into a reusable virtual disk.
type QCOW2Options struct {
	ArchivePath string
	ToolLock    string
	OutputDir   string
}

// QCOW2Result identifies the raw disk and its fixed-compression download.
type QCOW2Result struct {
	Path           string
	SHA256         string
	CompressedPath string
	CompressedSHA  string
}

// BuildQCOW2 creates a matching-native reusable disk from the exact local
// single-platform OCI archive. The archive digest remains the deployment
// image reference; Image Builder receives that exact reference from its local
// container storage and no registry credential is involved.
func (b *Builder) BuildQCOW2(ctx context.Context, options QCOW2Options) (QCOW2Result, error) {
	if err := b.requireNativeHost(); err != nil {
		return QCOW2Result{}, err
	}
	input, err := b.prepareQCOW2(options)
	if err != nil {
		return QCOW2Result{}, err
	}
	if err := b.buildQCOW2(ctx, input); err != nil {
		return QCOW2Result{}, err
	}
	return b.compressQCOW2(ctx, input)
}

type qcow2Input struct {
	lock           toolLock
	archivePath    string
	reference      string
	outputDir      string
	rawPath        string
	compressedPath string
}

func (b *Builder) prepareQCOW2(options QCOW2Options) (qcow2Input, error) {
	if !regularFile(options.ArchivePath) || !regularFile(options.ToolLock) {
		return qcow2Input{}, errors.New("QCOW2 construction requires a local OCI archive and image-builder lock")
	}
	lock, err := readToolLock(options.ToolLock, b.Spec.Platform)
	if err != nil {
		return qcow2Input{}, err
	}
	reference, err := archiveReference(options.ArchivePath, b.Spec.Platform.Architecture.OCI)
	if err != nil {
		return qcow2Input{}, err
	}
	outputDir := options.OutputDir
	if outputDir == "" {
		outputDir = filepath.Join(b.Root, ".artifacts", "images")
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return qcow2Input{}, fmt.Errorf("create QCOW2 output directory: %w", err)
	}
	outputName := "SodaOS-" + b.Spec.Identity.Version + "-" + b.Spec.Platform.Architecture.Artifact + ".qcow2"
	rawPath := filepath.Join(outputDir, outputName)
	compressedPath := rawPath + ".zst"
	if err := requireAbsentQCOW2Outputs(rawPath, compressedPath); err != nil {
		return qcow2Input{}, err
	}
	return qcow2Input{lock: lock, archivePath: options.ArchivePath, reference: reference, outputDir: outputDir, rawPath: rawPath, compressedPath: compressedPath}, nil
}

func (b *Builder) buildQCOW2(ctx context.Context, input qcow2Input) error {
	volumeName := fmt.Sprintf("soda-qcow2-%s-%d", strings.TrimPrefix(input.reference, Repository+"@sha256:")[:12], os.Getpid())
	if err := b.runner.Run(ctx, process.Command{Dir: b.Root, Name: "docker", Args: []string{"volume", "create", volumeName}}); err != nil {
		return fmt.Errorf("create disposable QCOW2 container storage: %w", err)
	}
	defer func() {
		_ = b.runner.Run(context.Background(), process.Command{Dir: b.Root, Name: "docker", Args: []string{"volume", "rm", "--force", volumeName}})
	}()
	if err := b.copyToStorage(ctx, input.lock, volumeName, input.archivePath, input.reference); err != nil {
		return err
	}
	return b.runQCOW2Build(ctx, input, volumeName)
}

func (b *Builder) runQCOW2Build(ctx context.Context, input qcow2Input, volumeName string) error {
	outputName := strings.TrimSuffix(filepath.Base(input.rawPath), ".qcow2")
	args := []string{"run", "--rm", "--platform", b.Spec.Base.Platform, "--privileged",
		"--volume", volumeName + ":/var/lib/containers/storage",
		"--volume", input.outputDir + ":/output", input.lock.Reference,
		"build", "--arch", b.Spec.Platform.Architecture.Installer, "--bootc-ref", input.reference,
		"--bootc-default-fs", "ext4", "--output-dir", "/output", "--output-name", outputName, "qcow2",
	}
	if err := b.runner.Run(ctx, process.Command{Dir: b.Root, Name: "docker", Args: args}); err != nil {
		return fmt.Errorf("build QCOW2: %w", err)
	}
	if !regularFile(input.rawPath) {
		return fmt.Errorf("image-builder did not create %s", input.rawPath)
	}
	return nil
}

func (b *Builder) compressQCOW2(ctx context.Context, input qcow2Input) (QCOW2Result, error) {
	rawDigest, err := fileSHA256(input.rawPath)
	if err != nil {
		return QCOW2Result{}, fmt.Errorf("checksum QCOW2: %w", err)
	}
	if err := b.runner.Run(ctx, process.Command{Dir: b.Root, Name: "zstd", Args: []string{"-q", "--no-progress", "-T1", "--force", "--output", input.compressedPath, input.rawPath}}); err != nil {
		return QCOW2Result{}, fmt.Errorf("compress QCOW2: %w", err)
	}
	if !regularFile(input.compressedPath) {
		return QCOW2Result{}, fmt.Errorf("zstd did not create %s", input.compressedPath)
	}
	compressedDigest, err := fileSHA256(input.compressedPath)
	if err != nil {
		return QCOW2Result{}, fmt.Errorf("checksum compressed QCOW2: %w", err)
	}
	if err := writeQCOW2Sidecar(input.compressedPath, compressedDigest); err != nil {
		return QCOW2Result{}, err
	}
	return QCOW2Result{Path: input.rawPath, SHA256: rawDigest, CompressedPath: input.compressedPath, CompressedSHA: compressedDigest}, nil
}

func requireAbsentQCOW2Outputs(rawPath, compressedPath string) error {
	for _, path := range []string{rawPath, compressedPath, compressedPath + ".sha256"} {
		if _, err := os.Lstat(path); err == nil {
			return fmt.Errorf("QCOW2 output %q already exists", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func writeQCOW2Sidecar(path, digest string) error {
	contents := []byte(digest + "  " + filepath.Base(path) + "\n")
	if err := os.WriteFile(path+".sha256", contents, 0o644); err != nil {
		return fmt.Errorf("write compressed QCOW2 checksum: %w", err)
	}
	return os.Chmod(path+".sha256", 0o644)
}
