package release

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/LevitateOS/soda-os/internal/process"
)

func (p *Publication) Publish(ctx context.Context, options PublishOptions) (PublicationResult, error) {
	revision, records, err := p.validatePublishInput(ctx, options)
	if err != nil {
		return PublicationResult{}, err
	}
	view, err := p.requirePublishableDraft(ctx, revision)
	if err != nil {
		return PublicationResult{}, err
	}
	if err := p.verifyPublicationOutputs(ctx, revision, records); err != nil {
		return PublicationResult{}, err
	}
	assets := assetIdentities(view.Assets)
	if err := p.publishDraft(ctx); err != nil {
		return PublicationResult{}, err
	}
	if err := p.requireUnchangedPublishedAssets(ctx, revision, assets); err != nil {
		return PublicationResult{}, err
	}
	return PublicationResult{Tag: p.tag(), Revision: revision, Assets: assets}, nil
}

func (p *Publication) validatePublishInput(ctx context.Context, options PublishOptions) (string, []releaseRecordPath, error) {
	revision, err := p.cleanRevision(ctx)
	if err != nil {
		return "", nil, err
	}
	records, err := p.verifySignedRecords(ctx, revision, options)
	if err != nil {
		return "", nil, err
	}
	return revision, records, nil
}

func (p *Publication) requirePublishableDraft(ctx context.Context, revision string) (releaseView, error) {
	if err := p.authenticate(ctx); err != nil {
		return releaseView{}, err
	}
	view, err := p.requireRelease(ctx, revision, draftRelease)
	if err != nil {
		return releaseView{}, err
	}
	if err := requirePublishableAssets(view.Assets, p.version); err != nil {
		return releaseView{}, err
	}
	return view, nil
}

func (p *Publication) verifyPublicationOutputs(ctx context.Context, revision string, records []releaseRecordPath) error {
	if err := p.verifyPublishedImages(ctx, records); err != nil {
		return err
	}
	return p.requireRemoteReleaseBranch(ctx, revision)
}

func (p *Publication) publishDraft(ctx context.Context) error {
	if err := p.runner.Run(ctx, p.gh("release", "edit", p.tag(), "--repo", p.repository, "--verify-tag", "--draft=false", "--latest")); err != nil {
		return fmt.Errorf("publish GitHub release: %w", err)
	}
	return nil
}

func (p *Publication) requireUnchangedPublishedAssets(ctx context.Context, revision string, before []string) error {
	view, err := p.requireRelease(ctx, revision, publishedRelease)
	if err != nil {
		return fmt.Errorf("verify published GitHub release: %w", err)
	}
	if after := assetIdentities(view.Assets); !equalStrings(before, after) {
		return errors.New("GitHub release assets changed while publishing")
	}
	return nil
}

func (p *Publication) verifySignedRecords(ctx context.Context, revision string, options PublishOptions) ([]releaseRecordPath, error) {
	records, err := p.validateReleaseRecords(revision, options.AArch64RecordPath, options.X86RecordPath)
	if err != nil {
		return nil, err
	}
	for _, item := range records {
		if err := p.verifySignedRecord(ctx, item); err != nil {
			return nil, err
		}
	}
	return records, nil
}

func (p *Publication) verifySignedRecord(ctx context.Context, item releaseRecordPath) error {
	bundle := item.path + ".sigstore.json"
	if err := requireRegularFile("signed release-record bundle", bundle); err != nil {
		return err
	}
	if err := p.runner.Run(ctx, p.cosign("verify-blob", "--bundle", bundle, "--certificate-identity", p.releaseWorkflowIdentity(), "--certificate-oidc-issuer", githubOIDCIssuer, item.path)); err != nil {
		return fmt.Errorf("verify %s signed release record: %w", item.architecture, err)
	}
	return nil
}

func (p *Publication) verifyPublishedImages(ctx context.Context, records []releaseRecordPath) error {
	for _, item := range records {
		if err := p.verifyPublishedImage(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

func (p *Publication) verifyPublishedImage(ctx context.Context, item releaseRecordPath) error {
	versionTag := p.versionImageTag(p.specs[item.architecture])
	if err := p.requireImageDigest(ctx, versionTag, item.record.SodaImageReference); err != nil {
		return fmt.Errorf("verify %s immutable GHCR version tag: %w", item.architecture, err)
	}
	if err := p.requireAnonymousImageDigest(ctx, item.record.SodaImageReference); err != nil {
		return fmt.Errorf("verify anonymous %s GHCR digest pull: %w", item.architecture, err)
	}
	if err := p.verifySignedImage(ctx, item.record.SodaImageReference); err != nil {
		return fmt.Errorf("verify %s GHCR signing: %w", item.architecture, err)
	}
	return nil
}

func (p *Publication) requireAnonymousImageDigest(ctx context.Context, reference string) error {
	output, err := p.runner.Output(ctx, p.skopeo("inspect", "--no-creds", "--format", "{{.Digest}}", "docker://"+reference))
	if err != nil {
		return fmt.Errorf("inspect exact GHCR digest anonymously: %w", err)
	}
	expected := strings.TrimPrefix(reference, Repository+"@")
	if strings.TrimSpace(output) != expected {
		return errors.New("anonymous GHCR digest pull differs from the Soda release record")
	}
	return nil
}

func (p *Publication) requireRemoteReleaseBranch(ctx context.Context, revision string) error {
	output, err := p.runner.Output(ctx, process.Command{Dir: p.root, Name: "git", Args: []string{"ls-remote", "--exit-code", "origin", "refs/heads/release/" + p.version}})
	if err != nil {
		return fmt.Errorf("inspect remote release branch: %w", err)
	}
	fields := strings.Fields(output)
	if len(fields) != 2 || fields[0] != revision || fields[1] != "refs/heads/release/"+p.version {
		return errors.New("remote release branch does not point to the clean Soda source revision")
	}
	return nil
}
