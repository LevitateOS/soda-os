package image

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/LevitateOS/soda-os/internal/build/oci"
	"github.com/LevitateOS/soda-os/internal/process"
)

// PublishImage publishes the archive's own revision, then advances only the
// selected native development tag from the verified exact remote digest.
// Explicit republishing may reuse an identical revision tag; conflicts stop.
func (b *Builder) PublishImage(ctx context.Context, archive string) (oci.Image, error) {
	if err := b.requireNativeHost(); err != nil {
		return oci.Image{}, err
	}
	archive = b.path(archive)
	image, err := oci.Inspect(archive, b.Spec.Platform.Architecture.OCI)
	if err != nil {
		return oci.Image{}, err
	}
	revisionTag := "sha-" + image.Revision + "-" + b.Spec.Platform.Architecture.Artifact
	if err := b.publishRevision(ctx, archive, revisionTag, image.Digest); err != nil {
		return oci.Image{}, err
	}
	development := oci.Repository + ":dev-" + b.Spec.Platform.Architecture.Artifact
	if err := b.runner.Run(ctx, b.skopeo("copy", "--preserve-digests", "--src-tls-verify=true", "--dest-tls-verify=true", "docker://"+image.Reference(), "docker://"+development)); err != nil {
		return oci.Image{}, fmt.Errorf("revision %s verified at %s; advancement of %s may have occurred; inspect remote state before an explicit retry: %w", revisionTag, image.Reference(), development, err)
	}
	if err := b.requirePublishedDigest(ctx, development, image.Digest, "--no-creds"); err != nil {
		return oci.Image{}, fmt.Errorf("revision %s verified; %s copy completed but anonymous verification failed; development tag may have advanced: %w", revisionTag, development, err)
	}
	return image, nil
}

func (b *Builder) publishRevision(ctx context.Context, archive, tag, digest string) error {
	output, err := b.runner.Output(ctx, b.skopeo("list-tags", "--tls-verify=true", "docker://"+oci.Repository))
	if err != nil {
		return fmt.Errorf("inspect revision tags before publication: %w", err)
	}
	var response struct{ Tags []string }
	if err := json.Unmarshal([]byte(output), &response); err != nil {
		return fmt.Errorf("decode revision tags: %w", err)
	}
	if response.Tags == nil {
		return errors.New("registry tag listing has no Tags array")
	}
	reference := oci.Repository + ":" + tag
	if slices.Contains(response.Tags, tag) {
		if err := b.requirePublishedDigest(ctx, reference, digest); err != nil {
			return fmt.Errorf("existing revision tag cannot be reused; no development advancement attempted: %w", err)
		}
		return nil
	}
	if err := b.runner.Run(ctx, b.skopeo("copy", "--preserve-digests", "--dest-tls-verify=true", "oci-archive:"+archive, "docker://"+reference)); err != nil {
		return fmt.Errorf("publication of %s may have occurred; no development advancement attempted; inspect before retry: %w", reference, err)
	}
	if err := b.requirePublishedDigest(ctx, reference, digest); err != nil {
		return fmt.Errorf("revision copy completed but verification failed; no development advancement attempted: %w", err)
	}
	return nil
}

func (b *Builder) requirePublishedDigest(ctx context.Context, reference, expected string, credentialArgs ...string) error {
	args := []string{"inspect", "--tls-verify=true", "--format", "{{.Digest}}"}
	args = append(args, credentialArgs...)
	args = append(args, "docker://"+reference)
	output, err := b.runner.Output(ctx, b.skopeo(args...))
	if err != nil {
		return fmt.Errorf("inspect %s: %w", reference, err)
	}
	if strings.TrimSpace(output) != expected {
		return fmt.Errorf("remote digest for %s differs from archive digest %s", reference, expected)
	}
	return nil
}

func (b *Builder) skopeo(args ...string) process.Command {
	return process.Command{Dir: b.Root, Env: []string{"NO_COLOR=1"}, Name: "skopeo", Args: args}
}
