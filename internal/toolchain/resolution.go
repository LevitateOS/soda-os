package toolchain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/LevitateOS/soda-os/internal/domain"
)

func (m *Manager) resolve(ctx context.Context, profile domain.ToolchainProfile) ([]artifact, error) {
	target, err := toolchainTarget(m.architecture)
	if err != nil {
		return nil, err
	}
	switch profile {
	case domain.ToolchainWeb:
		node, err := m.resolveNode(ctx, target)
		if err != nil {
			return nil, err
		}
		bun, err := m.resolveGitHub(ctx, "oven-sh/bun", target.bunAsset, "bun", zipArchive)
		return []artifact{node, bun}, err
	case domain.ToolchainPython:
		uv, err := m.resolveGitHub(ctx, "astral-sh/uv", target.uvAsset, "uv", tarGz)
		return []artifact{uv}, err
	case domain.ToolchainRust:
		rust, err := m.resolveRust(ctx, target)
		return []artifact{rust}, err
	case domain.ToolchainGo:
		goTool, err := m.resolveGo(ctx, target)
		return []artifact{goTool}, err
	default:
		return nil, fmt.Errorf("unknown toolchain profile %q", profile)
	}
}

type targetAssets struct {
	goArchitecture, nodeArchitecture, rustTriple, bunAsset, uvAsset string
}

func toolchainTarget(architecture string) (targetAssets, error) {
	switch architecture {
	case "arm64":
		return targetAssets{"arm64", "arm64", "aarch64-unknown-linux-gnu", "bun-linux-aarch64.zip", "uv-aarch64-unknown-linux-gnu.tar.gz"}, nil
	case "amd64":
		return targetAssets{"amd64", "x64", "x86_64-unknown-linux-gnu", "bun-linux-x64.zip", "uv-x86_64-unknown-linux-gnu.tar.gz"}, nil
	default:
		return targetAssets{}, fmt.Errorf("unsupported Soda toolchain architecture %q", architecture)
	}
}

func (m *Manager) resolveNode(ctx context.Context, target targetAssets) (artifact, error) {
	var releases []struct {
		Version string   `json:"version"`
		LTS     any      `json:"lts"`
		Files   []string `json:"files"`
	}
	if err := m.getJSON(ctx, "https://nodejs.org/dist/index.json", &releases); err != nil {
		return artifact{}, err
	}
	for _, release := range releases {
		assetPlatform := "linux-" + target.nodeArchitecture
		if _, ok := release.LTS.(string); !ok || !slices.Contains(release.Files, assetPlatform) {
			continue
		}
		filename := "node-" + release.Version + "-" + assetPlatform + ".tar.xz"
		base := "https://nodejs.org/dist/" + release.Version + "/"
		sums, err := m.getText(ctx, base+"SHASUMS256.txt")
		if err != nil {
			return artifact{}, err
		}
		sum, err := checksumLine(sums, filename)
		return artifact{"node", release.Version, base + filename, sum, tarXz}, err
	}
	return artifact{}, errors.New("Node active LTS release not found for this architecture")
}

func (m *Manager) resolveGo(ctx context.Context, target targetAssets) (artifact, error) {
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
			if file.OS == "linux" && file.Arch == target.goArchitecture && file.Kind == "archive" {
				return artifact{"go", release.Version, "https://go.dev/dl/" + file.Filename, file.SHA256, tarGz}, nil
			}
		}
	}
	return artifact{}, errors.New("Go Linux archive not found for this architecture")
}

func (m *Manager) resolveRust(ctx context.Context, target targetAssets) (artifact, error) {
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
	url := "https://static.rust-lang.org/rustup/dist/" + target.rustTriple + "/rustup-init"
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

func (m *Manager) request(ctx context.Context, url string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "SodaOS/0.2 toolchain resolver")
	response, err := m.client.Do(request)
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
func (m *Manager) getJSON(ctx context.Context, url string, target any) error {
	body, err := m.request(ctx, url)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, target)
}
