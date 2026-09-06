package acceptance

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/build/oci/ocitest"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/stretchr/testify/require"
)

func TestValidateArtifactsUsesNativeBytesAndIndependentFallback(t *testing.T) {
	directory := t.TempDir()
	candidate := writeArtifactFixture(t, directory, "candidate", strings.Repeat("a", 40))
	fallback := writeArtifactFixture(t, directory, "fallback", strings.Repeat("b", 40))
	require.NoError(t, os.Remove(fallback.ISO))
	require.NoError(t, os.Remove(fallback.QCOW2))
	validated, err := ValidateArtifacts(candidate, fallback)
	require.NoError(t, err)
	require.Equal(t, strings.Repeat("a", 40), validated.Candidate.Revision)
	require.Equal(t, strings.Repeat("b", 40), validated.Fallback.Revision)
	require.NotEqual(t, validated.Candidate.Version, validated.Fallback.Version)
	require.NotEqual(t, validated.Candidate.BaseReference, validated.Fallback.BaseReference)
	_, err = ValidateArtifacts(fallback, candidate)
	require.Error(t, err, "candidate installer files are required")
	_, err = ValidateArtifacts(candidate, candidate)
	require.ErrorContains(t, err, "distinct image digests")
}

func TestValidateArtifactsAllowsSameVersionDifferentRevisions(t *testing.T) {
	directory := t.TempDir()
	candidate := writeArtifactFixture(t, directory, "candidate", strings.Repeat("a", 40))
	fallback := writeArtifactFixture(t, directory, "earlier", strings.Repeat("b", 40))
	validated, err := ValidateArtifacts(candidate, fallback)
	require.NoError(t, err)
	require.Equal(t, validated.Candidate.Version, validated.Fallback.Version)
	require.NotEqual(t, validated.Candidate.Digest, validated.Fallback.Digest)
}

func TestValidateArtifactsRejectsBadFilesAndSidecars(t *testing.T) {
	for _, fault := range []string{"symlink", "checksum", "name", "missing", "oci"} {
		t.Run(fault, func(t *testing.T) {
			directory := t.TempDir()
			candidate := writeArtifactFixture(t, directory, "candidate", strings.Repeat("a", 40))
			fallback := writeArtifactFixture(t, directory, "fallback", strings.Repeat("b", 40))
			switch fault {
			case "symlink":
				target := candidate.ISO
				candidate.ISO += ".link"
				require.NoError(t, os.Symlink(target, candidate.ISO))
			case "checksum":
				require.NoError(t, os.WriteFile(candidate.QCOW2, []byte("changed"), 0o600))
			case "name":
				require.NoError(t, os.WriteFile(candidate.ISO+".sha256", []byte(checksumString("iso")+"  other.iso\n"), 0o600))
			case "missing":
				require.NoError(t, os.Remove(candidate.ISO+".sha256"))
			case "oci":
				require.NoError(t, os.WriteFile(candidate.OCI, []byte("invalid OCI"), 0o600))
			}
			_, err := ValidateArtifacts(candidate, fallback)
			require.Error(t, err)
		})
	}
}

func writeArtifactFixture(t *testing.T, directory, name, revision string) ArtifactSet {
	t.Helper()
	set := ArtifactSet{OCI: filepath.Join(directory, name+".oci.tar"), ISO: filepath.Join(directory, name+".iso"), QCOW2: filepath.Join(directory, name+".qcow2")}
	version := "0.6.3"
	if name == "fallback" {
		version = "0.5.0"
	}
	img := ocitest.Image(t, &v1.ConfigFile{Architecture: runtime.GOARCH, OS: "linux", Config: v1.Config{Labels: map[string]string{
		"org.opencontainers.image.version":   version,
		"org.opencontainers.image.revision":  revision,
		"org.opencontainers.image.base.name": "quay.io/fedora/fedora-bootc@sha256:" + strings.Repeat(revision[:1], 64),
	}}})
	require.NoError(t, copyFile(ocitest.Archive(t, img, runtime.GOARCH), set.OCI))
	for _, item := range []struct{ path, value string }{{set.ISO, "iso"}, {set.QCOW2, "qcow2"}} {
		require.NoError(t, os.WriteFile(item.path, []byte(item.value), 0o600))
		require.NoError(t, os.WriteFile(item.path+".sha256", []byte(checksumString(item.value)+"  "+filepath.Base(item.path)+"\n"), 0o600))
	}
	return set
}

func checksumString(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}
