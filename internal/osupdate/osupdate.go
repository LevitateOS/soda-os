package osupdate

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/LevitateOS/soda-os/internal/process"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

const (
	Repository    = "registry.soda.local/soda/os"
	StateSchema   = uint32(2)
	DefaultBootc  = "/usr/bin/bootc"
	DefaultSkopeo = "/usr/bin/skopeo"
	DefaultCosign = "/usr/libexec/soda/cosign"
	DefaultCA     = "/usr/share/pki/ca-trust-source/anchors/soda-registry-ca.crt"
	DefaultKey    = "/usr/share/soda/release/cosign.pub"
)

var (
	ErrInvalid      = errors.New("invalid OS update request")
	ErrUnavailable  = errors.New("OS update service unavailable")
	ErrRejected     = errors.New("OS update rejected")
	ErrPrecondition = errors.New("OS update precondition failed")
)

type Deployment struct {
	ImageReference string
	Version        string
	Digest         string
	Architecture   string
	Signature      string
	Incompatible   bool
	DownloadOnly   bool
}

type Status struct {
	Booted   *Deployment
	Staged   *Deployment
	ReadOnly bool
}

type Candidate struct {
	ImageReference      string
	Version             string
	SourceRevision      string
	FedoraBaseReference string
	Digest              string
	StateSchema         uint32
	Available           bool
}

type discovery interface {
	ResolveCurrent(context.Context) (string, error)
}

type inspector interface {
	Inspect(context.Context, string) (imageMetadata, error)
}

type verifier interface {
	Verify(context.Context, string) error
}

type Manager struct {
	runner    process.Runner
	bootc     string
	discovery discovery
	verifier  verifier
	inspector inspector
	platform  platformContract
}

type Options struct {
	Runner       process.Runner
	BootcPath    string
	SkopeoPath   string
	CosignPath   string
	RegistryCA   string
	PublicKey    string
	Architecture string
}

func New(options Options) (*Manager, error) {
	if options.Runner == nil {
		options.Runner = process.OSRunner{Stdout: os.Stdout, Stderr: os.Stderr}
	}
	if options.BootcPath == "" {
		options.BootcPath = DefaultBootc
	}
	if options.SkopeoPath == "" {
		options.SkopeoPath = DefaultSkopeo
	}
	if options.CosignPath == "" {
		options.CosignPath = DefaultCosign
	}
	if options.RegistryCA == "" {
		options.RegistryCA = DefaultCA
	}
	if options.PublicKey == "" {
		options.PublicKey = DefaultKey
	}
	architecture := options.Architecture
	if architecture == "" {
		architecture = runtime.GOARCH
	}
	platform, err := platformFor(architecture)
	if err != nil {
		return nil, err
	}
	transport, err := registryTransport(options.RegistryCA)
	if err != nil {
		return nil, err
	}
	remoteOptions := []remote.Option{remote.WithAuth(authn.Anonymous), remote.WithTransport(transport)}
	return &Manager{
		runner:    options.Runner,
		bootc:     options.BootcPath,
		discovery: remoteDiscovery{options: remoteOptions, tag: platform.discoveryTag(), platform: platform},
		verifier:  cosignVerifier{runner: options.Runner, executable: options.CosignPath, ca: options.RegistryCA, publicKey: options.PublicKey},
		inspector: skopeoInspector{runner: options.Runner, executable: options.SkopeoPath, architecture: platform.ociArchitecture},
		platform:  platform,
	}, nil
}

func (m *Manager) Status(ctx context.Context) (Status, error) {
	output, err := m.runner.Output(ctx, process.Command{Name: m.bootc, Args: []string{"status", "--format=json", "--format-version=1"}})
	if err != nil {
		return Status{}, fmt.Errorf("%w: read bootc status", ErrUnavailable)
	}
	status, err := parseBootcStatus([]byte(output), m.platform)
	if err != nil {
		return Status{}, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	return status, nil
}

func (m *Manager) Check(ctx context.Context) (Candidate, error) {
	exactReference, err := m.discovery.ResolveCurrent(ctx)
	if err != nil {
		return Candidate{}, fmt.Errorf("%w: resolve current release", ErrUnavailable)
	}
	candidate, err := m.inspectRelease(ctx, exactReference)
	if err != nil {
		return Candidate{}, err
	}
	status, statusErr := m.Status(ctx)
	if statusErr != nil {
		return Candidate{}, statusErr
	}
	candidate.Available = status.Booted == nil || status.Booted.Digest != candidate.Digest
	return candidate, nil
}

func (m *Manager) inspectRelease(ctx context.Context, exactReference string) (Candidate, error) {
	if !isSodaDigestReference(exactReference) {
		return Candidate{}, fmt.Errorf("%w: release discovery did not resolve an exact Soda digest", ErrRejected)
	}
	if err := m.verifier.Verify(ctx, exactReference); err != nil {
		return Candidate{}, fmt.Errorf("%w: exact image signature verification failed", ErrRejected)
	}
	metadata, err := m.inspector.Inspect(ctx, exactReference)
	if err != nil {
		return Candidate{}, fmt.Errorf("%w: signed image verification failed", ErrRejected)
	}
	return candidateFromMetadata(exactReference, metadata, m.platform)
}

func candidateFromMetadata(exactReference string, metadata imageMetadata, platform platformContract) (Candidate, error) {
	if metadata.OS != "linux" || metadata.Architecture != platform.ociArchitecture {
		return Candidate{}, fmt.Errorf("%w: release platform is %s/%s, expected %s", ErrRejected, metadata.OS, metadata.Architecture, platform.ociPlatform)
	}
	labels := metadata.Labels
	stateSchema, err := strconv.ParseUint(labels["org.sodaos.state-schema"], 10, 32)
	if err != nil || uint32(stateSchema) != StateSchema {
		return Candidate{}, fmt.Errorf("%w: release state schema must be %d", ErrRejected, StateSchema)
	}
	version := strings.TrimSpace(labels["org.opencontainers.image.version"])
	revision := strings.TrimSpace(labels["org.opencontainers.image.revision"])
	base := strings.TrimSpace(labels["org.opencontainers.image.base.name"])
	if version == "" || len(revision) != 40 || !hexadecimal(revision) || !isDigestReference(base) {
		return Candidate{}, fmt.Errorf("%w: release identity labels are incomplete", ErrRejected)
	}
	digest := strings.TrimPrefix(exactReference, Repository+"@")
	if metadata.Digest != digest {
		return Candidate{}, fmt.Errorf("%w: inspected digest differs from resolved release", ErrRejected)
	}
	return Candidate{ImageReference: exactReference, Version: version, SourceRevision: revision, FedoraBaseReference: base, Digest: digest, StateSchema: uint32(stateSchema)}, nil
}

func (m *Manager) Stage(ctx context.Context, exactReference string) (Status, error) {
	if !isSodaDigestReference(exactReference) {
		return Status{}, fmt.Errorf("%w: an exact %s digest reference is required", ErrInvalid, Repository)
	}
	current, err := m.Status(ctx)
	if err != nil {
		return Status{}, err
	}
	if current.ReadOnly {
		return Status{}, fmt.Errorf("%w: host is a read-only bootc environment", ErrPrecondition)
	}
	if _, err := m.inspectRelease(ctx, exactReference); err != nil {
		return Status{}, err
	}
	if err := m.runner.Run(ctx, process.Command{Name: m.bootc, Args: []string{"switch", "--download-only", "--enforce-container-sigpolicy", exactReference}}); err != nil {
		return Status{}, fmt.Errorf("%w: bootc could not stage the signed release", ErrUnavailable)
	}
	status, err := m.Status(ctx)
	if err != nil {
		return Status{}, err
	}
	if !matchesDownloadedDeployment(status.Staged, exactReference, m.platform.ociArchitecture) {
		return Status{}, fmt.Errorf("%w: bootc did not lock the exact signed %s deployment", ErrPrecondition, m.platform.artifactArchitecture)
	}
	return status, nil
}

func matchesDownloadedDeployment(deployment *Deployment, exactReference, architecture string) bool {
	if deployment == nil {
		return false
	}
	return deployment.ImageReference == exactReference && deployment.DownloadOnly && deployment.Signature == "containerPolicy" && deployment.Architecture == architecture && !deployment.Incompatible
}

func (m *Manager) Activate(ctx context.Context) error {
	status, err := m.Status(ctx)
	if err != nil {
		return err
	}
	if status.Staged == nil {
		return fmt.Errorf("%w: no downloaded deployment is staged", ErrPrecondition)
	}
	if status.ReadOnly || !status.Staged.DownloadOnly {
		return fmt.Errorf("%w: staged deployment is not locked for explicit activation", ErrPrecondition)
	}
	if err := m.runner.Run(ctx, process.Command{Name: m.bootc, Args: []string{"switch", "--from-downloaded", "--apply"}}); err != nil {
		return fmt.Errorf("%w: bootc could not activate the staged release", ErrUnavailable)
	}
	return nil
}

type remoteDiscovery struct {
	options  []remote.Option
	tag      string
	platform platformContract
}

func (d remoteDiscovery) ResolveCurrent(ctx context.Context) (string, error) {
	ref, err := name.ParseReference(d.tag)
	if err != nil {
		return "", err
	}
	descriptor, err := remote.Get(ref, append(d.options, remote.WithContext(ctx))...)
	if err != nil {
		return "", err
	}
	if descriptor.MediaType.IsImage() {
		return Repository + "@" + descriptor.Digest.String(), nil
	}
	return "", fmt.Errorf("%s must be one %s image manifest, got %s", d.tag, d.platform.ociPlatform, descriptor.MediaType)
}

type imageMetadata struct {
	Digest       string            `json:"Digest"`
	Architecture string            `json:"Architecture"`
	OS           string            `json:"Os"`
	Labels       map[string]string `json:"Labels"`
}

type skopeoInspector struct {
	runner       process.Runner
	executable   string
	architecture string
}

func (i skopeoInspector) Inspect(ctx context.Context, reference string) (imageMetadata, error) {
	output, err := i.runner.Output(ctx, process.Command{Name: i.executable, Args: []string{
		"--override-os", "linux", "--override-arch", i.architecture,
		"inspect", "--no-creds", "--no-tags", "--tls-verify=true", "docker://" + reference,
	}})
	if err != nil {
		return imageMetadata{}, err
	}
	var metadata imageMetadata
	if err := json.Unmarshal([]byte(output), &metadata); err != nil {
		return imageMetadata{}, err
	}
	return metadata, nil
}

type cosignVerifier struct {
	runner                    process.Runner
	executable, ca, publicKey string
}

func (v cosignVerifier) Verify(ctx context.Context, reference string) error {
	return v.runner.Run(ctx, process.Command{Name: v.executable, Args: []string{
		"verify", "--key", v.publicKey, "--registry-cacert", v.ca,
		"--insecure-ignore-tlog=true", reference,
	}})
}

type bootcStatusDocument struct {
	Status struct {
		Booted   *bootcDeployment `json:"booted"`
		Staged   *bootcDeployment `json:"staged"`
		ReadOnly bool             `json:"readOnly"`
	} `json:"status"`
}

type bootcDeployment struct {
	Image struct {
		Image struct {
			Image     string `json:"image"`
			Transport string `json:"transport"`
			Signature string `json:"signature"`
		} `json:"image"`
		Version      string `json:"version"`
		ImageDigest  string `json:"imageDigest"`
		Architecture string `json:"architecture"`
	} `json:"image"`
	Incompatible bool `json:"incompatible"`
	DownloadOnly bool `json:"downloadOnly"`
}

func parseBootcStatus(contents []byte, platform platformContract) (Status, error) {
	var document bootcStatusDocument
	if err := json.Unmarshal(contents, &document); err != nil {
		return Status{}, fmt.Errorf("decode bootc status: %w", err)
	}
	if document.Status.Booted == nil {
		return Status{}, errors.New("host has no booted bootc deployment")
	}
	booted, err := deployment(document.Status.Booted, platform)
	if err != nil {
		return Status{}, fmt.Errorf("decode booted deployment: %w", err)
	}
	result := Status{Booted: &booted, ReadOnly: document.Status.ReadOnly}
	if document.Status.Staged != nil {
		staged, err := stagedDeployment(document.Status.Staged, platform)
		if err != nil {
			return Status{}, fmt.Errorf("decode staged deployment: %w", err)
		}
		result.Staged = &staged
	}
	return result, nil
}

func stagedDeployment(value *bootcDeployment, platform platformContract) (Deployment, error) {
	result, err := deployment(value, platform)
	if err != nil {
		return Deployment{}, err
	}
	if value.Image.Image.Signature != "containerPolicy" {
		return Deployment{}, errors.New("staged deployment does not enforce the Soda container signature policy")
	}
	return result, nil
}

func deployment(value *bootcDeployment, platform platformContract) (Deployment, error) {
	digest := strings.TrimSpace(value.Image.ImageDigest)
	if !validSHA256(digest) {
		return Deployment{}, errors.New("deployment has no valid image digest")
	}
	ref, err := name.ParseReference(value.Image.Image.Image)
	if err != nil || ref.Context().Name() != Repository {
		return Deployment{}, errors.New("deployment is not a Soda OS image")
	}
	if value.Image.Image.Transport != "registry" {
		return Deployment{}, errors.New("deployment is not registry-backed")
	}
	if value.Image.Architecture != platform.ociArchitecture {
		return Deployment{}, fmt.Errorf("deployment is not %s", platform.artifactArchitecture)
	}
	if value.Incompatible {
		return Deployment{}, errors.New("deployment is incompatible with bootc mutation")
	}
	return Deployment{
		ImageReference: Repository + "@" + digest, Version: value.Image.Version, Digest: digest,
		Architecture: value.Image.Architecture, Signature: value.Image.Image.Signature,
		Incompatible: value.Incompatible, DownloadOnly: value.DownloadOnly,
	}, nil
}

func registryTransport(caPath string) (*http.Transport, error) {
	ca, err := os.ReadFile(caPath)
	if err != nil {
		return nil, fmt.Errorf("read registry CA: %w", err)
	}
	roots, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("load system CA pool: %w", err)
	}
	if !roots.AppendCertsFromPEM(ca) {
		return nil, errors.New("registry CA is not a PEM certificate")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}
	return transport, nil
}

func isSodaDigestReference(value string) bool {
	ref, err := name.NewDigest(value)
	return err == nil && ref.Context().Name() == Repository && validSHA256(ref.DigestStr())
}

func isDigestReference(value string) bool {
	ref, err := name.NewDigest(value)
	return err == nil && validSHA256(ref.DigestStr())
}

func validSHA256(value string) bool {
	if !strings.HasPrefix(value, "sha256:") || len(value) != len("sha256:")+64 {
		return false
	}
	return hexadecimal(strings.TrimPrefix(value, "sha256:"))
}

func hexadecimal(value string) bool {
	_, err := hex.DecodeString(value)
	return err == nil && value != "" && value == strings.ToLower(value)
}
