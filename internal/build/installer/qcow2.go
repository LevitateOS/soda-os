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
	if !regularFile(options.ArchivePath) || !regularFile(options.ToolLock) {
		return QCOW2Result{}, errors.New("QCOW2 construction requires a local OCI archive and image-builder lock")
	}
	lock, err := readToolLock(options.ToolLock, b.Spec.Platform)
	if err != nil {
		return QCOW2Result{}, err
	}
	reference, err := archiveReference(options.ArchivePath, b.Spec.Platform.Architecture.OCI)
	if err != nil {
		return QCOW2Result{}, err
	}
	outputDir := options.OutputDir
	if outputDir == "" {
		outputDir = filepath.Join(b.Root, ".artifacts", "images")
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return QCOW2Result{}, fmt.Errorf("create QCOW2 output directory: %w", err)
	}
	outputName := "SodaOS-" + b.Spec.Identity.Version + "-" + b.Spec.Platform.Architecture.Artifact + ".qcow2"
	rawPath := filepath.Join(outputDir, outputName)
	compressedPath := rawPath + ".zst"
	if err := requireAbsentQCOW2Outputs(rawPath, compressedPath); err != nil {
		return QCOW2Result{}, err
	}
	volumeName := fmt.Sprintf("soda-qcow2-%s-%d", strings.TrimPrefix(reference, Repository+"@sha256:")[:12], os.Getpid())
	if err := b.runner.Run(ctx, process.Command{Dir: b.Root, Name: "docker", Args: []string{"volume", "create", volumeName}}); err != nil {
		return QCOW2Result{}, fmt.Errorf("create disposable QCOW2 container storage: %w", err)
	}
	defer func() {
		_ = b.runner.Run(context.Background(), process.Command{Dir: b.Root, Name: "docker", Args: []string{"volume", "rm", "--force", volumeName}})
	}()
	if err := b.copyToStorage(ctx, lock, volumeName, options.ArchivePath, reference); err != nil {
		return QCOW2Result{}, err
	}
	args := []string{"run", "--rm", "--platform", b.Spec.Base.Platform, "--privileged",
		"--volume", volumeName + ":/var/lib/containers/storage",
		"--volume", outputDir + ":/output", lock.Reference,
		"build", "--arch", b.Spec.Platform.Architecture.Installer, "--bootc-ref", reference,
		"--bootc-default-fs", "ext4", "--output-dir", "/output", "--output-name", strings.TrimSuffix(outputName, ".qcow2"), "qcow2",
	}
	if err := b.runner.Run(ctx, process.Command{Dir: b.Root, Name: "docker", Args: args}); err != nil {
		return QCOW2Result{}, fmt.Errorf("build QCOW2: %w", err)
	}
	if !regularFile(rawPath) {
		return QCOW2Result{}, fmt.Errorf("image-builder did not create %s", rawPath)
	}
	rawDigest, err := fileSHA256(rawPath)
	if err != nil {
		return QCOW2Result{}, fmt.Errorf("checksum QCOW2: %w", err)
	}
	if err := b.runner.Run(ctx, process.Command{Dir: b.Root, Name: "zstd", Args: []string{"--quiet", "--no-progress", "--threads=1", "--force", "--output", compressedPath, rawPath}}); err != nil {
		return QCOW2Result{}, fmt.Errorf("compress QCOW2: %w", err)
	}
	if !regularFile(compressedPath) {
		return QCOW2Result{}, fmt.Errorf("zstd did not create %s", compressedPath)
	}
	compressedDigest, err := fileSHA256(compressedPath)
	if err != nil {
		return QCOW2Result{}, fmt.Errorf("checksum compressed QCOW2: %w", err)
	}
	if err := writeQCOW2Sidecar(compressedPath, compressedDigest); err != nil {
		return QCOW2Result{}, err
	}
	return QCOW2Result{Path: rawPath, SHA256: rawDigest, CompressedPath: compressedPath, CompressedSHA: compressedDigest}, nil
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
