package release

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LevitateOS/soda-os/internal/config"
)

const githubReleaseAssetLimit = 2 << 30

type localAsset struct {
	Path   string
	Name   string
	Size   int64
	Digest string
}

func validateUploadArtifacts(spec config.DistroSpec, revision string, options UploadOptions) ([]localAsset, error) {
	expected := expectedUploadNames(spec)
	if err := expected.validate(options); err != nil {
		return nil, err
	}
	paths := uploadPaths(options)
	if err := validateArtifactPaths(paths); err != nil {
		return nil, err
	}
	record, err := validateUploadRecord(options.RecordPath, spec, revision)
	if err != nil {
		return nil, err
	}
	if err := validateUploadChecksums(record, options, expected); err != nil {
		return nil, err
	}
	assets, err := inspectLocalAssets(paths)
	if err != nil {
		return nil, err
	}
	if err := validateGitHubAssetSizes(assets); err != nil {
		return nil, err
	}
	return assets, nil
}

type uploadNames struct {
	iso          string
	qcow2ZST     string
	record       string
	recordBundle string
}

func expectedUploadNames(spec config.DistroSpec) uploadNames {
	record := "soda-os-" + spec.Identity.Version + "-" + spec.Platform.Release.Channel + ".release.json"
	return uploadNames{
		iso:          "SodaOS-" + spec.Identity.Version + "-" + spec.Platform.Architecture.Artifact + ".iso",
		qcow2ZST:     "SodaOS-" + spec.Identity.Version + "-" + spec.Platform.Architecture.Artifact + ".qcow2.zst",
		record:       record,
		recordBundle: record + ".sigstore.json",
	}
}

func (names uploadNames) validate(options UploadOptions) error {
	if filepath.Base(options.ISOPath) != names.iso || filepath.Base(options.QCOW2ZSTPath) != names.qcow2ZST || filepath.Base(options.RecordPath) != names.record || filepath.Base(options.RecordBundlePath) != names.recordBundle {
		return errors.New("release artifact filenames differ from the selected Soda architecture")
	}
	return nil
}

func uploadPaths(options UploadOptions) []string {
	return []string{options.ISOPath, options.ISOPath + ".sha256", options.QCOW2ZSTPath, options.QCOW2ZSTPath + ".sha256", options.RecordPath, options.RecordBundlePath}
}

func validateUploadRecord(path string, spec config.DistroSpec, revision string) (Record, error) {
	record, err := readStrictRecord(path)
	if err != nil {
		return Record{}, err
	}
	if err := validatePublicationRecord(record, spec, revision); err != nil {
		return Record{}, err
	}
	return record, nil
}

func validateUploadChecksums(record Record, options UploadOptions, names uploadNames) error {
	if err := validateReleaseAssetChecksum("installer ISO", options.ISOPath, record.ISOChecksum, names.iso); err != nil {
		return err
	}
	return validateReleaseAssetChecksum("compressed QCOW2", options.QCOW2ZSTPath, record.QCOW2ZSTChecksum, names.qcow2ZST)
}

func validateReleaseAssetChecksum(label, path, expectedDigest, expectedName string) error {
	digest, err := fileSHA256(path)
	if err != nil {
		return fmt.Errorf("checksum %s: %w", label, err)
	}
	if digest != expectedDigest {
		return fmt.Errorf("%s checksum differs from its release record", label)
	}
	return validateSidecar(path+".sha256", digest, expectedName)
}

func validateGitHubAssetSizes(assets []localAsset) error {
	for _, asset := range assets {
		if asset.Size >= githubReleaseAssetLimit {
			return fmt.Errorf("GitHub release asset %q exceeds the 2 GiB per-file limit", asset.Name)
		}
	}
	return nil
}

func validateArtifactPaths(paths []string) error {
	if err := requireDistinctPaths(paths); err != nil {
		return err
	}
	for _, path := range paths {
		if err := requireRegularFile("release artifact", path); err != nil {
			return err
		}
	}
	return nil
}

func inspectLocalAssets(paths []string) ([]localAsset, error) {
	assets := make([]localAsset, 0, len(paths))
	for _, path := range paths {
		asset, err := inspectLocalAsset(path)
		if err != nil {
			return nil, err
		}
		assets = append(assets, asset)
	}
	return assets, nil
}

func validatePublicationRecord(record Record, spec config.DistroSpec, revision string) error {
	if !validRecordIdentity(record, spec, revision) {
		return errors.New("release record identity differs from the clean Soda source revision")
	}
	if !validRecordPlatform(record, spec) {
		return errors.New("release record platform differs from the selected Soda architecture")
	}
	if !validRecordProvenance(record) {
		return errors.New("release record contains invalid image or checksum provenance")
	}
	return nil
}

func validRecordIdentity(record Record, spec config.DistroSpec, revision string) bool {
	return record.SchemaVersion == 3 && record.SodaVersion == spec.Identity.Version && record.SourceRevision == revision
}

func validRecordPlatform(record Record, spec config.DistroSpec) bool {
	return record.Platform == spec.Base.Platform && record.Channel == spec.Platform.Release.Channel && record.FedoraBaseReference == spec.Base.Reference
}

func validRecordProvenance(record Record) bool {
	return isSodaDigestReference(record.SodaImageReference) && validHexadecimal(record.RPMInventorySHA256, 64) && validHexadecimal(record.ISOChecksum, 64) && validHexadecimal(record.QCOW2Checksum, 64) && validHexadecimal(record.QCOW2ZSTChecksum, 64)
}

func readStrictRecord(path string) (Record, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return Record{}, fmt.Errorf("read release record: %w", err)
	}
	if err := rejectDuplicateJSONFields(contents); err != nil {
		return Record{}, fmt.Errorf("decode release record: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	var record Record
	if err := decoder.Decode(&record); err != nil {
		return Record{}, fmt.Errorf("decode release record: %w", err)
	}
	if err := requireJSONEnd(decoder); err != nil {
		return Record{}, fmt.Errorf("decode release record: %w", err)
	}
	return record, nil
}

func rejectDuplicateJSONFields(contents []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(contents))
	if err := scanJSONValue(decoder); err != nil {
		return err
	}
	return requireJSONEnd(decoder)
}

func scanJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		return scanJSONObject(decoder)
	case '[':
		return scanJSONArray(decoder)
	default:
		return errors.New("unexpected JSON delimiter")
	}
}

func scanJSONObject(decoder *json.Decoder) error {
	seen := map[string]struct{}{}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		name, ok := token.(string)
		if !ok {
			return errors.New("JSON object field is not a string")
		}
		if _, duplicate := seen[name]; duplicate {
			return fmt.Errorf("duplicate JSON field %q", name)
		}
		seen[name] = struct{}{}
		if err := scanJSONValue(decoder); err != nil {
			return err
		}
	}
	_, err := decoder.Token()
	return err
}

func scanJSONArray(decoder *json.Decoder) error {
	for decoder.More() {
		if err := scanJSONValue(decoder); err != nil {
			return err
		}
	}
	_, err := decoder.Token()
	return err
}

func requireJSONEnd(decoder *json.Decoder) error {
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("release record contains multiple JSON values")
		}
		return err
	}
	return nil
}

func validateSidecar(path, digest, isoName string) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read installer checksum sidecar: %w", err)
	}
	expected := digest + "  " + isoName + "\n"
	if string(contents) != expected {
		return errors.New("installer checksum sidecar differs from the ISO bytes or deterministic filename")
	}
	return nil
}

func inspectLocalAsset(path string) (localAsset, error) {
	info, err := os.Stat(path)
	if err != nil {
		return localAsset{}, err
	}
	digest, err := fileSHA256(path)
	if err != nil {
		return localAsset{}, err
	}
	return localAsset{Path: path, Name: filepath.Base(path), Size: info.Size(), Digest: "sha256:" + digest}, nil
}

func requireRegularFile(label, path string) error {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("%s %q must be a regular non-symlink file", label, path)
	}
	return nil
}

func requireDistinctPaths(paths []string) error {
	seen := map[string]struct{}{}
	for _, path := range paths {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		if _, duplicate := seen[absolute]; duplicate {
			return errors.New("release artifact paths must be distinct")
		}
		seen[absolute] = struct{}{}
	}
	return nil
}

func requireUploadNamesAbsent(remote []remoteAsset, local []localAsset) error {
	existing := map[string]struct{}{}
	for _, asset := range remote {
		if _, duplicate := existing[asset.Name]; duplicate {
			return fmt.Errorf("GitHub release has duplicate asset %q", asset.Name)
		}
		existing[asset.Name] = struct{}{}
	}
	for _, asset := range local {
		if _, collision := existing[asset.Name]; collision {
			return fmt.Errorf("GitHub release asset %q already exists", asset.Name)
		}
	}
	return nil
}

func verifyUploadedAssets(remote []remoteAsset, local []localAsset) error {
	byName, err := remoteAssetsByName(remote)
	if err != nil {
		return err
	}
	for _, expected := range local {
		actual, ok := byName[expected.Name]
		if !ok {
			return fmt.Errorf("uploaded GitHub release asset %q is missing", expected.Name)
		}
		if actual.State != "uploaded" || actual.Size != expected.Size || actual.Digest != expected.Digest {
			return fmt.Errorf("uploaded GitHub release asset %q differs from the local artifact", expected.Name)
		}
	}
	return nil
}

func requirePublishableAssets(assets []remoteAsset, version string) error {
	byName, err := remoteAssetsByName(assets)
	if err != nil {
		return err
	}
	for _, asset := range assets {
		if asset.State != "uploaded" || asset.Size <= 0 || !validDigest(asset.Digest) {
			return fmt.Errorf("GitHub release asset %q is not fully uploaded with a SHA-256 digest", asset.Name)
		}
	}
	for _, name := range requiredAssetNames(version) {
		if _, ok := byName[name]; !ok {
			return fmt.Errorf("GitHub release is missing required asset %q", name)
		}
	}
	return nil
}

func remoteAssetsByName(assets []remoteAsset) (map[string]remoteAsset, error) {
	byName := make(map[string]remoteAsset, len(assets))
	for _, asset := range assets {
		if asset.Name == "" {
			return nil, errors.New("GitHub release contains an unnamed asset")
		}
		if _, duplicate := byName[asset.Name]; duplicate {
			return nil, fmt.Errorf("GitHub release has duplicate asset %q", asset.Name)
		}
		byName[asset.Name] = asset
	}
	return byName, nil
}

func requiredAssetNames(version string) []string {
	names := make([]string, 0, 12)
	for _, architecture := range []string{"aarch64", "x86_64"} {
		iso := "SodaOS-" + version + "-" + architecture + ".iso"
		qcow2 := "SodaOS-" + version + "-" + architecture + ".qcow2.zst"
		record := "soda-os-" + version + "-" + architecture + ".release.json"
		names = append(names, iso, iso+".sha256", qcow2, qcow2+".sha256", record, record+".sigstore.json")
	}
	return names
}

func assetIdentities(assets []remoteAsset) []string {
	identities := make([]string, 0, len(assets))
	for _, asset := range assets {
		identities = append(identities, fmt.Sprintf("%s:%d:%s:%s", asset.Name, asset.Size, asset.State, asset.Digest))
	}
	sort.Strings(identities)
	return identities
}

func artifactNames(assets []localAsset) []string {
	names := make([]string, 0, len(assets))
	for _, asset := range assets {
		names = append(names, asset.Name)
	}
	return names
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func isSodaDigestReference(reference string) bool {
	prefix := Repository + "@sha256:"
	return strings.HasPrefix(reference, prefix) && validHexadecimal(strings.TrimPrefix(reference, prefix), 64)
}

func validDigest(digest string) bool {
	return strings.HasPrefix(digest, "sha256:") && validHexadecimal(strings.TrimPrefix(digest, "sha256:"), 64)
}

func validHexadecimal(value string, length int) bool {
	if len(value) != length || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
