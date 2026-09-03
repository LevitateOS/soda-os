package release

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSignRecordUsesTheFixedBundleAndWorkflowIdentity(t *testing.T) {
	options, _ := writeUploadArtifacts(t, testArmPublicationSpec(), testRevision)
	require.NoError(t, os.Remove(options.RecordBundlePath))
	runner := &publicationRunner{revision: testRevision}
	publication := testPublication(t, runner, "arm64")

	result, err := publication.SignRecord(context.Background(), RecordSignOptions{Architecture: "aarch64", RecordPath: options.RecordPath})
	require.NoError(t, err)
	require.Equal(t, options.RecordPath+".sigstore.json", result.BundlePath)
	require.Equal(t, []string{"sign-blob", "--yes", "--bundle", result.BundlePath, options.RecordPath}, runner.commands[2].Args)
	require.Equal(t, []string{"verify-blob", "--bundle", result.BundlePath, "--certificate-identity", publication.releaseWorkflowIdentity(), "--certificate-oidc-issuer", githubOIDCIssuer, options.RecordPath}, runner.commands[3].Args)
}

func TestSignRecordRejectsExistingBundleBeforeCosign(t *testing.T) {
	options, _ := writeUploadArtifacts(t, testArmPublicationSpec(), testRevision)
	runner := &publicationRunner{revision: testRevision}
	publication := testPublication(t, runner, "arm64")
	_, err := publication.SignRecord(context.Background(), RecordSignOptions{Architecture: "aarch64", RecordPath: options.RecordPath})
	require.ErrorContains(t, err, "already exists")
	require.NotContains(t, strings.Join(commandStrings(runner.commands), "\n"), "cosign")
}
