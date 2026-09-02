package release

import (
	"bytes"
	"context"
	"encoding/json"
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
