package toolchain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (m *Manager) installArtifact(ctx context.Context, item artifact) (string, error) {
	destination := filepath.Join(m.root, item.tool, item.version)
	if _, err := os.Stat(filepath.Join(destination, ".ready")); err == nil {
		if err := makeReadable(m.root, destination); err != nil {
			return "", err
		}
		return artifactBin(item, destination), nil
	}
	payload, err := m.request(ctx, item.url)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(payload)
	actual := hex.EncodeToString(digest[:])
	if !strings.EqualFold(actual, item.checksum) {
		return "", fmt.Errorf("toolchain checksum mismatch: expected %s, got %s", item.checksum, actual)
	}
	return m.materializeArtifact(ctx, item, destination, payload)
}

func (m *Manager) materializeArtifact(ctx context.Context, item artifact, destination string, payload []byte) (string, error) {
	if err := os.RemoveAll(destination); err != nil {
		return "", err
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return "", err
	}
	if err := m.unpackArtifact(ctx, item, payload, destination); err != nil {
		return "", err
	}
	if normalizedToolBinaries[item.tool] {
		if err := normalizeBinary(destination, item.tool); err != nil {
			return "", err
		}
	}
	if err := os.WriteFile(filepath.Join(destination, ".ready"), []byte("ready\n"), 0o644); err != nil {
		return "", err
	}
	if err := makeReadable(m.root, destination); err != nil {
		return "", err
	}
	return artifactBin(item, destination), nil
}

func (m *Manager) unpackArtifact(ctx context.Context, item artifact, payload []byte, destination string) error {
	switch item.kind {
	case tarGz:
		return extractTarGz(payload, destination)
	case tarXz:
		return extractTarXz(ctx, payload, destination)
	case zipArchive:
		return extractZip(payload, destination)
	case executable:
		return m.installRustup(ctx, payload, destination)
	default:
		return errors.New("unsupported toolchain artifact")
	}
}

func (m *Manager) installRustup(ctx context.Context, payload []byte, destination string) error {
	temporary, err := os.CreateTemp(m.root, ".rustup-init-")
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
	root := filepath.Join(m.root, "python")
	command := exec.CommandContext(ctx, filepath.Join(m.root, "uv", uv.version, "bin", "uv"), "python", "install")
	command.Env = append(os.Environ(), "UV_PYTHON_INSTALL_DIR="+root)
	if output, err := command.CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("uv python install: %w: %s", err, strings.TrimSpace(string(output)))
	}
	bin, err := firstPythonBin(root)
	if err != nil {
		return "", "", err
	}
	output, err := exec.CommandContext(ctx, filepath.Join(bin, "python3"), "--version").CombinedOutput()
	if err != nil {
		return "", "", err
	}
	if err = makeReadable(m.root, root); err != nil {
		return "", "", err
	}
	return bin, strings.TrimSpace(string(output)), nil
}
