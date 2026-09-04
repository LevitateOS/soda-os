package image

import (
	"context"
	"errors"
	"fmt"
)

type imageBuildInputs struct{ revision, baseTag string }

func (b *Builder) prepareImageBuildInputs(ctx context.Context) (imageBuildInputs, error) {
	if err := b.requireNativeHost(); err != nil {
		return imageBuildInputs{}, err
	}
	if err := b.Check(ctx); err != nil {
		return imageBuildInputs{}, err
	}
	revision, err := b.sourceRevision(ctx)
	if err != nil {
		return imageBuildInputs{}, err
	}
	if err := b.verifyFetchedBuildInputs(); err != nil {
		return imageBuildInputs{}, err
	}
	baseTag, err := PrepareLocalBootcBase(ctx, b.Root, b.runner, b.Spec.Platform)
	if err != nil {
		return imageBuildInputs{}, err
	}
	return imageBuildInputs{revision: revision, baseTag: baseTag}, nil
}

// verifyFetchedBuildInputs checks every downloaded input before Docker creates
// the RPM builder or runs an upstream build.
func (b *Builder) verifyFetchedBuildInputs() error {
	return errors.Join(
		b.verifyForgejoInput(),
		b.verifyTeaInput(),
		b.verifyGitHubRunnerInput(),
		b.verifyMiseInput(),
	)
}

func (b *Builder) verifyForgejoInput() error {
	lock, err := readForgejoSourceLock(b.path("distro/locks/forgejo-source.toml"))
	if err != nil {
		return err
	}
	if err := verifyFileSHA256(b.artifactPath("tools", lock.SourceArchive), lock.SHA256); err != nil {
		return fmt.Errorf("verify Forgejo source; run just forgejo-source: %w", err)
	}
	return verifyFileSHA256(b.path("packaging/rpm/forgejo/sources/patches/0001-pam-do-not-retain-password.patch"), lock.PatchSHA256)
}

func (b *Builder) verifyTeaInput() error {
	lock, err := readTeaSourceLock(b.path("distro/locks/tea-source.toml"))
	if err != nil {
		return err
	}
	if err := verifyFileSHA256(b.artifactPath("tools", lock.SourceArchive), lock.SourceSHA256); err != nil {
		return fmt.Errorf("verify Tea source; run just tea-source: %w", err)
	}
	return verifyFileSHA256(b.path("packaging/rpm/tea/sources/LICENSE"), lock.LicenseSHA256)
}

func (b *Builder) verifyGitHubRunnerInput() error {
	lock, err := readGitHubRunnerSourceLock(b.path("distro/locks/github-runner-source.toml"))
	if err != nil {
		return err
	}
	asset, err := lock.asset(b.Spec.Platform.Architecture.Name)
	if err != nil {
		return err
	}
	if err := verifyFileSHA256(b.artifactPath("tools", asset.Archive), asset.SHA256); err != nil {
		return fmt.Errorf("verify GitHub runner source; run just github-runner %s: %w", asset.Architecture, err)
	}
	return nil
}

func (b *Builder) verifyMiseInput() error {
	lock, err := readMiseSourceLock(b.path("distro/locks/mise-source.toml"))
	if err != nil {
		return err
	}
	asset, ok := lock.Asset[b.Spec.Platform.Architecture.Name]
	if !ok {
		return fmt.Errorf("mise source lock has no %s asset", b.Spec.Platform.Architecture.Name)
	}
	if err := verifyFileSHA256(b.artifactPath("tools", asset.File), asset.SHA256); err != nil {
		return fmt.Errorf("verify mise RPM; run just mise-rpm on matching-native %s: %w", b.Spec.Platform.Architecture.Name, err)
	}
	return nil
}
