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
	iso := filepath.Join(t.TempDir(), "SodaOS.iso")
	require.NoError(t, os.WriteFile(iso, []byte("installer bytes"), 0o644))
	qcow2, qcow2ZST := writeQCOW2Artifacts(t)
	publisher := &Publisher{spec: testSpec(), hostArchitecture: "arm64"}
	publisher.isoValidator = &fakeISOValidator{}
	output := t.TempDir()

	result, err := publisher.CreateRecord(context.Background(), RecordOptions{ArchivePath: archive, ISOPath: iso, QCOW2Path: qcow2, QCOW2ZSTPath: qcow2ZST, OutputDir: output})
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
	require.Equal(t, sha256Hex([]byte("raw QCOW2")), record.QCOW2Checksum)
	require.Equal(t, sha256Hex([]byte("compressed QCOW2")), record.QCOW2ZSTChecksum)
	require.NotContains(t, string(contents), "state_schema")
}

func TestCreateRecordBindsInspectedISO(t *testing.T) {
	iso := filepath.Join(t.TempDir(), "SodaOS.iso")
	require.NoError(t, os.WriteFile(iso, []byte("installer bytes"), 0o644))
	qcow2, qcow2ZST := writeQCOW2Artifacts(t)
	validator := &fakeISOValidator{}
	publisher := &Publisher{spec: testSpec(), hostArchitecture: "arm64", isoValidator: validator}

	result, err := publisher.CreateRecord(context.Background(), RecordOptions{ArchivePath: writeOCIArchive(t, matchingTestImage(t)), ISOPath: iso, QCOW2Path: qcow2, QCOW2ZSTPath: qcow2ZST, OutputDir: t.TempDir()})
	require.NoError(t, err)
	require.Equal(t, 1, validator.calls)
	contents, err := os.ReadFile(result.RecordPath)
	require.NoError(t, err)
	var record Record
	require.NoError(t, json.Unmarshal(contents, &record))
	require.Equal(t, sha256Hex([]byte("installer bytes")), record.ISOChecksum)
}

func TestCreateRecordRequiresEveryReleaseArtifact(t *testing.T) {
	publisher := &Publisher{spec: testSpec(), hostArchitecture: "arm64"}
	_, err := publisher.CreateRecord(context.Background(), RecordOptions{ArchivePath: writeOCIArchive(t, matchingTestImage(t))})
	require.EqualError(t, err, "release record requires the installer ISO, raw QCOW2, and compressed QCOW2")
}

func TestQCOW2InspectionRejectsUnsafeAndMismatchedArtifacts(t *testing.T) {
	raw, compressed := writeQCOW2Artifacts(t)
	_, _, err := inspectQCOW2Artifacts(raw, compressed)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(compressed+".sha256", []byte("invalid\n"), 0o644))
	_, _, err = inspectQCOW2Artifacts(raw, compressed)
	require.ErrorContains(t, err, "checksum sidecar differs")

	require.NoError(t, os.Remove(compressed+".sha256"))
	require.NoError(t, os.Symlink(raw, compressed+".sha256"))
	_, _, err = inspectQCOW2Artifacts(raw, compressed)
	require.ErrorContains(t, err, "regular non-symlink")
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

func writeQCOW2Artifacts(t *testing.T) (string, string) {
	t.Helper()
	directory := t.TempDir()
	raw := filepath.Join(directory, "SodaOS-0.2.0-aarch64.qcow2")
	compressed := raw + ".zst"
	require.NoError(t, os.WriteFile(raw, []byte("raw QCOW2"), 0o644))
	require.NoError(t, os.WriteFile(compressed, []byte("compressed QCOW2"), 0o644))
	digest, err := fileSHA256(compressed)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(compressed+".sha256", []byte(digest+"  "+filepath.Base(compressed)+"\n"), 0o644))
	return raw, compressed
}
