package release

import (
	"context"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/process"
	"github.com/stretchr/testify/require"
)

func TestPublishRequiresCompleteAssetsAndPreservesThem(t *testing.T) {
	assets := requiredRemoteAssets("0.2.0")
	assets = append(assets, remoteAsset{Name: "SodaOS-0.2.0-x86_64.iso.sig", Size: 64, State: "uploaded", Digest: "sha256:" + strings.Repeat("c", 64)})
	runner := &publicationRunner{
		revision: testRevision,
		states: []string{
			repositoryStateJSON(testRevision, draftRelease),
			repositoryStateJSON(testRevision, publishedRelease),
		},
		views: []string{releaseViewJSON(draftRelease, assets), releaseViewJSON(publishedRelease, assets)},
		imageDigests: []string{
			"sha256:" + strings.Repeat("a", 64), "sha256:" + strings.Repeat("a", 64),
			"sha256:" + strings.Repeat("a", 64), "sha256:" + strings.Repeat("a", 64),
		},
		remoteBranches: []string{testRevision + "\trefs/heads/production\n"},
	}
	publication := testPublication(t, runner, "arm64")
	options := writePublishOptions(t, testRevision)
	result, err := publication.Publish(context.Background(), options)
	require.NoError(t, err)
	require.Len(t, result.Assets, 13)
	var edit process.Command
	for _, command := range runner.commands {
		if strings.HasPrefix(command.String(), "gh release edit ") {
			edit = command
		}
	}
	require.Equal(t, []string{"release", "edit", "v0.2.0", "--repo", "LevitateOS/soda-os", "--verify-tag", "--draft=false", "--latest"}, edit.Args)
	requireGHEnvironment(t, runner.commands)
	cosignCommands := make([]process.Command, 0, 6)
	for _, command := range runner.commands {
		if command.Name == "cosign" {
			cosignCommands = append(cosignCommands, command)
		}
	}
	require.Len(t, cosignCommands, 6)
	require.Equal(t, []string{"verify-blob", "--bundle", options.AArch64RecordPath + ".sigstore.json", "--certificate-identity", publication.releaseWorkflowIdentity(), "--certificate-oidc-issuer", githubOIDCIssuer, options.AArch64RecordPath}, cosignCommands[0].Args)
	require.Equal(t, []string{"verify", "--certificate-identity", publication.releaseWorkflowIdentity(), "--certificate-oidc-issuer", githubOIDCIssuer, "ghcr.io/levitateos/soda-os@sha256:" + strings.Repeat("a", 64)}, cosignCommands[2].Args)
	require.Equal(t, []string{"verify-attestation", "--type", "slsaprovenance", "--certificate-identity", publication.releaseWorkflowIdentity(), "--certificate-oidc-issuer", githubOIDCIssuer, "ghcr.io/levitateos/soda-os@sha256:" + strings.Repeat("a", 64)}, cosignCommands[3].Args)
}

func TestPublishRefusesRemoteBranchOrAnonymousDigestMismatchBeforeReleaseMutation(t *testing.T) {
	assets := requiredRemoteAssets("0.2.0")
	options := writePublishOptions(t, testRevision)
	for name, runner := range map[string]*publicationRunner{
		"anonymous digest": {revision: testRevision, states: []string{repositoryStateJSON(testRevision, draftRelease)}, views: []string{releaseViewJSON(draftRelease, assets)}, imageDigests: []string{"sha256:" + strings.Repeat("a", 64), "sha256:" + strings.Repeat("b", 64)}},
		"remote branch": {
			revision: testRevision, states: []string{repositoryStateJSON(testRevision, draftRelease)}, views: []string{releaseViewJSON(draftRelease, assets)},
			imageDigests:   []string{"sha256:" + strings.Repeat("a", 64), "sha256:" + strings.Repeat("a", 64), "sha256:" + strings.Repeat("a", 64), "sha256:" + strings.Repeat("a", 64)},
			remoteBranches: []string{strings.Repeat("b", 40) + "\trefs/heads/production\n"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := testPublication(t, runner, "arm64").Publish(context.Background(), options)
			require.Error(t, err)
			require.NotContains(t, strings.Join(commandStrings(runner.commands), "\n"), "gh release edit")
		})
	}
}

func TestPublishRefusesIncompleteDraftWithoutMutation(t *testing.T) {
	runner := &publicationRunner{revision: testRevision, states: []string{repositoryStateJSON(testRevision, draftRelease)}, views: []string{releaseViewJSON(draftRelease, requiredRemoteAssets("0.2.0")[:5])}}
	_, err := testPublication(t, runner, "arm64").Publish(context.Background(), writePublishOptions(t, testRevision))
	require.ErrorContains(t, err, "missing required asset")
	require.NotContains(t, strings.Join(commandStrings(runner.commands), "\n"), "gh release edit")
}

func TestPublishRejectsChangedOrInvalidRemoteAssets(t *testing.T) {
	t.Run("invalid extra asset", func(t *testing.T) {
		assets := append(requiredRemoteAssets("0.2.0"), remoteAsset{Name: "signature.sig", Size: 1, State: "uploaded"})
		runner := &publicationRunner{revision: testRevision, states: []string{repositoryStateJSON(testRevision, draftRelease)}, views: []string{releaseViewJSON(draftRelease, assets)}}
		_, err := testPublication(t, runner, "arm64").Publish(context.Background(), writePublishOptions(t, testRevision))
		require.ErrorContains(t, err, "not fully uploaded with a SHA-256 digest")
		require.NotContains(t, strings.Join(commandStrings(runner.commands), "\n"), "gh release edit")
	})

	t.Run("asset changed while publishing", func(t *testing.T) {
		before := requiredRemoteAssets("0.2.0")
		after := append([]remoteAsset(nil), before...)
		after[0].Size++
		runner := &publicationRunner{
			revision:       testRevision,
			states:         []string{repositoryStateJSON(testRevision, draftRelease), repositoryStateJSON(testRevision, publishedRelease)},
			views:          []string{releaseViewJSON(draftRelease, before), releaseViewJSON(publishedRelease, after)},
			imageDigests:   []string{"sha256:" + strings.Repeat("a", 64), "sha256:" + strings.Repeat("a", 64), "sha256:" + strings.Repeat("a", 64), "sha256:" + strings.Repeat("a", 64)},
			remoteBranches: []string{testRevision + "\trefs/heads/production\n"},
		}
		_, err := testPublication(t, runner, "arm64").Publish(context.Background(), writePublishOptions(t, testRevision))
		require.EqualError(t, err, "GitHub release assets changed while publishing")
		require.Equal(t, 1, strings.Count(strings.Join(commandStrings(runner.commands), "\n"), "gh release edit"))
	})
}
