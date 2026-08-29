package release

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/BurntSushi/toml"
	"github.com/LevitateOS/soda-os/internal/build/installer"
	"github.com/LevitateOS/soda-os/internal/config"
	"github.com/LevitateOS/soda-os/internal/process"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

const (
	Repository    = "ghcr.io/levitateos/soda-os"
	CosignVersion = "v3.1.2"
)

type SigningOptions struct {
	PublicKey  string
	PrivateKey string
	CosignPath string
	ToolLock   string
}

type PublicationOptions struct {
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
	StateSchema         uint32 `json:"state_schema"`
	RPMInventorySHA256  string `json:"rpm_inventory_sha256"`
	ISOChecksum         string `json:"iso_sha256,omitempty"`
}

type Result struct {
	ImageReference string
	RecordPath     string
	BundlePath     string
}

type registryClient interface {
	Push(context.Context, string, v1.Image) error
	Resolve(context.Context, string) (v1.Hash, error)
}

type signer interface {
	SignImage(context.Context, string) error
	VerifyImage(context.Context, string) error
	SignBlob(context.Context, string, string) error
	VerifyBlob(context.Context, string, string) error
}

type isoValidator interface {
	ValidateISO(context.Context, string, string, string, string) (string, error)
}

type Publisher struct {
	spec         config.DistroSpec
	registry     registryClient
	signer       signer
	isoValidator isoValidator
	publicKey    string
}

func NewPublisher(root string, spec config.DistroSpec, options SigningOptions, runner process.Runner) (*Publisher, error) {
	if spec.Image.Registry != Repository {
		return nil, fmt.Errorf("release repository must be %s", Repository)
	}
	for label, path := range map[string]string{
		"public signing key":  options.PublicKey,
		"private signing key": options.PrivateKey,
		"cosign executable":   options.CosignPath,
	} {
		if !regularFile(path) {
			return nil, fmt.Errorf("%s %q is not a regular file", label, path)
		}
	}
	if err := verifyCosignBinary(options.CosignPath, options.ToolLock); err != nil {
		return nil, err
	}
	if runner == nil {
		runner = process.OSRunner{Stdout: os.Stdout, Stderr: os.Stderr}
	}
	return &Publisher{
		spec: spec,
		registry: &remoteRegistry{options: []remote.Option{
			remote.WithAuthFromKeychain(authn.DefaultKeychain),
		}},
		signer:       &cosignSigner{runner: runner, executable: options.CosignPath, publicKey: options.PublicKey, privateKey: options.PrivateKey},
		isoValidator: installer.NewBuilder(root, spec, runner),
		publicKey:    options.PublicKey,
	}, nil
}

func (p *Publisher) Prepare(ctx context.Context, archive string) (string, error) {
	prepared, err := p.prepareExactImage(ctx, archive)
	if err != nil {
		return "", err
	}
	defer prepared.cleanup()
	if _, err := p.inspect(prepared.image, prepared.reference); err != nil {
		return "", err
	}
	if err := p.signExactImage(ctx, prepared.reference); err != nil {
		return "", err
	}
	return prepared.reference, nil
}

func (p *Publisher) Publish(ctx context.Context, options PublicationOptions) (Result, error) {
	prepared, err := p.prepareExactImage(ctx, options.ArchivePath)
	if err != nil {
		return Result{}, err
	}
	defer prepared.cleanup()
	return p.finalizePublication(ctx, prepared, options)
}

type preparedRelease struct {
	image     v1.Image
	reference string
	cleanup   func()
}

func (p *Publisher) prepareExactImage(ctx context.Context, archive string) (preparedRelease, error) {
	img, cleanup, err := imageFromOCIArchive(archive, p.spec.Platform.Architecture.OCI)
	if err != nil {
		return preparedRelease{}, err
	}
	versionTag := p.versionTag()
	if err := p.registry.Push(ctx, versionTag, img); err != nil {
		cleanup()
		return preparedRelease{}, fmt.Errorf("push versioned image: %w", err)
	}
	digest, err := p.registry.Resolve(ctx, versionTag)
	if err != nil {
		cleanup()
		return preparedRelease{}, fmt.Errorf("resolve canonical registry digest: %w", err)
	}
	localDigest, err := img.Digest()
	if err != nil {
		cleanup()
		return preparedRelease{}, fmt.Errorf("compute local image digest: %w", err)
	}
	if digest != localDigest {
		cleanup()
		return preparedRelease{}, fmt.Errorf("canonical registry digest %s differs from pushed image digest %s", digest, localDigest)
	}
	return preparedRelease{img, Repository + "@" + digest.String(), cleanup}, nil
}

func (p *Publisher) finalizePublication(ctx context.Context, prepared preparedRelease, options PublicationOptions) (Result, error) {
	record, err := p.inspect(prepared.image, prepared.reference)
	if err != nil {
		return Result{}, err
	}
	if options.ISOPath != "" {
		checksum, err := p.isoValidator.ValidateISO(ctx, options.ISOPath, prepared.reference, options.InstallerArchive, options.InstallerToolLock)
		if err != nil {
			return Result{}, fmt.Errorf("independently inspect installer ISO: %w", err)
		}
		record.ISOChecksum = checksum
	}
	if err := p.signExactImage(ctx, prepared.reference); err != nil {
		return Result{}, err
	}
	recordPath, bundlePath, err := p.writeSignedRecord(ctx, record, options.OutputDir)
	if err != nil {
		return Result{}, err
	}
	return Result{ImageReference: prepared.reference, RecordPath: recordPath, BundlePath: bundlePath}, nil
}

func (p *Publisher) signExactImage(ctx context.Context, reference string) error {
	if err := p.signer.SignImage(ctx, reference); err != nil {
		return fmt.Errorf("sign exact image digest: %w", err)
	}
	if err := p.signer.VerifyImage(ctx, reference); err != nil {
		return fmt.Errorf("verify exact image signature: %w", err)
	}
	return nil
}

func (p *Publisher) writeSignedRecord(ctx context.Context, record Record, outputDir string) (string, string, error) {
	if outputDir == "" {
		outputDir = ".artifacts/releases"
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", "", fmt.Errorf("create release output: %w", err)
	}
	recordPath := filepath.Join(outputDir, "soda-os-"+p.spec.Identity.Version+"-"+p.spec.Platform.Architecture.Artifact+".release.json")
	bundlePath := recordPath + ".sigstore.json"
	encoded, err := json.Marshal(record)
	if err != nil {
		return "", "", err
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(recordPath, encoded, 0o644); err != nil {
		return "", "", fmt.Errorf("write canonical release record: %w", err)
	}
	if err := p.signer.SignBlob(ctx, recordPath, bundlePath); err != nil {
		return "", "", fmt.Errorf("sign release record: %w", err)
	}
	if err := p.signer.VerifyBlob(ctx, recordPath, bundlePath); err != nil {
		return "", "", fmt.Errorf("verify release record: %w", err)
	}
	return recordPath, bundlePath, nil
}

func (p *Publisher) versionTag() string {
	return Repository + ":" + p.spec.Identity.Version + "-" + p.spec.Platform.Release.Channel
}

type remoteRegistry struct{ options []remote.Option }

func (r *remoteRegistry) Push(ctx context.Context, reference string, img v1.Image) error {
	ref, err := name.ParseReference(reference)
	if err != nil {
		return err
	}
	return remote.Write(ref, img, append(r.options, remote.WithContext(ctx))...)
}

func (r *remoteRegistry) Resolve(ctx context.Context, reference string) (v1.Hash, error) {
	ref, err := name.ParseReference(reference)
	if err != nil {
		return v1.Hash{}, err
	}
	descriptor, err := remote.Get(ref, append(r.options, remote.WithContext(ctx))...)
	if err != nil {
		return v1.Hash{}, err
	}
	return descriptor.Digest, nil
}

type cosignSigner struct {
	runner                process.Runner
	executable            string
	publicKey, privateKey string
}

func (s *cosignSigner) SignImage(ctx context.Context, reference string) error {
	return s.runner.Run(ctx, process.Command{Name: s.executable, Args: []string{
		"sign", "--yes", "--use-signing-config=false", "--tlog-upload=false", "--registry-referrers-mode=legacy", "--new-bundle-format=false",
		"--key", s.privateKey, reference,
	}})
}

func (s *cosignSigner) VerifyImage(ctx context.Context, reference string) error {
	return s.runner.Run(ctx, process.Command{Name: s.executable, Args: []string{
		"verify", "--key", s.publicKey,
		"--insecure-ignore-tlog=true", reference,
	}})
}

func (s *cosignSigner) SignBlob(ctx context.Context, blob, bundle string) error {
	return s.runner.Run(ctx, process.Command{Name: s.executable, Args: []string{
		"sign-blob", "--yes", "--use-signing-config=false", "--tlog-upload=false", "--key", s.privateKey,
		"--bundle", bundle, blob,
	}})
}

func (s *cosignSigner) VerifyBlob(ctx context.Context, blob, bundle string) error {
	return s.runner.Run(ctx, process.Command{Name: s.executable, Args: []string{
		"verify-blob", "--key", s.publicKey, "--bundle", bundle,
		"--insecure-ignore-tlog=true", blob,
	}})
}

type toolLock struct {
	Version string         `toml:"version"`
	Binary  []lockedBinary `toml:"binary"`
}

type lockedBinary struct {
	OS     string `toml:"os"`
	Arch   string `toml:"arch"`
	SHA256 string `toml:"sha256"`
}

func verifyCosignBinary(path, lockPath string) error {
	if !regularFile(lockPath) {
		return fmt.Errorf("release tool lock %q is not a regular file", lockPath)
	}
	var lock toolLock
	if _, err := toml.DecodeFile(lockPath, &lock); err != nil {
		return fmt.Errorf("decode release tool lock: %w", err)
	}
	if lock.Version != CosignVersion {
		return fmt.Errorf("release tool lock must pin cosign %s", CosignVersion)
	}
	want := ""
	for _, binary := range lock.Binary {
		if binary.OS == runtime.GOOS && binary.Arch == runtime.GOARCH {
			want = binary.SHA256
			break
		}
	}
	if want == "" {
		return fmt.Errorf("release tool lock has no cosign binary for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read cosign binary: %w", err)
	}
	got := sha256Hex(contents)
	if got != want {
		return fmt.Errorf("cosign binary SHA-256 %s differs from pinned %s", got, want)
	}
	return nil
}
