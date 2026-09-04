package release

// ReadStrictRecord parses one release record using the publication boundary's
// duplicate-field, unknown-field, and trailing-data checks.
func ReadStrictRecord(path string) (Record, error) {
	return readStrictRecord(path)
}
