package toolchain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/LevitateOS/soda-os/internal/domain"
	"golang.org/x/sys/unix"
)

func (m *Manager) Install(ctx context.Context, profile domain.ToolchainProfile) (domain.ToolchainInstallation, error) {
	artifacts, err := m.resolve(ctx, profile)
	if err != nil {
		return domain.ToolchainInstallation{}, err
	}
	release, err := m.lockProfile(profile)
	if err != nil {
		return domain.ToolchainInstallation{}, err
	}
	defer release()
	versions := make([]string, 0, len(artifacts))
	paths := make([]string, 0, len(artifacts))
	for _, item := range artifacts {
		bin, installErr := m.installArtifact(ctx, item)
		if installErr != nil {
			return domain.ToolchainInstallation{}, installErr
		}
		versions = append(versions, item.tool+"="+item.version)
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
	return m.writeInstallation(profile, artifacts, version, paths)
}

func (m *Manager) lockProfile(profile domain.ToolchainProfile) (func(), error) {
	if err := os.MkdirAll(m.root, 0o755); err != nil {
		return nil, err
	}
	if err := os.Chmod(m.root, 0o755); err != nil {
		return nil, err
	}
	lock, err := os.OpenFile(filepath.Join(m.root, "."+string(profile)+".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX); err != nil {
		_ = lock.Close()
		return nil, err
	}
	return func() {
		_ = unix.Flock(int(lock.Fd()), unix.LOCK_UN)
		_ = lock.Close()
	}, nil
}

func (m *Manager) writeInstallation(profile domain.ToolchainProfile, artifacts []artifact, version string, paths []string) (domain.ToolchainInstallation, error) {
	digest := sha256.Sum256([]byte(version))
	profileRoot := filepath.Join(m.root, "profiles", string(profile), hex.EncodeToString(digest[:]))
	if err := os.MkdirAll(profileRoot, 0o755); err != nil {
		return domain.ToolchainInstallation{}, err
	}
	environment := domain.ProjectEnvironment{Profile: string(profile), Path: paths}
	if profile == domain.ToolchainRust {
		rust := findArtifact(artifacts, "rust")
		if rust == nil {
			return domain.ToolchainInstallation{}, errors.New("Rust profile is missing rustup")
		}
		environment.Variables = map[string]string{
			"RUSTUP_HOME": filepath.Join(m.root, "rust", rust.version, "rustup"),
			"CARGO_HOME":  filepath.Join(m.root, "rust", rust.version, "cargo"),
		}
	}
	encoded, err := json.Marshal(environment)
	if err != nil {
		return domain.ToolchainInstallation{}, err
	}
	if err = os.WriteFile(filepath.Join(profileRoot, "environment.json"), append(encoded, '\n'), 0o644); err != nil {
		return domain.ToolchainInstallation{}, err
	}
	if err = makeReadable(m.root, profileRoot); err != nil {
		return domain.ToolchainInstallation{}, err
	}
	checksums := sha256.New()
	for _, item := range artifacts {
		_, _ = checksums.Write([]byte(item.checksum))
	}
	return domain.ToolchainInstallation{Profile: profile, Version: version, Path: profileRoot, Checksum: hex.EncodeToString(checksums.Sum(nil)), State: domain.JobReady}, nil
}
