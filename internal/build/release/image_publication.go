package release

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	if err := p.runner.Run(ctx, p.skopeo("copy", "--src-tls-verify=true", "--dest-tls-verify=true", "docker://"+candidate, "docker://"+versionTag)); err != nil {
		return ImageResult{}, fmt.Errorf("promote immutable GHCR image: %w", err)
	}
	if err := p.requireImageDigest(ctx, versionTag, record.SodaImageReference); err != nil {
		return ImageResult{}, err
	}
	return ImageResult{Architecture: options.Architecture, Reference: record.SodaImageReference, Revision: revision}, nil
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
