package release

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LevitateOS/soda-os/internal/config"
	"github.com/LevitateOS/soda-os/internal/process"
)

type ImageStageOptions struct {
	Architecture string
	ArchivePath  string
}

type ImagePromoteOptions struct {
	Architecture string
	RecordPath   string
}

type ImageResult struct {
	Architecture string
	Reference    string
	Revision     string
}

// ImageStage publishes one matching-native local OCI archive under an
// immutable source-revision candidate tag, then verifies the registry digest.
func (p *Publication) ImageStage(ctx context.Context, options ImageStageOptions) (ImageResult, error) {
	spec, err := p.nativeSpec(options.Architecture)
	if err != nil {
		return ImageResult{}, err
	}
	revision, err := p.cleanRevision(ctx)
	if err != nil {
		return ImageResult{}, err
	}
	reference, err := stageArchiveReference(options.ArchivePath, spec, revision)
	if err != nil {
		return ImageResult{}, err
	}
	candidate := p.candidateImageTag(revision, spec)
	if err := p.requireImageTagAbsent(ctx, candidate); err != nil {
		return ImageResult{}, err
	}
	if err := p.runner.Run(ctx, p.skopeo("copy", "--dest-tls-verify=true", "oci-archive:"+options.ArchivePath, "docker://"+candidate)); err != nil {
		return ImageResult{}, fmt.Errorf("publish immutable GHCR candidate image: %w", err)
	}
	if err := p.requireImageDigest(ctx, candidate, reference); err != nil {
		return ImageResult{}, err
	}
	return ImageResult{Architecture: options.Architecture, Reference: reference, Revision: revision}, nil
}

// ImagePromote creates the immutable version tag only after the locally
// validated record and existing source-revision candidate agree on one digest.
func (p *Publication) ImagePromote(ctx context.Context, options ImagePromoteOptions) (ImageResult, error) {
	spec, err := p.nativeSpec(options.Architecture)
	if err != nil {
		return ImageResult{}, err
	}
	revision, err := p.cleanRevision(ctx)
	if err != nil {
		return ImageResult{}, err
	}
	record, err := readStrictRecord(options.RecordPath)
	if err != nil {
		return ImageResult{}, err
	}
	if err := validatePublicationRecord(record, spec, revision); err != nil {
		return ImageResult{}, err
	}
	candidate := p.candidateImageTag(revision, spec)
	if err := p.requireImageDigest(ctx, candidate, record.SodaImageReference); err != nil {
		return ImageResult{}, err
	}
	versionTag := p.versionImageTag(spec)
	if err := p.requireImageTagAbsent(ctx, versionTag); err != nil {
		return ImageResult{}, err
	}
	if err := p.signAndAttestImage(ctx, record, spec); err != nil {
		return ImageResult{}, err
	}
	if err := p.runner.Run(ctx, p.skopeo("copy", "--src-tls-verify=true", "--dest-tls-verify=true", "docker://"+candidate, "docker://"+versionTag)); err != nil {
		return ImageResult{}, fmt.Errorf("promote immutable GHCR image: %w", err)
	}
	if err := p.requireImageDigest(ctx, versionTag, record.SodaImageReference); err != nil {
		return ImageResult{}, err
	}
	return ImageResult{Architecture: options.Architecture, Reference: record.SodaImageReference, Revision: revision}, nil
}

func (p *Publication) signAndAttestImage(ctx context.Context, record Record, spec config.DistroSpec) error {
	if p.workflowRunURL == "" {
		return errors.New("image signing requires a GitHub Actions workflow run identity")
	}
	predicate, cleanup, err := p.writeImageProvenance(record, spec)
	if err != nil {
		return err
	}
	defer cleanup()
	if err := p.runner.Run(ctx, p.cosign("sign", "--yes", record.SodaImageReference)); err != nil {
		return fmt.Errorf("keylessly sign exact GHCR image digest: %w", err)
	}
	if err := p.runner.Run(ctx, p.cosign("attest", "--yes", "--type", "slsaprovenance", "--predicate", predicate, record.SodaImageReference)); err != nil {
		return fmt.Errorf("attach exact GHCR image provenance: %w", err)
	}
	if err := p.verifySignedImage(ctx, record.SodaImageReference); err != nil {
		return err
	}
	return nil
}

func (p *Publication) verifySignedImage(ctx context.Context, reference string) error {
	if err := p.runner.Run(ctx, p.cosign("verify", "--certificate-identity", p.releaseWorkflowIdentity(), "--certificate-oidc-issuer", githubOIDCIssuer, reference)); err != nil {
		return fmt.Errorf("verify GHCR image signature: %w", err)
	}
	if err := p.runner.Run(ctx, p.cosign("verify-attestation", "--type", "slsaprovenance", "--certificate-identity", p.releaseWorkflowIdentity(), "--certificate-oidc-issuer", githubOIDCIssuer, reference)); err != nil {
		return fmt.Errorf("verify GHCR image provenance: %w", err)
	}
	return nil
}

type imageProvenance struct {
	Type          string              `json:"_type"`
	Subject       []provenanceSubject `json:"subject"`
	PredicateType string              `json:"predicateType"`
	Predicate     provenancePredicate `json:"predicate"`
}

type provenanceSubject struct {
	Name   string            `json:"name"`
	Digest map[string]string `json:"digest"`
}

type provenancePredicate struct {
	BuildDefinition provenanceBuildDefinition `json:"buildDefinition"`
	RunDetails      provenanceRunDetails      `json:"runDetails"`
}

type provenanceBuildDefinition struct {
	BuildType            string                 `json:"buildType"`
	ExternalParameters   provenanceInputs       `json:"externalParameters"`
	InternalParameters   map[string]string      `json:"internalParameters"`
	ResolvedDependencies []provenanceDependency `json:"resolvedDependencies"`
}

type provenanceInputs struct {
	SourceRevision      string `json:"source_revision"`
	Architecture        string `json:"architecture"`
	FedoraBaseReference string `json:"fedora_base_reference"`
	RuntimePackageLock  string `json:"runtime_package_lock"`
	RuntimeLockSHA256   string `json:"runtime_lock_sha256"`
	RPMInventorySHA256  string `json:"rpm_inventory_sha256"`
	ISOChecksum         string `json:"iso_sha256"`
	QCOW2Checksum       string `json:"qcow2_sha256"`
	QCOW2ZSTChecksum    string `json:"qcow2_zst_sha256"`
}

type provenanceDependency struct {
	URI    string            `json:"uri"`
	Digest map[string]string `json:"digest"`
}

type provenanceRunDetails struct {
	Builder struct {
		ID string `json:"id"`
	} `json:"builder"`
	Metadata struct {
		InvocationID string `json:"invocationId"`
	} `json:"metadata"`
}

func (p *Publication) writeImageProvenance(record Record, spec config.DistroSpec) (string, func(), error) {
	if err := p.validateRecordedRuntimeLock(record, spec); err != nil {
		return "", nil, err
	}
	imageDigest := strings.TrimPrefix(record.SodaImageReference, Repository+"@sha256:")
	_, baseDigest, found := strings.Cut(record.FedoraBaseReference, "@sha256:")
	if !found || !validHexadecimal(baseDigest, 64) {
		return "", nil, errors.New("Fedora base reference does not contain an exact SHA-256 digest")
	}
	statement := imageProvenance{
		Type:          "https://in-toto.io/Statement/v1",
		Subject:       []provenanceSubject{{Name: Repository, Digest: map[string]string{"sha256": imageDigest}}},
		PredicateType: "https://slsa.dev/provenance/v1",
		Predicate: provenancePredicate{
			BuildDefinition: provenanceBuildDefinition{
				BuildType: "https://github.com/LevitateOS/soda-os/.github/workflows/release.yml",
				ExternalParameters: provenanceInputs{
					SourceRevision: record.SourceRevision, Architecture: spec.Platform.Architecture.Name,
					FedoraBaseReference: record.FedoraBaseReference, RuntimePackageLock: record.RuntimePackageLock, RuntimeLockSHA256: record.RuntimeLockSHA256,
					RPMInventorySHA256: record.RPMInventorySHA256, ISOChecksum: record.ISOChecksum,
					QCOW2Checksum: record.QCOW2Checksum, QCOW2ZSTChecksum: record.QCOW2ZSTChecksum,
				},
				InternalParameters: map[string]string{},
				ResolvedDependencies: []provenanceDependency{
					{URI: "git+https://github.com/" + p.repository + "@" + record.SourceRevision, Digest: map[string]string{"sha1": record.SourceRevision}},
					{URI: record.FedoraBaseReference, Digest: map[string]string{"sha256": baseDigest}},
				},
			},
		},
	}
	statement.Predicate.RunDetails.Builder.ID = p.releaseWorkflowIdentity()
	statement.Predicate.RunDetails.Metadata.InvocationID = p.workflowRunURL
	encoded, err := json.Marshal(statement)
	if err != nil {
		return "", nil, fmt.Errorf("encode image provenance: %w", err)
	}
	file, err := os.CreateTemp("", "soda-image-provenance-*.json")
	if err != nil {
		return "", nil, fmt.Errorf("create image provenance: %w", err)
	}
	path := file.Name()
	if _, err := file.Write(append(encoded, '\n')); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", nil, fmt.Errorf("write image provenance: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", nil, fmt.Errorf("close image provenance: %w", err)
	}
	return path, func() { _ = os.Remove(path) }, nil
}

func (p *Publication) validateRecordedRuntimeLock(record Record, spec config.DistroSpec) error {
	if record.RuntimePackageLock != spec.Platform.Base.RuntimePackageLock {
		return errors.New("release record runtime package lock differs from the selected platform")
	}
	lockPath := record.RuntimePackageLock
	if !filepath.IsAbs(lockPath) {
		lockPath = filepath.Join(p.root, lockPath)
	}
	lockSHA256, err := fileSHA256(lockPath)
	if err != nil {
		return fmt.Errorf("checksum %s runtime package lock: %w", spec.Platform.Architecture.Name, err)
	}
	if lockSHA256 != record.RuntimeLockSHA256 {
		return errors.New("release record runtime package lock checksum differs from the selected platform")
	}
	return nil
}

func (p *Publication) skopeo(args ...string) process.Command {
	return process.Command{Dir: p.root, Env: []string{"NO_COLOR=1"}, Name: "skopeo", Args: args}
}

func (p *Publication) candidateImageTag(revision string, spec config.DistroSpec) string {
	return Repository + ":sha-" + revision + "-" + spec.Platform.Architecture.Artifact
}

func (p *Publication) versionImageTag(spec config.DistroSpec) string {
	return Repository + ":" + p.version + "-" + spec.Platform.Architecture.Artifact
}

func (p *Publication) requireImageTagAbsent(ctx context.Context, tag string) error {
	output, err := p.runner.Output(ctx, p.skopeo("list-tags", "docker://"+Repository))
	if err != nil {
		return fmt.Errorf("inspect GHCR image tags: %w", err)
	}
	var response struct {
		Tags []string `json:"Tags"`
	}
	if err := json.Unmarshal([]byte(output), &response); err != nil {
		return fmt.Errorf("decode GHCR image tags: %w", err)
	}
	_, name, found := strings.Cut(tag, ":")
	if !found || name == "" {
		return errors.New("invalid immutable GHCR image tag")
	}
	for _, existing := range response.Tags {
		if existing == name {
			return fmt.Errorf("immutable GHCR image tag %q already exists", name)
		}
	}
	return nil
}

func (p *Publication) requireImageDigest(ctx context.Context, tag, reference string) error {
	output, err := p.runner.Output(ctx, p.skopeo("inspect", "--format", "{{.Digest}}", "docker://"+tag))
	if err != nil {
		return fmt.Errorf("inspect GHCR image digest: %w", err)
	}
	digest := strings.TrimSpace(output)
	expected := strings.TrimPrefix(reference, Repository+"@")
	if digest != expected {
		return errors.New("GHCR image digest differs from the exact Soda release record")
	}
	return nil
}

func stageArchiveReference(path string, spec config.DistroSpec, revision string) (string, error) {
	image, cleanup, err := imageFromOCIArchive(path, spec.Platform.Architecture.OCI)
	if err != nil {
		return "", err
	}
	defer cleanup()
	digest, err := image.Digest()
	if err != nil {
		return "", fmt.Errorf("compute local image digest: %w", err)
	}
	record, err := (&Publisher{spec: spec}).inspect(image, Repository+"@"+digest.String())
	if err != nil {
		return "", err
	}
	if record.SourceRevision != revision {
		return "", errors.New("local OCI archive source revision differs from clean Soda source revision")
	}
	return record.SodaImageReference, nil
}
