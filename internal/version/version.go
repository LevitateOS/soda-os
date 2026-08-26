package version

const (
	Name           = "Soda OS"
	ID             = "sodaos"
	DefaultVersion = "0.2.0"
)

// These values may be replaced with reproducible -ldflags at build time.
var (
	Version   = DefaultVersion
	Commit    = "unknown"
	BuildDate = "unknown"
)
