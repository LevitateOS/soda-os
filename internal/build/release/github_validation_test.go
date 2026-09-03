package release

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUploadedAssetsRequireExactRemoteMetadata(t *testing.T) {
	local := []localAsset{{Name: "artifact", Size: 5, Digest: "sha256:" + strings.Repeat("a", 64)}}
	matching := remoteAsset{Name: "artifact", Size: 5, State: "uploaded", Digest: local[0].Digest}
	for name, remote := range map[string][]remoteAsset{
		"missing": nil,
		"size":    {{Name: matching.Name, Size: 6, State: matching.State, Digest: matching.Digest}},
		"state":   {{Name: matching.Name, Size: matching.Size, State: "new", Digest: matching.Digest}},
		"digest":  {{Name: matching.Name, Size: matching.Size, State: matching.State, Digest: "sha256:" + strings.Repeat("b", 64)}},
	} {
		t.Run(name, func(t *testing.T) {
			require.Error(t, verifyUploadedAssets(remote, local))
		})
	}
	require.NoError(t, verifyUploadedAssets([]remoteAsset{matching}, local))
}

func TestStrictReleaseRecordRejectsUnknownAndDuplicateFields(t *testing.T) {
	for name, contents := range map[string]string{
		"unknown":   `{"schema_version":2,"unknown":true}`,
		"duplicate": `{"schema_version":2,"schema_version":2}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "record.json")
			require.NoError(t, os.WriteFile(path, []byte(contents), 0o644))
			_, err := readStrictRecord(path)
			require.Error(t, err)
		})
	}
}

func TestUploadArtifactValidationRejectsUnsafeFilesAndSidecars(t *testing.T) {
	t.Run("symlink", func(t *testing.T) {
		options, _ := writeUploadArtifacts(t, testArmPublicationSpec(), testRevision)
		realISO := options.ISOPath + ".real"
		require.NoError(t, os.Rename(options.ISOPath, realISO))
		require.NoError(t, os.Symlink(realISO, options.ISOPath))
		_, err := validateUploadArtifacts(testArmPublicationSpec(), testRevision, options)
		require.ErrorContains(t, err, "regular non-symlink file")
	})

	t.Run("sidecar syntax", func(t *testing.T) {
		options, _ := writeUploadArtifacts(t, testArmPublicationSpec(), testRevision)
		require.NoError(t, os.WriteFile(options.ISOPath+".sha256", []byte(strings.Repeat("a", 64)+"  wrong.iso\n"), 0o644))
		_, err := validateUploadArtifacts(testArmPublicationSpec(), testRevision, options)
		require.ErrorContains(t, err, "checksum sidecar differs")
	})

	t.Run("source revision", func(t *testing.T) {
		options, _ := writeUploadArtifacts(t, testArmPublicationSpec(), testRevision)
		_, err := validateUploadArtifacts(testArmPublicationSpec(), strings.Repeat("c", 40), options)
		require.ErrorContains(t, err, "clean Soda source revision")
	})
}

func TestPublicationRecordRequiresExactIdentityAndProvenance(t *testing.T) {
	spec := testArmPublicationSpec()
	valid := Record{
		SchemaVersion:       3,
		SodaVersion:         spec.Identity.Version,
		SourceRevision:      testRevision,
		Platform:            spec.Base.Platform,
		Channel:             spec.Platform.Release.Channel,
		FedoraBaseReference: spec.Base.Reference,
		SodaImageReference:  Repository + "@sha256:" + strings.Repeat("a", 64),
		ArtifactChecksums: ArtifactChecksums{
			RPMInventorySHA256: strings.Repeat("b", 64),
			ISOChecksum:        strings.Repeat("c", 64),
			QCOW2Checksum:      strings.Repeat("d", 64),
			QCOW2ZSTChecksum:   strings.Repeat("e", 64),
		},
	}
	require.NoError(t, validatePublicationRecord(valid, spec, testRevision))

	mutations := map[string]func(*Record){
		"schema":   func(record *Record) { record.SchemaVersion = 1 },
		"version":  func(record *Record) { record.SodaVersion = "different" },
		"source":   func(record *Record) { record.SourceRevision = strings.Repeat("d", 40) },
		"platform": func(record *Record) { record.Platform = "linux/amd64" },
		"channel":  func(record *Record) { record.Channel = "x86_64" },
		"base":     func(record *Record) { record.FedoraBaseReference = "different" },
		"image reference": func(record *Record) {
			record.SodaImageReference = "example.invalid/image@sha256:" + strings.Repeat("a", 64)
		},
		"RPM checksum":              func(record *Record) { record.RPMInventorySHA256 = "invalid" },
		"ISO checksum":              func(record *Record) { record.ISOChecksum = "invalid" },
		"raw QCOW2 checksum":        func(record *Record) { record.QCOW2Checksum = "invalid" },
		"compressed QCOW2 checksum": func(record *Record) { record.QCOW2ZSTChecksum = "invalid" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			record := valid
			mutate(&record)
			require.Error(t, validatePublicationRecord(record, spec, testRevision))
		})
	}
}
