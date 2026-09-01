package release

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/stretchr/testify/require"
)

const testRevision = "2b6b23e356ded84d4ef7fee52b242ae4855793ca"

func TestCreateRecordUsesLocalArchiveDigest(t *testing.T) {
	img := matchingTestImage(t)
	archive := writeOCIArchive(t, img)
	publisher := &Publisher{spec: testSpec(), hostArchitecture: "arm64"}
	output := t.TempDir()

	result, err := publisher.CreateRecord(context.Background(), RecordOptions{ArchivePath: archive, OutputDir: output})
	require.NoError(t, err)
	digest, err := img.Digest()
	require.NoError(t, err)
	exact := Repository + "@" + digest.String()
	require.Equal(t, exact, result.ImageReference)

	contents, err := os.ReadFile(result.RecordPath)
	require.NoError(t, err)
	require.True(t, bytes.HasSuffix(contents, []byte("\n")))
	var record Record
	require.NoError(t, json.Unmarshal(contents, &record))
	require.Equal(t, exact, record.SodaImageReference)
	require.Equal(t, testRevision, record.SourceRevision)
	require.Equal(t, sha256Hex([]byte("rpm inventory\n")), record.RPMInventorySHA256)
	require.NotContains(t, string(contents), "state_schema")
}

func TestCreateRecordBindsInspectedISO(t *testing.T) {
	iso := filepath.Join(t.TempDir(), "SodaOS.iso")
	require.NoError(t, os.WriteFile(iso, []byte("installer bytes"), 0o644))
	validator := &fakeISOValidator{}
	publisher := &Publisher{spec: testSpec(), hostArchitecture: "arm64", isoValidator: validator}

	result, err := publisher.CreateRecord(context.Background(), RecordOptions{ArchivePath: writeOCIArchive(t, matchingTestImage(t)), ISOPath: iso, OutputDir: t.TempDir()})
	require.NoError(t, err)
	require.Equal(t, 1, validator.calls)
	contents, err := os.ReadFile(result.RecordPath)
	require.NoError(t, err)
	var record Record
	require.NoError(t, json.Unmarshal(contents, &record))
	require.Equal(t, sha256Hex([]byte("installer bytes")), record.ISOChecksum)
}

func TestCreateRecordRejectsMismatchedHostBeforeInspectingArtifacts(t *testing.T) {
	publisher := &Publisher{spec: testSpec(), hostArchitecture: "amd64"}
	_, err := publisher.CreateRecord(context.Background(), RecordOptions{})
	require.EqualError(t, err, "Soda aarch64 artifact operations require a native arm64 host; running on amd64")
}

func TestInspectRejectsRPMInventorySidecarMismatch(t *testing.T) {
	publisher := &Publisher{spec: testSpec()}
	_, err := publisher.inspect(testImageWithSidecar(t, strings.Repeat("0", 64)), Repository+"@sha256:"+strings.Repeat("a", 64))
	require.EqualError(t, err, "installed RPM inventory does not match its image sidecar")
}

func TestReleaseIndexRequiresTwoMatchingSiblingArtifacts(t *testing.T) {
	root := t.TempDir()
	publisher := &Publisher{spec: testSpec()}
	artifacts := map[string]ReleaseArtifact{}
	for architecture, digest := range map[string]string{"aarch64": strings.Repeat("a", 64), "x86_64": strings.Repeat("b", 64)} {
		isoPath := filepath.Join(root, architecture+".iso")
		require.NoError(t, os.WriteFile(isoPath, []byte(architecture+" installer"), 0o644))
		checksum, err := fileSHA256(isoPath)
		require.NoError(t, err)
		record := Record{SodaVersion: "0.2.0", SourceRevision: testRevision, SodaImageReference: Repository + "@sha256:" + digest, ISOChecksum: checksum}
		recordPath := filepath.Join(root, architecture+".release.json")
		encoded, err := json.Marshal(record)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(recordPath, encoded, 0o644))
		artifacts[architecture] = ReleaseArtifact{ISOPath: isoPath, RecordPath: recordPath}
	}
	index, paths, err := publisher.releaseIndex(artifacts)
	require.NoError(t, err)
	require.Equal(t, []string{"aarch64", "x86_64"}, []string{index.Releases[0].Architecture, index.Releases[1].Architecture})
	require.Len(t, paths, 4)

	contents, err := os.ReadFile(artifacts["x86_64"].RecordPath)
	require.NoError(t, err)
	var mismatched Record
	require.NoError(t, json.Unmarshal(contents, &mismatched))
	mismatched.SourceRevision = strings.Repeat("c", 40)
	encoded, err := json.Marshal(mismatched)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(artifacts["x86_64"].RecordPath, encoded, 0o644))
	_, _, err = publisher.releaseIndex(artifacts)
	require.EqualError(t, err, "paired release records have different source revisions")
}

type fakeGitHubReleaseClient struct{ events []string }

func (f *fakeGitHubReleaseClient) CreateDraft(_ context.Context, repository, tag, title string) (githubDraft, error) {
	f.events = append(f.events, "draft:"+repository+":"+tag+":"+title)
	return githubDraft{ID: 7}, nil
}
func (f *fakeGitHubReleaseClient) Upload(_ context.Context, _ githubDraft, path string) error {
	f.events = append(f.events, "upload:"+filepath.Base(path))
	return nil
}
func (f *fakeGitHubReleaseClient) VerifyAssets(_ context.Context, _ githubDraft, paths []string) error {
	f.events = append(f.events, fmt.Sprintf("verify:%d", len(paths)))
	return nil
}
func (f *fakeGitHubReleaseClient) Publish(_ context.Context, _ githubDraft) error {
	f.events = append(f.events, "publish")
	return nil
}

func TestPairedGitHubReleasePublishesOnlyAfterUploadVerification(t *testing.T) {
	client := &fakeGitHubReleaseClient{}
	_, err := publishPaired(context.Background(), client, pairedUpload{repository: "LevitateOS/soda-os", tag: "v0.2.0", indexPath: "/tmp/index.json", paths: []string{"/tmp/a.iso", "/tmp/x.iso"}})
	require.NoError(t, err)
	require.Equal(t, []string{"draft:LevitateOS/soda-os:v0.2.0:Soda OS 0.2.0", "upload:a.iso", "upload:x.iso", "verify:2", "publish"}, client.events)
}

func TestOCIArchiveRequiresExactlyOneArm64Manifest(t *testing.T) {
	img := matchingTestImage(t)
	index := mutate.AppendManifests(empty.Index,
		mutate.IndexAddendum{Add: img, Descriptor: v1.Descriptor{Platform: &v1.Platform{OS: "linux", Architecture: "arm64"}}},
		mutate.IndexAddendum{Add: img, Descriptor: v1.Descriptor{Platform: &v1.Platform{OS: "linux", Architecture: "amd64"}}},
	)
	archive := writeIndexArchive(t, index)
	_, cleanup, err := imageFromOCIArchive(archive, "arm64")
	defer cleanup()
	require.EqualError(t, err, "OCI archive must contain exactly one manifest")
}
