package toolchain

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/LevitateOS/soda-os/internal/domain"
	"golang.org/x/sys/unix"
)

type Installer interface {
	Install(context.Context, domain.ToolchainProfile) (domain.ToolchainInstallation, error)
}

type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

type Manager struct {
	Root   string
	Client HTTPClient
}

type archiveKind uint8

const (
	tarGz archiveKind = iota
	tarXz
	zipArchive
	executable
)

type artifact struct {
	tool, version, url, checksum string
	kind                         archiveKind
}

const defaultHTTPRequestTimeout = 10 * time.Minute

func New(root string) *Manager {
	return &Manager{Root: root, Client: &http.Client{Timeout: defaultHTTPRequestTimeout}}
}

func (m *Manager) Install(ctx context.Context, profile domain.ToolchainProfile) (domain.ToolchainInstallation, error) {
	if !validProfile(profile) {
		return domain.ToolchainInstallation{}, fmt.Errorf("unknown toolchain profile %q", profile)
	}
	if err := os.MkdirAll(m.Root, 0o755); err != nil {
		return domain.ToolchainInstallation{}, err
	}
	if err := os.Chmod(m.Root, 0o755); err != nil {
		return domain.ToolchainInstallation{}, err
	}
	lock, err := os.OpenFile(filepath.Join(m.Root, "."+string(profile)+".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return domain.ToolchainInstallation{}, err
	}
	defer lock.Close()
	if err = unix.Flock(int(lock.Fd()), unix.LOCK_EX); err != nil {
		return domain.ToolchainInstallation{}, err
	}
	defer unix.Flock(int(lock.Fd()), unix.LOCK_UN)
	artifacts, err := m.resolve(ctx, profile)
	if err != nil {
		return domain.ToolchainInstallation{}, err
	}
	versions := make([]string, 0, len(artifacts))
	paths := make([]string, 0, len(artifacts))
	for _, item := range artifacts {
		versions = append(versions, item.tool+"="+item.version)
		bin, installErr := m.installArtifact(ctx, item)
		if installErr != nil {
			return domain.ToolchainInstallation{}, installErr
		}
		paths = append(paths, bin)
	}
	version := strings.Join(versions, ",")
	if profile == domain.ToolchainPython {
		pythonPath, pythonVersion, installErr := m.installPython(ctx, artifacts)
		if installErr != nil {
			return domain.ToolchainInstallation{}, installErr
		}
		paths = append(paths, pythonPath)
		version += ",python=" + pythonVersion
	}
	digest := sha256.Sum256([]byte(version))
	profileRoot := filepath.Join(m.Root, "profiles", string(profile), hex.EncodeToString(digest[:]))
	if err = os.MkdirAll(profileRoot, 0o755); err != nil {
		return domain.ToolchainInstallation{}, err
	}
	var env strings.Builder
	fmt.Fprintf(&env, "export SODA_PROFILE=%s\n", profile)
	if profile == domain.ToolchainRust {
		rust := findArtifact(artifacts, "rust")
		if rust == nil {
			return domain.ToolchainInstallation{}, errors.New("Rust profile is missing rustup")
		}
		fmt.Fprintf(&env, "export RUSTUP_HOME=%s\n", filepath.Join(m.Root, "rust", rust.version, "rustup"))
		fmt.Fprintf(&env, "export CARGO_HOME=%s\n", filepath.Join(m.Root, "rust", rust.version, "cargo"))
	}
	fmt.Fprintf(&env, "export PATH=%s:$PATH\n", strings.Join(paths, ":"))
	if err = os.WriteFile(filepath.Join(profileRoot, "env"), []byte(env.String()), 0o644); err != nil {
		return domain.ToolchainInstallation{}, err
	}
	if err = makeReadable(m.Root, profileRoot); err != nil {
		return domain.ToolchainInstallation{}, err
	}
	return domain.ToolchainInstallation{Profile: profile, Version: version, Path: profileRoot, Checksum: aggregateChecksum(artifacts), State: domain.JobReady}, nil
}

func (m *Manager) resolve(ctx context.Context, profile domain.ToolchainProfile) ([]artifact, error) {
	switch profile {
	case domain.ToolchainWeb:
		node, err := m.resolveNode(ctx)
		if err != nil {
			return nil, err
		}
		bun, err := m.resolveGitHub(ctx, "oven-sh/bun", "bun-linux-aarch64.zip", "bun", zipArchive)
		return []artifact{node, bun}, err
	case domain.ToolchainPython:
		uv, err := m.resolveGitHub(ctx, "astral-sh/uv", "uv-aarch64-unknown-linux-gnu.tar.gz", "uv", tarGz)
		return []artifact{uv}, err
	case domain.ToolchainRust:
		rust, err := m.resolveRust(ctx)
		return []artifact{rust}, err
	case domain.ToolchainGo:
		goTool, err := m.resolveGo(ctx)
		return []artifact{goTool}, err
	default:
		return nil, fmt.Errorf("unknown toolchain profile %q", profile)
	}
}

func (m *Manager) resolveNode(ctx context.Context) (artifact, error) {
	var releases []struct {
		Version string   `json:"version"`
		LTS     any      `json:"lts"`
		Files   []string `json:"files"`
	}
	if err := m.getJSON(ctx, "https://nodejs.org/dist/index.json", &releases); err != nil {
		return artifact{}, err
	}
	for _, release := range releases {
		if _, ok := release.LTS.(string); !ok {
			continue
		}
		if !contains(release.Files, "linux-arm64") {
			continue
		}
		filename := "node-" + release.Version + "-linux-arm64.tar.xz"
		base := "https://nodejs.org/dist/" + release.Version + "/"
		sums, err := m.getText(ctx, base+"SHASUMS256.txt")
		if err != nil {
			return artifact{}, err
		}
		sum, err := checksumLine(sums, filename)
		return artifact{"node", release.Version, base + filename, sum, tarXz}, err
	}
	return artifact{}, errors.New("Node active LTS AArch64 release not found")
}
func (m *Manager) resolveGo(ctx context.Context) (artifact, error) {
	var releases []struct {
		Version string `json:"version"`
		Stable  bool   `json:"stable"`
		Files   []struct {
			Filename string `json:"filename"`
			OS       string `json:"os"`
			Arch     string `json:"arch"`
			SHA256   string `json:"sha256"`
			Kind     string `json:"kind"`
		} `json:"files"`
	}
	if err := m.getJSON(ctx, "https://go.dev/dl/?mode=json", &releases); err != nil {
		return artifact{}, err
	}
	for _, release := range releases {
		if !release.Stable {
			continue
		}
		for _, file := range release.Files {
			if file.OS == "linux" && file.Arch == "arm64" && file.Kind == "archive" {
				return artifact{"go", release.Version, "https://go.dev/dl/" + file.Filename, file.SHA256, tarGz}, nil
			}
		}
	}
	return artifact{}, errors.New("Go Linux AArch64 archive not found")
}
func (m *Manager) resolveRust(ctx context.Context) (artifact, error) {
	var channel struct {
		Pkg struct {
			Rust struct {
				Version string `toml:"version"`
			} `toml:"rust"`
		} `toml:"pkg"`
	}
	body, err := m.getText(ctx, "https://static.rust-lang.org/dist/channel-rust-stable.toml")
	if err != nil {
		return artifact{}, err
	}
	if _, err = toml.Decode(body, &channel); err != nil {
		return artifact{}, fmt.Errorf("invalid Rust channel manifest: %w", err)
	}
	url := "https://static.rust-lang.org/rustup/dist/aarch64-unknown-linux-gnu/rustup-init"
	sum, err := m.getText(ctx, url+".sha256")
	if err != nil {
		return artifact{}, err
	}
	fields := strings.Fields(sum)
	if len(fields) == 0 {
		return artifact{}, errors.New("Rust checksum is empty")
	}
	return artifact{"rust", channel.Pkg.Rust.Version, url, fields[0], executable}, nil
}
func (m *Manager) resolveGitHub(ctx context.Context, repository, assetName, tool string, kind archiveKind) (artifact, error) {
	var release struct {
		Tag    string `json:"tag_name"`
		Assets []struct {
			Name       string  `json:"name"`
			Digest     *string `json:"digest"`
			BrowserURL string  `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := m.getJSON(ctx, "https://api.github.com/repos/"+repository+"/releases/latest", &release); err != nil {
		return artifact{}, err
	}
	for _, asset := range release.Assets {
		if asset.Name != assetName {
			continue
		}
		if asset.Digest == nil {
			return artifact{}, fmt.Errorf("%s asset %s has no publisher digest", repository, assetName)
		}
		sum, ok := strings.CutPrefix(*asset.Digest, "sha256:")
		if !ok {
			return artifact{}, fmt.Errorf("unsupported asset digest %s", *asset.Digest)
		}
		return artifact{tool, release.Tag, asset.BrowserURL, sum, kind}, nil
	}
	return artifact{}, fmt.Errorf("%s asset %s not found", repository, assetName)
}

func (m *Manager) installArtifact(ctx context.Context, item artifact) (string, error) {
	destination := filepath.Join(m.Root, item.tool, item.version)
	ready := filepath.Join(destination, ".ready")
	if _, err := os.Stat(ready); err == nil {
		if err = makeReadable(m.Root, destination); err != nil {
			return "", err
		}
		return artifactBin(item, destination), nil
	}
	if err := os.RemoveAll(destination); err != nil {
		return "", err
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return "", err
	}
	payload, err := m.getBytes(ctx, item.url)
	if err != nil {
		return "", err
	}
	if err = VerifyChecksum(payload, item.checksum); err != nil {
		return "", err
	}
	switch item.kind {
	case tarGz:
		err = extractTarGz(payload, destination)
	case tarXz:
		err = extractTarXz(ctx, payload, destination)
	case zipArchive:
		err = extractZip(payload, destination)
	case executable:
		err = m.installRustup(ctx, payload, destination)
	}
	if err != nil {
		return "", err
	}
	if item.tool == "uv" || item.tool == "bun" {
		if err = normalizeBinary(destination, item.tool); err != nil {
			return "", err
		}
	}
	if err = os.WriteFile(ready, []byte("ready\n"), 0o644); err != nil {
		return "", err
	}
	if err = makeReadable(m.Root, destination); err != nil {
		return "", err
	}
	return artifactBin(item, destination), nil
}
func (m *Manager) installRustup(ctx context.Context, payload []byte, destination string) error {
	temporary, err := os.CreateTemp(m.Root, ".rustup-init-")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if _, err = temporary.Write(payload); err != nil {
		return err
	}
	if err = temporary.Close(); err != nil {
		return err
	}
	if err = os.Chmod(name, 0o755); err != nil {
		return err
	}
	command := exec.CommandContext(ctx, name, "-y", "--no-modify-path", "--profile", "default", "--default-toolchain", "stable")
	command.Env = append(os.Environ(), "RUSTUP_HOME="+filepath.Join(destination, "rustup"), "CARGO_HOME="+filepath.Join(destination, "cargo"))
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rustup-init: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}
func (m *Manager) installPython(ctx context.Context, artifacts []artifact) (string, string, error) {
	uv := findArtifact(artifacts, "uv")
	if uv == nil {
		return "", "", errors.New("Python profile is missing uv")
	}
	binary := filepath.Join(m.Root, "uv", uv.version, "bin", "uv")
	root := filepath.Join(m.Root, "python")
	command := exec.CommandContext(ctx, binary, "python", "install")
	command.Env = append(os.Environ(), "UV_PYTHON_INSTALL_DIR="+root)
	if output, err := command.CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("uv python install: %w: %s", err, strings.TrimSpace(string(output)))
	}
	bin, err := firstPythonBin(root)
	if err != nil {
		return "", "", err
	}
	versionCommand := exec.CommandContext(ctx, filepath.Join(bin, "python3"), "--version")
	output, err := versionCommand.CombinedOutput()
	if err != nil {
		return "", "", err
	}
	if err = makeReadable(m.Root, root); err != nil {
		return "", "", err
	}
	return bin, strings.TrimSpace(string(output)), nil
}

func (m *Manager) request(ctx context.Context, url string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "SodaOS/0.2 toolchain resolver")
	response, err := m.Client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("toolchain metadata request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("toolchain metadata request failed: %s", response.Status)
	}
	return io.ReadAll(response.Body)
}
func (m *Manager) getText(ctx context.Context, url string) (string, error) {
	body, err := m.request(ctx, url)
	return string(body), err
}
func (m *Manager) getBytes(ctx context.Context, url string) ([]byte, error) {
	return m.request(ctx, url)
}
func (m *Manager) getJSON(ctx context.Context, url string, target any) error {
	body, err := m.request(ctx, url)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, target)
}

func VerifyChecksum(payload []byte, expected string) error {
	actual := sha256.Sum256(payload)
	got := hex.EncodeToString(actual[:])
	if !strings.EqualFold(got, expected) {
		return fmt.Errorf("toolchain checksum mismatch: expected %s, got %s", expected, got)
	}
	return nil
}
func checksumLine(contents, filename string) (string, error) {
	for _, line := range strings.Split(contents, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.TrimPrefix(fields[1], "*") == filename {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("checksum for %s not found", filename)
}
func aggregateChecksum(items []artifact) string {
	hash := sha256.New()
	for _, item := range items {
		io.WriteString(hash, item.checksum)
	}
	return hex.EncodeToString(hash.Sum(nil))
}
func validProfile(profile domain.ToolchainProfile) bool {
	return profile == domain.ToolchainWeb || profile == domain.ToolchainPython || profile == domain.ToolchainRust || profile == domain.ToolchainGo
}
func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
func findArtifact(items []artifact, tool string) *artifact {
	for index := range items {
		if items[index].tool == tool {
			return &items[index]
		}
	}
	return nil
}
func artifactBin(item artifact, destination string) string {
	if item.kind == executable {
		return filepath.Join(destination, "cargo", "bin")
	}
	return filepath.Join(destination, "bin")
}

func extractTarGz(payload []byte, destination string) error {
	reader, err := gzip.NewReader(bytes.NewReader(payload))
	if err != nil {
		return err
	}
	defer reader.Close()
	return extractTar(reader, destination)
}
func extractTar(reader io.Reader, destination string) error {
	archive := tar.NewReader(reader)
	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		parts := strings.Split(filepath.Clean(header.Name), string(os.PathSeparator))
		if len(parts) < 2 {
			continue
		}
		relative := filepath.Join(parts[1:]...)
		target := filepath.Join(destination, relative)
		if !within(destination, target) {
			return errors.New("archive path escapes destination")
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err = os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err = os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			file, openErr := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode))
			if openErr != nil {
				return openErr
			}
			_, copyErr := io.Copy(file, archive)
			closeErr := file.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		case tar.TypeSymlink:
			if err = os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err = os.Symlink(header.Linkname, target); err != nil {
				return err
			}
		}
	}
}
func extractTarXz(ctx context.Context, payload []byte, destination string) error {
	temporary, err := os.CreateTemp("", "soda-toolchain-*.tar.xz")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if _, err = temporary.Write(payload); err != nil {
		return err
	}
	if err = temporary.Close(); err != nil {
		return err
	}
	command := exec.CommandContext(ctx, "tar", "-xJf", name, "-C", destination, "--strip-components=1")
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("extract xz archive: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}
func extractZip(payload []byte, destination string) error {
	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		return err
	}
	for _, entry := range reader.File {
		target := filepath.Join(destination, entry.Name)
		if !within(destination, target) {
			return errors.New("archive path escapes destination")
		}
		if entry.FileInfo().IsDir() {
			if err = os.MkdirAll(target, entry.Mode()); err != nil {
				return err
			}
			continue
		}
		if err = os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		source, err := entry.Open()
		if err != nil {
			return err
		}
		file, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, entry.Mode())
		if err != nil {
			source.Close()
			return err
		}
		_, copyErr := io.Copy(file, source)
		closeErr := file.Close()
		sourceErr := source.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if sourceErr != nil {
			return sourceErr
		}
	}
	return nil
}
func within(root, target string) bool {
	relative, err := filepath.Rel(root, target)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator))
}
func normalizeBinary(root, name string) error {
	source, err := findFile(root, name)
	if err != nil {
		return err
	}
	bin := filepath.Join(root, "bin")
	if err = os.MkdirAll(bin, 0o755); err != nil {
		return err
	}
	target := filepath.Join(bin, name)
	if source != target {
		contents, readErr := os.ReadFile(source)
		if readErr != nil {
			return readErr
		}
		if err = os.WriteFile(target, contents, 0o755); err != nil {
			return err
		}
	}
	return os.Chmod(target, 0o755)
}
func findFile(root, name string) (string, error) {
	var candidates []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && entry.Name() == name {
			candidates = append(candidates, path)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("downloaded archive does not contain %s", name)
	}
	sort.Strings(candidates)
	return candidates[0], nil
}
func firstPythonBin(root string) (string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		bin := filepath.Join(root, entry.Name(), "bin")
		if exists(filepath.Join(bin, "python")) || exists(filepath.Join(bin, "python3")) {
			return bin, nil
		}
	}
	return "", errors.New("uv did not install a Python interpreter")
}
func exists(path string) bool { _, err := os.Stat(path); return err == nil }
func makeReadable(root, path string) error {
	if err := filepath.Walk(path, func(current string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		mode := os.FileMode(0o644)
		if info.IsDir() || info.Mode()&0o111 != 0 {
			mode = 0o755
		}
		return os.Chmod(current, mode)
	}); err != nil {
		return err
	}
	for current := filepath.Dir(path); within(root, current); current = filepath.Dir(current) {
		if err := os.Chmod(current, 0o755); err != nil {
			return err
		}
		if filepath.Clean(current) == filepath.Clean(root) {
			break
		}
	}
	return nil
}
