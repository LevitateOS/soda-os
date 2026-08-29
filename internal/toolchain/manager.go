package toolchain

import (
	"context"
	"net/http"
	"runtime"
	"time"

	"github.com/LevitateOS/soda-os/internal/domain"
)

type Installer interface {
	Install(context.Context, domain.ToolchainProfile) (domain.ToolchainInstallation, error)
}

type httpClient interface {
	Do(*http.Request) (*http.Response, error)
}

type Manager struct {
	root         string
	client       httpClient
	architecture string
}

type archiveKind uint8

const (
	tarGz archiveKind = iota
	tarXz
	zipArchive
	executable

	defaultHTTPRequestTimeout = 10 * time.Minute
)

type artifact struct {
	tool, version, url, checksum string
	kind                         archiveKind
}

var normalizedToolBinaries = map[string]bool{"uv": true, "bun": true}

func New(root string) *Manager {
	return &Manager{root: root, client: &http.Client{Timeout: defaultHTTPRequestTimeout}, architecture: runtime.GOARCH}
}
