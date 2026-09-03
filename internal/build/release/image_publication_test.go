package release

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImageStageUsesAnImmutableCandidateTagAndVerifiesTheDigest(t *testing.T) {
	image := matchingTestImage(t)
	archive := writeOCIArchive(t, image)
	digest, err := image.Digest()
	require.NoError(t, err)
	runner := &publicationRunner{
		revision:     testRevision,
		imageTags:    []string{`{"Tags":[]}`},
		imageDigests: []string{digest.String()},
	}
	publication := testPublication(t, runner, "arm64")
	result, err := publication.ImageStage(context.Background(), ImageStageOptions{Architecture: "aarch64", ArchivePath: archive})
	require.NoError(t, err)
	require.Equal(t, Repository+"@"+digest.String(), result.Reference)
	commands := commandStrings(runner.commands)
	candidate := Repository + ":sha-" + testRevision + "-aarch64"
	require.Contains(t, commands, "skopeo list-tags docker://"+Repository)
	require.Contains(t, commands, "skopeo copy --dest-tls-verify=true oci-archive:"+archive+" docker://"+candidate)
	require.Contains(t, commands, "skopeo inspect --format {{.Digest}} docker://"+candidate)
}

func TestImageStageRefusesAnExistingCandidateBeforePublishing(t *testing.T) {
	runner := &publicationRunner{revision: testRevision, imageTags: []string{`{"Tags":["sha-` + testRevision + `-aarch64"]}`}}
	publication := testPublication(t, runner, "arm64")
	_, err := publication.ImageStage(context.Background(), ImageStageOptions{Architecture: "aarch64", ArchivePath: writeOCIArchive(t, matchingTestImage(t))})
	require.ErrorContains(t, err, "already exists")
	require.NotContains(t, strings.Join(commandStrings(runner.commands), "\n"), "skopeo copy")
}

func TestImagePromoteRequiresTheCandidateAndNeverMovesAnExistingVersionTag(t *testing.T) {
	options, _ := writeUploadArtifacts(t, testArmPublicationSpec(), testRevision)
	record, err := readStrictRecord(options.RecordPath)
	require.NoError(t, err)
	digest := strings.TrimPrefix(record.SodaImageReference, Repository+"@")
	runner := &publicationRunner{
		revision: testRevision,
		imageTags: []string{
			`{"Tags":["sha-` + testRevision + `-aarch64"]}`,
			`{"Tags":["sha-` + testRevision + `-aarch64"]}`,
		},
		imageDigests: []string{digest, digest},
	}
	publication := testPublication(t, runner, "arm64")
	result, err := publication.ImagePromote(context.Background(), ImagePromoteOptions{Architecture: "aarch64", RecordPath: options.RecordPath})
	require.NoError(t, err)
	require.Equal(t, record.SodaImageReference, result.Reference)
	commands := strings.Join(commandStrings(runner.commands), "\n")
	candidate := Repository + ":sha-" + testRevision + "-aarch64"
	version := Repository + ":0.2.0-aarch64"
	require.Contains(t, commands, "skopeo copy --src-tls-verify=true --dest-tls-verify=true docker://"+candidate+" docker://"+version)

	locked := &publicationRunner{revision: testRevision, imageTags: []string{`{"Tags":["sha-` + testRevision + `-aarch64","0.2.0-aarch64"]}`}, imageDigests: []string{digest}}
	_, err = testPublication(t, locked, "arm64").ImagePromote(context.Background(), ImagePromoteOptions{Architecture: "aarch64", RecordPath: options.RecordPath})
	require.ErrorContains(t, err, "already exists")
	require.NotContains(t, strings.Join(commandStrings(locked.commands), "\n"), "skopeo copy")
	require.NotContains(t, strings.Join(commandStrings(locked.commands), "\n"), "cosign")
}
