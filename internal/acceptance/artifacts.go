package acceptance

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/LevitateOS/soda-os/internal/build/release"
)

type ArtifactSet struct {
	Record string
	OCI    string
	ISO    string
	QCOW2  string
}

type releaseRecord = release.Record

type ValidatedArtifacts struct {
	Candidate      releaseRecord
	Fallback       releaseRecord
	CandidateOCI   string
	FallbackOCI    string
	CandidateISO   string
	CandidateQCOW2 string
}

func ValidateArtifacts(candidate, fallback ArtifactSet) (ValidatedArtifacts, error) {
	candidateRecord, err := readReleaseRecord(candidate.Record)
	if err != nil {
		return ValidatedArtifacts{}, fmt.Errorf("candidate release record: %w", err)
	}
	fallbackRecord, err := readReleaseRecord(fallback.Record)
	if err != nil {
		return ValidatedArtifacts{}, fmt.Errorf("fallback release record: %w", err)
	}
	if err = validateMatchingNative(candidateRecord, fallbackRecord); err != nil {
		return ValidatedArtifacts{}, err
	}
	paths := []string{candidate.OCI, candidate.ISO, candidate.QCOW2, fallback.OCI}
	for _, path := range paths {
		if err = requireRegularFile(path); err != nil {
			return ValidatedArtifacts{}, err
		}
	}
	if err = requireChecksum(candidate.ISO, candidateRecord.ISOChecksum, "candidate ISO"); err != nil {
		return ValidatedArtifacts{}, err
	}
	if err = requireChecksum(candidate.QCOW2, candidateRecord.QCOW2Checksum, "candidate QCOW2"); err != nil {
		return ValidatedArtifacts{}, err
	}
	return ValidatedArtifacts{
		Candidate:      candidateRecord,
		Fallback:       fallbackRecord,
		CandidateOCI:   candidate.OCI,
		FallbackOCI:    fallback.OCI,
		CandidateISO:   candidate.ISO,
		CandidateQCOW2: candidate.QCOW2,
	}, nil
}

func readReleaseRecord(path string) (releaseRecord, error) {
	if err := requireRegularFile(path); err != nil {
		return releaseRecord{}, err
	}
	record, err := release.ReadStrictRecord(path)
	if err != nil {
		return releaseRecord{}, fmt.Errorf("decode: %w", err)
	}
	if !validReleaseRecord(record) {
		return releaseRecord{}, errors.New("release record identity or provenance is incomplete")
	}
	return record, nil
}

func validReleaseRecord(record releaseRecord) bool {
	if (record.SchemaVersion != 3 && record.SchemaVersion != 4) || !gitRevision(record.SourceRevision) {
		return false
	}
	if record.SodaVersion == "" || record.Channel == "" || !exactReference(record.FedoraBaseReference) {
		return false
	}
	if !strings.HasPrefix(record.SodaImageReference, release.Repository+"@sha256:") {
		return false
	}
	if !exactReference(record.SodaImageReference) || !validReleaseChecksums(record) {
		return false
	}
	return validReleaseRuntimeLock(record)
}

func validReleaseRuntimeLock(record releaseRecord) bool {
	return record.SchemaVersion != 4 || (record.RuntimePackageLock != "" && validHex(record.RuntimeLockSHA256))
}

func validReleaseChecksums(record releaseRecord) bool {
	checksums := []string{record.RPMInventorySHA256, record.ISOChecksum, record.QCOW2Checksum, record.QCOW2ZSTChecksum}
	for _, checksum := range checksums {
		if !validHex(checksum) {
			return false
		}
	}
	return true
}

func validHex(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validateMatchingNative(candidate, fallback releaseRecord) error {
	expected := map[string]string{"amd64": "linux/amd64", "arm64": "linux/arm64"}[runtime.GOARCH]
	if expected == "" {
		return fmt.Errorf("acceptance requires matching-native x86-64 or AArch64, not %s", runtime.GOARCH)
	}
	if candidate.Platform != expected || fallback.Platform != expected {
		return fmt.Errorf("release records must both target native platform %s", expected)
	}
	if candidate.SourceRevision == fallback.SourceRevision {
		return errors.New("candidate and fallback records name the same source revision")
	}
	return nil
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

func requireChecksum(path, expected, label string) error {
	actual, err := fileSHA256(path)
	if err != nil {
		return fmt.Errorf("checksum %s: %w", label, err)
	}
	if actual != expected {
		return fmt.Errorf("%s checksum %s does not match record %s", label, actual, expected)
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
	if len(parts) != 2 || parts[0] == "" || len(parts[1]) != 64 {
		return false
	}
	_, err := hex.DecodeString(parts[1])
	return err == nil
}
