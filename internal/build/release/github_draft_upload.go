package release

import (
	"context"
	"errors"
	"fmt"
)

func (p *Publication) Draft(ctx context.Context, options DraftOptions) (PublicationResult, error) {
	revision, err := p.validateDraftInput(ctx, options)
	if err != nil {
		return PublicationResult{}, err
	}
	if err := p.requireAbsentDraft(ctx, revision); err != nil {
		return PublicationResult{}, err
	}
	if err := p.createTag(ctx, revision); err != nil {
		return PublicationResult{}, fmt.Errorf("create GitHub release tag: %w", err)
	}
	if err := p.createDraft(ctx, options.NotesPath); err != nil {
		return PublicationResult{}, fmt.Errorf("create GitHub draft release: %w", err)
	}
	if err := p.requireEmptyDraft(ctx, revision); err != nil {
		return PublicationResult{}, err
	}
	return PublicationResult{Tag: p.tag(), Revision: revision}, nil
}

func (p *Publication) validateDraftInput(ctx context.Context, options DraftOptions) (string, error) {
	if err := requireRegularFile("release notes", options.NotesPath); err != nil {
		return "", err
	}
	revision, err := p.cleanRevision(ctx)
	if err != nil {
		return "", err
	}
	records, err := p.validateReleaseRecords(revision, options.AArch64RecordPath, options.X86RecordPath)
	if err != nil {
		return "", err
	}
	if err := requireReleaseNotesDigests(options.NotesPath, records); err != nil {
		return "", err
	}
	return revision, nil
}

func (p *Publication) requireAbsentDraft(ctx context.Context, revision string) error {
	if err := p.authenticate(ctx); err != nil {
		return err
	}
	state, err := p.repositoryState(ctx, revision)
	if err != nil {
		return err
	}
	return requireAbsentReleaseState(state, revision)
}

func (p *Publication) requireEmptyDraft(ctx context.Context, revision string) error {
	view, err := p.requireRelease(ctx, revision, draftRelease)
	if err != nil {
		return fmt.Errorf("verify GitHub draft release: %w", err)
	}
	if len(view.Assets) != 0 {
		return errors.New("new GitHub draft release is not empty")
	}
	return nil
}

func (p *Publication) Upload(ctx context.Context, options UploadOptions) (PublicationResult, error) {
	revision, artifacts, err := p.validateUploadInput(ctx, options)
	if err != nil {
		return PublicationResult{}, err
	}
	if _, err := p.requireDraftForUpload(ctx, revision, artifacts); err != nil {
		return PublicationResult{}, err
	}
	if err := p.uploadAssets(ctx, artifacts); err != nil {
		return PublicationResult{}, err
	}
	if err := p.verifyUpload(ctx, revision, artifacts); err != nil {
		return PublicationResult{}, err
	}
	return PublicationResult{Tag: p.tag(), Revision: revision, Assets: artifactNames(artifacts)}, nil
}

func (p *Publication) validateUploadInput(ctx context.Context, options UploadOptions) (string, []localAsset, error) {
	spec, err := p.nativeSpec(options.Architecture)
	if err != nil {
		return "", nil, err
	}
	revision, err := p.cleanRevision(ctx)
	if err != nil {
		return "", nil, err
	}
	artifacts, err := validateUploadArtifacts(spec, revision, options)
	if err != nil {
		return "", nil, err
	}
	return revision, artifacts, nil
}

func (p *Publication) requireDraftForUpload(ctx context.Context, revision string, artifacts []localAsset) (releaseView, error) {
	if err := p.authenticate(ctx); err != nil {
		return releaseView{}, err
	}
	view, err := p.requireRelease(ctx, revision, draftRelease)
	if err != nil {
		return releaseView{}, err
	}
	if err := requireUploadNamesAbsent(view.Assets, artifacts); err != nil {
		return releaseView{}, err
	}
	return view, nil
}

func (p *Publication) uploadAssets(ctx context.Context, artifacts []localAsset) error {
	paths := make([]string, 0, len(artifacts))
	for _, artifact := range artifacts {
		paths = append(paths, artifact.Path)
	}
	if err := p.runner.Run(ctx, p.gh(append([]string{"release", "upload", p.tag()}, append(paths, "--repo", p.repository)...)...)); err != nil {
		return fmt.Errorf("upload GitHub release assets: %w", err)
	}
	return nil
}

func (p *Publication) verifyUpload(ctx context.Context, revision string, artifacts []localAsset) error {
	view, err := p.requireRelease(ctx, revision, draftRelease)
	if err != nil {
		return fmt.Errorf("verify uploaded GitHub release assets: %w", err)
	}
	return verifyUploadedAssets(view.Assets, artifacts)
}
