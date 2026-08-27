// Package release publishes one signed Soda OS AArch64 bootc release.
package release

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/LevitateOS/soda-os/internal/config"
	imagebuild "github.com/LevitateOS/soda-os/internal/image"
	"github.com/LevitateOS/soda-os/internal/installer"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

const (
	Repository    = "registry.soda.local/soda/os"
	Platform      = "linux/arm64"
	CosignVersion = "v3.1.2"
)

// Options are explicit operator inputs. The private key and passphrase are
// never copied into output; cosign reads the passphrase interactively.
type Options struct {
	ArchivePath string
	RegistryCA  string
	PublicKey   string
	PrivateKey  string
	ISOPath     string
	// DeferCurrent publishes and signs the exact image so it can be embedded in
	// an installer ISO. It intentionally writes neither a release record nor
	// the mutable current discovery tag.
	DeferCurrent      bool
	OutputDir         string
	CosignPath        string
	ToolLock          string
	InstallerArchive  string
	InstallerToolLock string
}

// Record is the minimal signed identity shared by the OCI and installer.
type Record struct {
	SchemaVersion       uint32 `json:"schema_version"`
	SodaVersion         string `json:"soda_version"`
	SourceRevision      string `json:"source_revision"`
	Platform            string `json:"platform"`
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
	CheckVersion(context.Context) error
	SignImage(context.Context, string) error
	VerifyImage(context.Context, string) error
	SignBlob(context.Context, string, string) error
	VerifyBlob(context.Context, string, string) error
}

type isoValidator interface {
	ValidateISO(context.Context, string, string, string, string) (installer.Provenance, error)
}

// Publisher is injectable so ordering and exact-digest behavior can be tested
// without contacting the production registry or using a production key.
type Publisher struct {
	Spec         config.DistroSpec
	Registry     registryClient
	Signer       signer
	ISOValidator isoValidator
}

func NewPublisher(spec config.DistroSpec, options Options, runner imagebuild.Runner) (*Publisher, error) {
	if spec.Image.Registry != Repository {
		return nil, fmt.Errorf("release repository must be %s", Repository)
	}
	for label, path := range map[string]string{
		"registry CA":         options.RegistryCA,
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
	transport, err := registryTransport(options.RegistryCA)
	if err != nil {
		return nil, err
	}
	if runner == nil {
		runner = imagebuild.OSRunner{Stdout: os.Stdout, Stderr: os.Stderr}
	}
	root, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("resolve workspace for ISO validation: %w", err)
	}
	return &Publisher{
		Spec: spec,
		Registry: &remoteRegistry{options: []remote.Option{
			remote.WithAuthFromKeychain(authn.DefaultKeychain),
			remote.WithTransport(transport),
		}},
		Signer:       &cosignSigner{runner: runner, executable: options.CosignPath, ca: options.RegistryCA, publicKey: options.PublicKey, privateKey: options.PrivateKey},
		ISOValidator: installer.NewBuilder(root, spec, runner),
	}, nil
}

// Publish follows the trusted-LAN release happy path. A deferred publication
// first makes a signed exact image available for ISO construction. The final
// ISO-bound publication writes the release record and current tag last.
func (p *Publisher) Publish(ctx context.Context, options Options) (Result, error) {
	if p.Registry == nil || p.Signer == nil {
		return Result{}, errors.New("release publisher requires a registry and signer")
	}
	if err := p.Signer.CheckVersion(ctx); err != nil {
		return Result{}, err
	}
	img, cleanup, err := imageFromOCIArchive(options.ArchivePath)
	if err != nil {
		return Result{}, err
	}
	defer cleanup()

	versionTag := Repository + ":" + p.Spec.Identity.Version
	if err := p.Registry.Push(ctx, versionTag, img); err != nil {
		return Result{}, fmt.Errorf("push versioned image: %w", err)
	}
	digest, err := p.Registry.Resolve(ctx, versionTag)
	if err != nil {
		return Result{}, fmt.Errorf("resolve canonical registry digest: %w", err)
	}
	localDigest, err := img.Digest()
	if err != nil {
		return Result{}, fmt.Errorf("compute local image digest: %w", err)
	}
	if digest != localDigest {
		return Result{}, fmt.Errorf("canonical registry digest %s differs from pushed image digest %s", digest, localDigest)
	}

	exactReference := Repository + "@" + digest.String()
	if options.ISOPath != "" {
		if p.ISOValidator == nil {
			return Result{}, errors.New("release publisher requires independent ISO inspection")
		}
		if _, err := p.ISOValidator.ValidateISO(ctx, options.ISOPath, exactReference, options.InstallerArchive, options.InstallerToolLock); err != nil {
			return Result{}, fmt.Errorf("independently inspect installer ISO: %w", err)
		}
	}
	record, err := p.inspect(img, exactReference, options.ISOPath, options.RegistryCA, options.PublicKey)
	if err != nil {
		return Result{}, err
	}
	if err := p.Signer.SignImage(ctx, exactReference); err != nil {
		return Result{}, fmt.Errorf("sign exact image digest: %w", err)
	}
	if err := p.Signer.VerifyImage(ctx, exactReference); err != nil {
		return Result{}, fmt.Errorf("verify exact image signature: %w", err)
	}
	if options.DeferCurrent {
		return Result{ImageReference: exactReference}, nil
	}

	outputDir := options.OutputDir
	if outputDir == "" {
		outputDir = ".artifacts/releases"
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("create release output: %w", err)
	}
	recordPath := filepath.Join(outputDir, "soda-os-"+p.Spec.Identity.Version+".release.json")
	bundlePath := recordPath + ".sigstore.json"
	encoded, err := json.Marshal(record)
	if err != nil {
		return Result{}, err
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(recordPath, encoded, 0o644); err != nil {
		return Result{}, fmt.Errorf("write canonical release record: %w", err)
	}
	if err := p.Signer.SignBlob(ctx, recordPath, bundlePath); err != nil {
		return Result{}, fmt.Errorf("sign release record: %w", err)
	}
	if err := p.Signer.VerifyBlob(ctx, recordPath, bundlePath); err != nil {
		return Result{}, fmt.Errorf("verify release record: %w", err)
	}
	if err := p.Registry.Push(ctx, Repository+":current", img); err != nil {
		return Result{}, fmt.Errorf("update current discovery tag: %w", err)
	}
	return Result{ImageReference: exactReference, RecordPath: recordPath, BundlePath: bundlePath}, nil
}

func (p *Publisher) inspect(img v1.Image, exactReference, isoPath, registryCAPath, publicKeyPath string) (Record, error) {
	configFile, err := img.ConfigFile()
	if err != nil {
		return Record{}, fmt.Errorf("inspect image configuration: %w", err)
	}
	if configFile.OS != "linux" || configFile.Architecture != "arm64" {
		return Record{}, fmt.Errorf("release image platform is %s/%s, expected %s", configFile.OS, configFile.Architecture, Platform)
	}
	labels := configFile.Config.Labels
	revision := labels["org.opencontainers.image.revision"]
	if len(revision) != 40 || !hexadecimal(revision) {
		return Record{}, errors.New("release image has no full source revision label")
	}
	stateSchema, err := strconv.ParseUint(labels["org.sodaos.state-schema"], 10, 32)
	if err != nil || uint32(stateSchema) != p.Spec.Image.StateSchema {
		return Record{}, errors.New("release image state schema label differs from the Soda specification")
	}
	if labels["org.opencontainers.image.version"] != p.Spec.Identity.Version {
		return Record{}, errors.New("release image version label differs from the Soda specification")
	}
	if labels["org.opencontainers.image.base.name"] != p.Spec.Base.Reference {
		return Record{}, errors.New("release image Fedora base label differs from the Soda specification")
	}
	for _, trust := range []struct{ label, imagePath, suppliedPath string }{
		{"registry CA", "usr/share/pki/ca-trust-source/anchors/soda-registry-ca.crt", registryCAPath},
		{"signing public key", "usr/share/soda/release/cosign.pub", publicKeyPath},
	} {
		embedded, err := imageFile(img, trust.imagePath)
		if err != nil {
			return Record{}, fmt.Errorf("read embedded %s: %w", trust.label, err)
		}
		supplied, err := os.ReadFile(trust.suppliedPath)
		if err != nil {
			return Record{}, fmt.Errorf("read supplied %s: %w", trust.label, err)
		}
		if !bytes.Equal(embedded, supplied) {
			return Record{}, fmt.Errorf("supplied %s differs from the file embedded in the release image", trust.label)
		}
	}
	inventory, err := imageFile(img, "usr/share/soda/rpm-inventory.txt")
	if err != nil {
		return Record{}, fmt.Errorf("read installed RPM inventory: %w", err)
	}
	sidecar, err := imageFile(img, "usr/share/soda/rpm-inventory.sha256")
	if err != nil {
		return Record{}, fmt.Errorf("read installed RPM inventory sidecar: %w", err)
	}
	inventoryDigest := sha256Hex(inventory)
	fields := strings.Fields(string(sidecar))
	if len(fields) != 2 || fields[0] != inventoryDigest || fields[1] != "rpm-inventory.txt" {
		return Record{}, errors.New("installed RPM inventory does not match its image sidecar")
	}
	record := Record{
		SchemaVersion: 1, SodaVersion: p.Spec.Identity.Version, SourceRevision: revision,
		Platform: Platform, FedoraBaseReference: p.Spec.Base.Reference,
		SodaImageReference: exactReference, StateSchema: p.Spec.Image.StateSchema,
		RPMInventorySHA256: inventoryDigest,
	}
	if isoPath != "" {
		provenance, err := installer.ValidateProvenance(isoPath, exactReference)
		if err != nil {
			return Record{}, fmt.Errorf("validate installer payload: %w", err)
		}
		record.ISOChecksum = provenance.ISOSHA256
	}
	return record, nil
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
	runner                    imagebuild.Runner
	executable                string
	ca, publicKey, privateKey string
}

func (s *cosignSigner) CheckVersion(ctx context.Context) error {
	output, err := s.runner.Output(ctx, imagebuild.Command{Name: s.executable, Args: []string{"version"}})
	if err != nil {
		return fmt.Errorf("inspect cosign version: %w", err)
	}
	if !strings.Contains(output, CosignVersion) {
		return fmt.Errorf("cosign must be %s", CosignVersion)
	}
	return nil
}

func (s *cosignSigner) SignImage(ctx context.Context, reference string) error {
	return s.runner.Run(ctx, imagebuild.Command{Name: s.executable, Args: []string{
		"sign", "--yes", "--use-signing-config=false", "--tlog-upload=false", "--registry-referrers-mode=legacy",
		"--key", s.privateKey, "--registry-cacert", s.ca, reference,
	}})
}

func (s *cosignSigner) VerifyImage(ctx context.Context, reference string) error {
	return s.runner.Run(ctx, imagebuild.Command{Name: s.executable, Args: []string{
		"verify", "--key", s.publicKey, "--registry-cacert", s.ca,
		"--insecure-ignore-tlog=true", reference,
	}})
}

func (s *cosignSigner) SignBlob(ctx context.Context, blob, bundle string) error {
	return s.runner.Run(ctx, imagebuild.Command{Name: s.executable, Args: []string{
		"sign-blob", "--yes", "--use-signing-config=false", "--tlog-upload=false", "--key", s.privateKey,
		"--bundle", bundle, blob,
	}})
}

func (s *cosignSigner) VerifyBlob(ctx context.Context, blob, bundle string) error {
	return s.runner.Run(ctx, imagebuild.Command{Name: s.executable, Args: []string{
		"verify-blob", "--key", s.publicKey, "--bundle", bundle,
		"--insecure-ignore-tlog=true", blob,
	}})
}

func registryTransport(caPath string) (*http.Transport, error) {
	ca, err := os.ReadFile(caPath)
	if err != nil {
		return nil, fmt.Errorf("read registry CA: %w", err)
	}
	roots, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("load system certificate pool: %w", err)
	}
	if !roots.AppendCertsFromPEM(ca) {
		return nil, errors.New("registry CA does not contain a PEM certificate")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = (&tls.Config{}).Clone()
	transport.TLSClientConfig.RootCAs = roots
	return transport, nil
}

func imageFromOCIArchive(path string) (v1.Image, func(), error) {
	if !regularFile(path) {
		return nil, func() {}, fmt.Errorf("OCI archive %q is not a regular file", path)
	}
	directory, err := os.MkdirTemp("", "soda-oci-layout-")
	if err != nil {
		return nil, func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(directory) }
	if err := extractOCIArchive(path, directory); err != nil {
		cleanup()
		return nil, func() {}, err
	}
	index, err := layout.ImageIndexFromPath(directory)
	if err != nil {
		cleanup()
		return nil, func() {}, fmt.Errorf("read OCI archive: %w", err)
	}
	manifest, err := index.IndexManifest()
	if err != nil {
		cleanup()
		return nil, func() {}, err
	}
	if len(manifest.Manifests) != 1 {
		cleanup()
		return nil, func() {}, errors.New("OCI archive must contain exactly one manifest")
	}
	selected := &manifest.Manifests[0]
	if selected.Platform == nil || selected.Platform.OS != "linux" || selected.Platform.Architecture != "arm64" {
		cleanup()
		return nil, func() {}, errors.New("OCI archive manifest must be linux/arm64")
	}
	img, err := index.Image(selected.Digest)
	if err != nil {
		cleanup()
		return nil, func() {}, fmt.Errorf("read AArch64 image: %w", err)
	}
	return img, cleanup, nil
}

func extractOCIArchive(path, directory string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	reader := tar.NewReader(file)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read OCI archive: %w", err)
		}
		clean := filepath.Clean(header.Name)
		if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("OCI archive contains unsafe path %q", header.Name)
		}
		target := filepath.Join(directory, clean)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
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
			if closeErr != nil {
				return closeErr
			}
		default:
			return fmt.Errorf("OCI archive contains unsupported entry %q", header.Name)
		}
	}
}

func imageFile(img v1.Image, target string) ([]byte, error) {
	layers, err := img.Layers()
	if err != nil {
		return nil, err
	}
	for i := len(layers) - 1; i >= 0; i-- {
		stream, err := layers[i].Uncompressed()
		if err != nil {
			return nil, err
		}
		reader := tar.NewReader(stream)
		for {
			header, nextErr := reader.Next()
			if errors.Is(nextErr, io.EOF) {
				break
			}
			if nextErr != nil {
				stream.Close()
				return nil, nextErr
			}
			if strings.TrimPrefix(filepath.Clean(header.Name), "/") == target {
				contents, readErr := io.ReadAll(reader)
				stream.Close()
				return contents, readErr
			}
		}
		stream.Close()
	}
	return nil, fmt.Errorf("image file /%s is missing", target)
}

func regularFileSHA256(path string) (string, error) {
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

func sha256Hex(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

func regularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func hexadecimal(value string) bool {
	_, err := hex.DecodeString(value)
	return err == nil
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
	got, err := regularFileSHA256(path)
	if err != nil {
		return fmt.Errorf("checksum cosign binary: %w", err)
	}
	if got != want {
		return fmt.Errorf("cosign binary SHA-256 %s differs from pinned %s", got, want)
	}
	return nil
}
