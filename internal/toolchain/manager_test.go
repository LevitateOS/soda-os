package toolchain

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/domain"
	"github.com/stretchr/testify/require"
)

type fixtureClient struct {
	responses map[string][]byte
	calls     map[string]int
}

func (c *fixtureClient) Do(request *http.Request) (*http.Response, error) {
	c.calls[request.URL.String()]++
	body, ok := c.responses[request.URL.String()]
	status := "200 OK"
	code := 200
	if !ok {
		body = []byte("missing")
		status = "404 Not Found"
		code = 404
	}
	return &http.Response{StatusCode: code, Status: status, Body: io.NopCloser(bytes.NewReader(body)), Header: make(http.Header)}, nil
}

func TestResolvesAllPublisherVerifiedProfiles(t *testing.T) {
	rustSum := strings.Repeat("a", 64)
	client := &fixtureClient{calls: map[string]int{}, responses: map[string][]byte{
		"https://nodejs.org/dist/index.json":                                                    []byte(`[{"version":"v22.1.0","lts":"LTS","files":["linux-arm64"]}]`),
		"https://nodejs.org/dist/v22.1.0/SHASUMS256.txt":                                        []byte(strings.Repeat("b", 64) + "  node-v22.1.0-linux-arm64.tar.xz\n"),
		"https://api.github.com/repos/oven-sh/bun/releases/latest":                              []byte(`{"tag_name":"bun-v1","assets":[{"name":"bun-linux-aarch64.zip","browser_download_url":"https://example/bun","digest":"sha256:` + strings.Repeat("c", 64) + `"}]}`),
		"https://api.github.com/repos/astral-sh/uv/releases/latest":                             []byte(`{"tag_name":"uv-v1","assets":[{"name":"uv-aarch64-unknown-linux-gnu.tar.gz","browser_download_url":"https://example/uv","digest":"sha256:` + strings.Repeat("d", 64) + `"}]}`),
		"https://static.rust-lang.org/dist/channel-rust-stable.toml":                            []byte("[pkg.rust]\nversion = \"1.90.0\"\n"),
		"https://static.rust-lang.org/rustup/dist/aarch64-unknown-linux-gnu/rustup-init.sha256": []byte(rustSum + "  rustup-init\n"),
		"https://go.dev/dl/?mode=json":                                                          []byte(`[{"version":"go1.25.1","stable":true,"files":[{"filename":"go1.25.1.linux-arm64.tar.gz","os":"linux","arch":"arm64","sha256":"` + strings.Repeat("e", 64) + `","kind":"archive"}]}]`),
	}}
	manager := &Manager{root: t.TempDir(), client: client}
	for _, profile := range []domain.ToolchainProfile{domain.ToolchainWeb, domain.ToolchainPython, domain.ToolchainRust, domain.ToolchainGo} {
		items, err := manager.resolve(context.Background(), profile)
		if err != nil {
			t.Fatalf("resolve %s: %v", profile, err)
		}
		if len(items) == 0 {
			t.Fatalf("resolve %s returned no artifacts", profile)
		}
		for _, item := range items {
			if len(item.checksum) != 64 {
				t.Fatalf("%s checksum = %q", profile, item.checksum)
			}
		}
	}
}

func TestArtifactCacheAndChecksumVerification(t *testing.T) {
	payload := tarPayload(t, "tool/bin/tool", []byte("binary"))
	sum := sha256.Sum256(payload)
	client := &fixtureClient{calls: map[string]int{}, responses: map[string][]byte{"https://example/tool": payload}}
	manager := &Manager{root: t.TempDir(), client: client}
	item := artifact{tool: "tool", version: "v1", url: "https://example/tool", checksum: hex.EncodeToString(sum[:]), kind: tarGz}
	first, err := manager.installArtifact(context.Background(), item)
	if err != nil {
		t.Fatal(err)
	}
	second, err := manager.installArtifact(context.Background(), item)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || client.calls[item.url] != 1 {
		t.Fatalf("paths %q %q, calls %d", first, second, client.calls[item.url])
	}
	bad := item
	bad.version = "v2"
	bad.checksum = "wrong"
	if _, err = manager.installArtifact(context.Background(), bad); err == nil {
		t.Fatal("checksum mismatch accepted")
	}
}

func TestCompletedToolchainAncestorsAreTraversable(t *testing.T) {
	root := t.TempDir()
	tool := filepath.Join(root, "tool")
	version := filepath.Join(tool, "v1")
	if err := os.MkdirAll(filepath.Join(version, "bin"), 0o770); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0o770); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(tool, 0o770); err != nil {
		t.Fatal(err)
	}
	if err := makeReadable(root, version); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{root, tool, version, filepath.Join(version, "bin")} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o755 {
			t.Fatalf("%s mode = %o", path, info.Mode().Perm())
		}
	}
}

func TestWriteInstallationEmitsStructuredRustEnvironment(t *testing.T) {
	root := t.TempDir()
	manager := New(root)
	paths := []string{filepath.Join(root, "rust", "1.90.0", "cargo", "bin")}
	installation, err := manager.writeInstallation(domain.ToolchainRust, []artifact{{tool: "rust", version: "1.90.0", checksum: strings.Repeat("a", 64)}}, "rust=1.90.0", paths)
	require.NoError(t, err)
	encoded, err := os.ReadFile(filepath.Join(installation.Path, "environment.json"))
	require.NoError(t, err)
	var environment domain.ProjectEnvironment
	require.NoError(t, json.Unmarshal(encoded, &environment))
	wantVariables := map[string]string{
		"RUSTUP_HOME": filepath.Join(root, "rust", "1.90.0", "rustup"),
		"CARGO_HOME":  filepath.Join(root, "rust", "1.90.0", "cargo"),
	}
	require.Equal(t, domain.ProjectEnvironment{Profile: string(domain.ToolchainRust), Path: paths, Variables: wantVariables}, environment)
	require.NoFileExists(t, filepath.Join(installation.Path, "env"))
}

func TestDefaultHTTPClientHasRequestTimeout(t *testing.T) {
	manager := New(t.TempDir())
	client, ok := manager.client.(*http.Client)
	if !ok {
		t.Fatalf("default client = %T", manager.client)
	}
	if client.Timeout != defaultHTTPRequestTimeout || client.Timeout <= 0 {
		t.Fatalf("HTTP timeout = %s", client.Timeout)
	}
}

func tarPayload(t *testing.T, name string, contents []byte) []byte {
	t.Helper()
	var output bytes.Buffer
	gzipWriter := gzip.NewWriter(&output)
	archive := tar.NewWriter(gzipWriter)
	if err := archive.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(contents))}); err != nil {
		t.Fatal(err)
	}
	if _, err := archive.Write(contents); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}
