package release

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/LevitateOS/soda-os/internal/build/installer"
	"github.com/LevitateOS/soda-os/internal/config"
	"github.com/LevitateOS/soda-os/internal/process"
)

const Repository = "ghcr.io/levitateos/soda-os"

type RecordOptions struct {
	ArchivePath       string
	ISOPath           string
	QCOW2Path         string
	QCOW2ZSTPath      string
	OutputDir         string
	InstallerArchive  string
	InstallerToolLock string
}

type Record struct {
	SchemaVersion       uint32 `json:"schema_version"`
	SodaVersion         string `json:"soda_version"`
	SourceRevision      string `json:"source_revision"`
	Platform            string `json:"platform"`
	Channel             string `json:"channel"`
	FedoraBaseReference string `json:"fedora_base_reference"`
	RuntimePackageLock  string `json:"runtime_package_lock"`
	RuntimeLockSHA256   string `json:"runtime_lock_sha256"`
	SodaImageReference  string `json:"soda_image_reference"`
	ArtifactChecksums
}

// ArtifactChecksums binds the local image inventory and every downloadable
// artifact without adding a parallel release-state representation.
type ArtifactChecksums struct {
	RPMInventorySHA256 string `json:"rpm_inventory_sha256"`
	ISOChecksum        string `json:"iso_sha256"`
	QCOW2Checksum      string `json:"qcow2_sha256"`
	QCOW2ZSTChecksum   string `json:"qcow2_zst_sha256"`
}

type Result struct {
	ImageReference string
	RecordPath     string
}

type isoValidator interface {
	ValidateISO(context.Context, string, string, string, string) (string, error)
}

type Publisher struct {
	root             string
	spec             config.DistroSpec
	hostArchitecture string
	isoValidator     isoValidator
}

func NewPublisher(root string, spec config.DistroSpec, runner process.Runner) (*Publisher, error) {
	if spec.Image.Registry != Repository {
		return nil, fmt.Errorf("release repository must be %s", Repository)
	}
	return &Publisher{root: root, spec: spec, hostArchitecture: runtime.GOARCH, isoValidator: installer.NewBuilder(root, spec, runner)}, nil
}

func (p *Publisher) requireNativeHost() error {
	hostArchitecture := p.hostArchitecture
	if hostArchitecture == "" {
		hostArchitecture = runtime.GOARCH
	}
	return config.RequireNativeHostArchitecture(p.spec.Platform.Architecture.Name, hostArchitecture)
}

func (p *Publisher) CreateRecord(ctx context.Context, options RecordOptions) (Result, error) {
	if err := p.requireNativeHost(); err != nil {
		return Result{}, err
	}
	record, reference, err := p.inspectArchive(options.ArchivePath)
	if err != nil {
		return Result{}, err
	}
	checksums, err := p.inspectRecordArtifacts(ctx, options, reference)
	if err != nil {
		return Result{}, err
	}
	lockPath, lockSHA256, err := p.runtimeLockIdentity()
	if err != nil {
		return Result{}, err
	}
	record.RuntimePackageLock = lockPath
	record.RuntimeLockSHA256 = lockSHA256
	checksums.RPMInventorySHA256 = record.RPMInventorySHA256
	record.ArtifactChecksums = checksums
	recordPath, err := writeRecord(record, options.OutputDir)
	if err != nil {
		return Result{}, err
	}
	return Result{ImageReference: reference, RecordPath: recordPath}, nil
}

func (p *Publisher) runtimeLockIdentity() (string, string, error) {
	path := p.spec.Platform.Base.RuntimePackageLock
	if path == "" {
		return "", "", errors.New("release record requires a selected runtime package lock")
	}
	resolvedPath := path
	if !filepath.IsAbs(resolvedPath) {
		resolvedPath = filepath.Join(p.root, resolvedPath)
	}
	digest, err := fileSHA256(resolvedPath)
	if err != nil {
		return "", "", fmt.Errorf("checksum selected runtime package lock: %w", err)
	}
	return path, digest, nil
}

func (p *Publisher) inspectArchive(path string) (Record, string, error) {
	img, cleanup, err := imageFromOCIArchive(path, p.spec.Platform.Architecture.OCI)
	if err != nil {
		return Record{}, "", err
	}
	defer cleanup()
	digest, err := img.Digest()
	if err != nil {
		return Record{}, "", fmt.Errorf("compute local image digest: %w", err)
	}
	reference := Repository + "@" + digest.String()
	record, err := p.inspect(img, reference)
	if err != nil {
		return Record{}, "", err
	}
	return record, reference, nil
}

func (p *Publisher) inspectRecordArtifacts(ctx context.Context, options RecordOptions, reference string) (ArtifactChecksums, error) {
	if options.ISOPath == "" || options.QCOW2Path == "" || options.QCOW2ZSTPath == "" {
		return ArtifactChecksums{}, errors.New("release record requires the installer ISO, raw QCOW2, and compressed QCOW2")
	}
	isoChecksum, err := p.isoValidator.ValidateISO(ctx, options.ISOPath, reference, options.InstallerArchive, options.InstallerToolLock)
	if err != nil {
		return ArtifactChecksums{}, fmt.Errorf("inspect installer ISO: %w", err)
	}
	qcow2Checksum, qcow2ZSTChecksum, err := inspectQCOW2Artifacts(options.QCOW2Path, options.QCOW2ZSTPath)
	if err != nil {
		return ArtifactChecksums{}, err
	}
	return ArtifactChecksums{ISOChecksum: isoChecksum, QCOW2Checksum: qcow2Checksum, QCOW2ZSTChecksum: qcow2ZSTChecksum}, nil
}

func writeRecord(record Record, outputDir string) (string, error) {
	if outputDir == "" {
		outputDir = ".artifacts/releases"
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", fmt.Errorf("create release output: %w", err)
	}
	path := filepath.Join(outputDir, "soda-os-"+record.SodaVersion+"-"+record.Channel+".release.json")
	encoded, err := json.Marshal(record)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, append(encoded, '\n'), 0o644); err != nil {
		return "", fmt.Errorf("write release record: %w", err)
	}
	return path, nil
}
