package acceptance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/build/release"
	"github.com/stretchr/testify/require"
)

func TestValidateArtifactsRequiresMatchingNativeRecordsAndChecksums(t *testing.T) {
	directory := t.TempDir()
	candidate := writeArtifactFixture(t, directory, "candidate", strings.Repeat("a", 40))
	fallback := writeArtifactFixture(t, directory, "fallback", strings.Repeat("b", 40))

	validated, err := ValidateArtifacts(candidate, fallback)
	require.NoError(t, err)
	require.Equal(t, strings.Repeat("a", 40), validated.Candidate.SourceRevision)
	require.Equal(t, strings.Repeat("b", 40), validated.Fallback.SourceRevision)
}

func TestValidateArtifactsAcceptsSchema4CandidateAndSchema3Fallback(t *testing.T) {
	directory := t.TempDir()
	candidate := writeArtifactFixture(t, directory, "candidate", strings.Repeat("a", 40))
	fallback := writeArtifactFixture(t, directory, "fallback", strings.Repeat("b", 40))
	contents, err := os.ReadFile(candidate.Record)
	require.NoError(t, err)
	var record releaseRecord
	require.NoError(t, json.Unmarshal(contents, &record))
	record.SchemaVersion = 4
	record.RuntimePackageLock = "distro/locks/runtime-packages-x86_64.toml"
	record.RuntimeLockSHA256 = strings.Repeat("a", 64)
	contents, err = json.Marshal(record)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(candidate.Record, contents, 0o600))

	_, err = ValidateArtifacts(candidate, fallback)
	require.NoError(t, err)
}

func TestValidateArtifactsRejectsSymlink(t *testing.T) {
	directory := t.TempDir()
	candidate := writeArtifactFixture(t, directory, "candidate", strings.Repeat("a", 40))
	fallback := writeArtifactFixture(t, directory, "fallback", strings.Repeat("b", 40))
	target := candidate.ISO
	candidate.ISO = filepath.Join(directory, "candidate-link.iso")
	require.NoError(t, os.Symlink(target, candidate.ISO))

	_, err := ValidateArtifacts(candidate, fallback)
	require.ErrorContains(t, err, "regular non-symlink")
}

func writeArtifactFixture(t *testing.T, directory, name, revision string) ArtifactSet {
	t.Helper()
	set := ArtifactSet{
		Record: filepath.Join(directory, name+".release.json"),
		OCI:    filepath.Join(directory, name+".oci.tar"),
		ISO:    filepath.Join(directory, name+".iso"),
		QCOW2:  filepath.Join(directory, name+".qcow2"),
	}
	for _, item := range []struct{ path, value string }{{set.OCI, "oci"}, {set.ISO, "iso"}, {set.QCOW2, "qcow2"}} {
		require.NoError(t, os.WriteFile(item.path, []byte(item.value), 0o600))
	}
	platform := map[string]string{"amd64": "linux/amd64", "arm64": "linux/arm64"}[runtime.GOARCH]
	record := releaseRecord{
		SchemaVersion: 3, SodaVersion: "0.5.0", SourceRevision: revision,
		Platform: platform, Channel: "stable", FedoraBaseReference: "quay.io/fedora/fedora-bootc@sha256:" + strings.Repeat("f", 64),
		SodaImageReference: release.Repository + "@sha256:" + strings.Repeat(name[:1], 64),
		ArtifactChecksums: release.ArtifactChecksums{
			RPMInventorySHA256: strings.Repeat("e", 64),
			ISOChecksum:        checksumString("iso"),
			QCOW2Checksum:      checksumString("qcow2"),
			QCOW2ZSTChecksum:   strings.Repeat("d", 64),
		},
	}
	contents, err := json.Marshal(record)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(set.Record, contents, 0o600))
	return set
}

func checksumString(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}
