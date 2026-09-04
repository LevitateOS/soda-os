package release

import "github.com/LevitateOS/soda-os/internal/config"

// ReadStrictRecord parses one release record using the publication boundary's
// duplicate-field, unknown-field, and trailing-data checks.
func ReadStrictRecord(path string) (Record, error) {
	return readStrictRecord(path)
}

// ValidateStrictRecord also binds a strict record to the selected Soda
// distribution, architecture, and exact source revision.
func ValidateStrictRecord(path string, spec config.DistroSpec, revision string) (Record, error) {
	return validateUploadRecord(path, spec, revision)
}
