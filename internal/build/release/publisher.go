package release

import (
	"context"
	"encoding/json"
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
	SodaImageReference  string `json:"soda_image_reference"`
	RPMInventorySHA256  string `json:"rpm_inventory_sha256"`
	ISOChecksum         string `json:"iso_sha256,omitempty"`
}

type Result struct {
	ImageReference string
	RecordPath     string
}

type isoValidator interface {
	ValidateISO(context.Context, string, string, string, string) (string, error)
}

type Publisher struct {
	spec             config.DistroSpec
	hostArchitecture string
	isoValidator     isoValidator
}

func NewPublisher(root string, spec config.DistroSpec, runner process.Runner) (*Publisher, error) {
	if spec.Image.Registry != Repository {
		return nil, fmt.Errorf("release repository must be %s", Repository)
	}
	return &Publisher{spec: spec, hostArchitecture: runtime.GOARCH, isoValidator: installer.NewBuilder(root, spec, runner)}, nil
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
	img, cleanup, err := imageFromOCIArchive(options.ArchivePath, p.spec.Platform.Architecture.OCI)
	if err != nil {
		return Result{}, err
	}
	defer cleanup()
	digest, err := img.Digest()
	if err != nil {
		return Result{}, fmt.Errorf("compute local image digest: %w", err)
	}
	reference := Repository + "@" + digest.String()
	record, err := p.inspect(img, reference)
	if err != nil {
		return Result{}, err
	}
	if options.ISOPath != "" {
		checksum, err := p.isoValidator.ValidateISO(ctx, options.ISOPath, reference, options.InstallerArchive, options.InstallerToolLock)
		if err != nil {
			return Result{}, fmt.Errorf("inspect installer ISO: %w", err)
		}
		record.ISOChecksum = checksum
	}
	recordPath, err := writeRecord(record, options.OutputDir)
	if err != nil {
		return Result{}, err
	}
	return Result{ImageReference: reference, RecordPath: recordPath}, nil
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
