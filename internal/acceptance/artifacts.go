package acceptance

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/LevitateOS/soda-os/internal/build/oci"
)

type ArtifactSet struct {
	OCI   string
	ISO   string
	QCOW2 string
}

type ValidatedArtifacts struct {
	Candidate      oci.Image
	Fallback       oci.Image
	CandidateOCI   string
	FallbackOCI    string
	CandidateISO   string
	CandidateQCOW2 string
}

func ValidateArtifacts(candidate, fallback ArtifactSet) (ValidatedArtifacts, error) {
	if nativeArchitecture() == "" {
		return ValidatedArtifacts{}, fmt.Errorf("acceptance requires matching-native x86-64 or AArch64, not %s", runtime.GOARCH)
	}
	candidateImage, err := oci.Inspect(candidate.OCI, runtime.GOARCH)
	if err != nil {
		return ValidatedArtifacts{}, fmt.Errorf("candidate OCI: %w", err)
	}
	// The earlier image supplies its own version/base, never the current spec's.
	fallbackImage, err := oci.Inspect(fallback.OCI, runtime.GOARCH)
	if err != nil {
		return ValidatedArtifacts{}, fmt.Errorf("fallback OCI: %w", err)
	}
	if candidateImage.Digest == fallbackImage.Digest || candidateImage.Revision == fallbackImage.Revision {
		return ValidatedArtifacts{}, errors.New("candidate and fallback must have distinct image digests and source revisions")
	}
	for _, path := range []string{candidate.ISO, candidate.QCOW2} {
		if err := requireChecksumSidecar(path); err != nil {
			return ValidatedArtifacts{}, err
		}
	}
	return ValidatedArtifacts{
		Candidate: candidateImage, Fallback: fallbackImage,
		CandidateOCI: candidate.OCI, FallbackOCI: fallback.OCI,
		CandidateISO: candidate.ISO, CandidateQCOW2: candidate.QCOW2,
	}, nil
}

func requireChecksumSidecar(path string) error {
	for _, input := range []string{path, path + ".sha256"} {
		if err := requireRegularFile(input); err != nil {
			return err
		}
	}
	contents, err := os.ReadFile(path + ".sha256")
	if err != nil {
		return err
	}
	fields := strings.Fields(string(contents))
	if len(fields) != 2 || !validHex(fields[0]) || fields[1] != filepath.Base(path) {
		return fmt.Errorf("checksum sidecar for %s must name its exact file and SHA-256", path)
	}
	actual, err := fileSHA256(path)
	if err != nil {
		return err
	}
	if actual != fields[0] {
		return fmt.Errorf("artifact %s checksum does not match sidecar", path)
	}
	return nil
}

func validHex(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func requireRegularFile(path string) error {
	if path == "" {
		return errors.New("artifact path is required")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect artifact %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("artifact %s must be a regular non-symlink file", path)
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err = io.Copy(digest, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func exactReference(reference string) bool {
	parts := strings.Split(reference, "@sha256:")
	return len(parts) == 2 && parts[0] != "" && validHex(parts[1])
}
