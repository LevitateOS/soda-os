package osupdate

import (
	"context"
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

	"github.com/LevitateOS/soda-os/internal/process"
)

const (
	Repository    = "ghcr.io/levitateos/soda-os"
	StateSchema   = uint32(3)
	DefaultBootc  = "/usr/bin/bootc"
	DefaultSkopeo = "/usr/bin/skopeo"
	DefaultCosign = "/usr/libexec/soda/cosign"
	DefaultKey    = "/usr/share/soda/release/cosign.pub"
	DefaultIndex  = "/usr/share/soda/release/distribution.json"
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
	PublicKey    string
	Architecture string
	Distribution string
	HTTPClient   *http.Client
}

func New(options Options) (*Manager, error) {
	applyDefaults(&options)
	architecture := options.Architecture
	if architecture == "" {
		architecture = runtime.GOARCH
	}
	platform, err := platformFor(architecture)
	if err != nil {
		return nil, err
	}
	distribution, err := readDistribution(options.Distribution)
	if err != nil {
		return nil, err
	}
	if options.HTTPClient == nil {
		options.HTTPClient = http.DefaultClient
	}
	return &Manager{
		runner:    options.Runner,
		bootc:     options.BootcPath,
		discovery: githubDiscovery{client: options.HTTPClient, runner: options.Runner, executable: options.CosignPath, publicKey: options.PublicKey, distribution: distribution, platform: platform},
		verifier:  cosignVerifier{runner: options.Runner, executable: options.CosignPath, publicKey: options.PublicKey},
		inspector: skopeoInspector{runner: options.Runner, executable: options.SkopeoPath, architecture: platform.ociArchitecture},
		platform:  platform,
	}, nil
}

func applyDefaults(options *Options) {
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
	if options.PublicKey == "" {
		options.PublicKey = DefaultKey
	}
	if options.Distribution == "" {
		options.Distribution = DefaultIndex
	}
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
		if errors.Is(err, errInvalidIndex) {
			return Candidate{}, fmt.Errorf("%w: signed release index", ErrRejected)
		}
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

var errInvalidIndex = errors.New("invalid release index")

type distribution struct {
	GitHubRepository string `json:"github_repository"`
	IndexURL         string `json:"index_url"`
	IndexBundleURL   string `json:"index_bundle_url"`
}

type releaseIndex struct {
	SchemaVersion  uint32         `json:"schema_version"`
	SodaVersion    string         `json:"soda_version"`
	SourceRevision string         `json:"source_revision"`
	Releases       []indexRelease `json:"releases"`
}

type indexRelease struct {
	Architecture   string `json:"architecture"`
	ImageReference string `json:"image_reference"`
	ISOAsset       string `json:"iso_asset"`
	ISOChecksum    string `json:"iso_sha256"`
	RecordAsset    string `json:"record_asset"`
	RecordChecksum string `json:"record_sha256"`
}

type githubDiscovery struct {
	client       *http.Client
	runner       process.Runner
	executable   string
	publicKey    string
	distribution distribution
	platform     platformContract
}

func (d githubDiscovery) ResolveCurrent(ctx context.Context) (string, error) {
	index, bundle, err := d.download(ctx)
	if err != nil {
		return "", err
	}
	directory, err := os.MkdirTemp("", "soda-release-index-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(directory)
	indexPath, bundlePath := filepath.Join(directory, "index.json"), filepath.Join(directory, "index.sigstore.json")
	if err = os.WriteFile(indexPath, index, 0o600); err != nil {
		return "", err
	}
	if err = os.WriteFile(bundlePath, bundle, 0o600); err != nil {
		return "", err
	}
	if err = d.runner.Run(ctx, process.Command{Name: d.executable, Args: []string{"verify-blob", "--key", d.publicKey, "--bundle", bundlePath, "--insecure-ignore-tlog=true", indexPath}}); err != nil {
		return "", fmt.Errorf("%w: signature verification failed", errInvalidIndex)
	}
	return releaseForPlatform(index, d.platform)
}

func (d githubDiscovery) download(ctx context.Context) ([]byte, []byte, error) {
	index, err := get(ctx, d.client, d.distribution.IndexURL)
	if err != nil {
		return nil, nil, err
	}
	bundle, err := get(ctx, d.client, d.distribution.IndexBundleURL)
	if err != nil {
		return nil, nil, err
	}
	return index, bundle, nil
}

func get(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s returned %s", url, response.Status)
	}
	return io.ReadAll(response.Body)
}

func releaseForPlatform(contents []byte, platform platformContract) (string, error) {
	var index releaseIndex
	if err := json.Unmarshal(contents, &index); err != nil {
		return "", fmt.Errorf("%w: decode JSON", errInvalidIndex)
	}
	if !validReleaseIndex(index) {
		return "", errInvalidIndex
	}
	return indexReference(index, platform)
}

func validReleaseIndex(index releaseIndex) bool {
	if index.SchemaVersion != 1 || index.SodaVersion == "" || len(index.Releases) != 2 || len(index.SourceRevision) != 40 || !hexadecimal(index.SourceRevision) {
		return false
	}
	seen := map[string]bool{}
	for _, release := range index.Releases {
		if seen[release.Architecture] || !validIndexRelease(release) {
			return false
		}
		seen[release.Architecture] = true
	}
	return seen["aarch64"] && seen["x86_64"]
}

func validIndexRelease(release indexRelease) bool {
	if release.Architecture != "aarch64" && release.Architecture != "x86_64" {
		return false
	}
	if !isSodaDigestReference(release.ImageReference) || release.ISOAsset == "" || release.RecordAsset == "" {
		return false
	}
	return len(release.ISOChecksum) == 64 && len(release.RecordChecksum) == 64
}

func indexReference(index releaseIndex, platform platformContract) (string, error) {
	for _, release := range index.Releases {
		if release.Architecture == platform.artifactArchitecture {
			return release.ImageReference, nil
		}
	}
	return "", errInvalidIndex
}

func readDistribution(path string) (distribution, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return distribution{}, fmt.Errorf("read release distribution: %w", err)
	}
	var value distribution
	if err := json.Unmarshal(contents, &value); err != nil || value.GitHubRepository != "LevitateOS/soda-os" || value.IndexURL == "" || value.IndexBundleURL == "" {
		return distribution{}, errors.New("invalid release distribution")
	}
	return value, nil
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
	runner                process.Runner
	executable, publicKey string
}

func (v cosignVerifier) Verify(ctx context.Context, reference string) error {
	return v.runner.Run(ctx, process.Command{Name: v.executable, Args: []string{
		"verify", "--key", v.publicKey,
		"--insecure-ignore-tlog=true", reference,
	}})
}

func hexadecimal(value string) bool {
	_, err := hex.DecodeString(value)
	return err == nil && value != "" && value == strings.ToLower(value)
}
