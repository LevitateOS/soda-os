package release

import (
	"context"
	"errors"
	"fmt"
	"os"
)

type RecordSignOptions struct {
	Architecture string
	RecordPath   string
}

type RecordSignResult struct {
	Architecture string
	BundlePath   string
	Revision     string
}

// SignRecord keylessly signs one matching-native release record. The fixed
// bundle path becomes a GitHub Release asset beside the record it verifies.
func (p *Publication) SignRecord(ctx context.Context, options RecordSignOptions) (RecordSignResult, error) {
	spec, err := p.nativeSpec(options.Architecture)
	if err != nil {
		return RecordSignResult{}, err
	}
	revision, err := p.cleanRevision(ctx)
	if err != nil {
		return RecordSignResult{}, err
	}
	if p.workflowRunURL == "" {
		return RecordSignResult{}, errors.New("release-record signing requires a GitHub Actions workflow run identity")
	}
	record, err := readStrictRecord(options.RecordPath)
	if err != nil {
		return RecordSignResult{}, err
	}
	if err := validatePublicationRecord(record, spec, revision); err != nil {
		return RecordSignResult{}, err
	}
	bundle := options.RecordPath + ".sigstore.json"
	if _, err := os.Lstat(bundle); err == nil {
		return RecordSignResult{}, errors.New("release-record bundle already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return RecordSignResult{}, fmt.Errorf("inspect release-record bundle: %w", err)
	}
	if err := p.runner.Run(ctx, p.cosign("sign-blob", "--yes", "--bundle", bundle, options.RecordPath)); err != nil {
		return RecordSignResult{}, fmt.Errorf("keylessly sign release record: %w", err)
	}
	if err := p.runner.Run(ctx, p.cosign("verify-blob", "--bundle", bundle, "--certificate-identity", p.releaseWorkflowIdentity(), "--certificate-oidc-issuer", githubOIDCIssuer, options.RecordPath)); err != nil {
		return RecordSignResult{}, fmt.Errorf("verify signed release record: %w", err)
	}
	return RecordSignResult{Architecture: options.Architecture, BundlePath: bundle, Revision: revision}, nil
}
