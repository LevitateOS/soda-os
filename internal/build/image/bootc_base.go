package image

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/LevitateOS/soda-os/internal/config"
	"github.com/LevitateOS/soda-os/internal/process"
)

// PrepareLocalBootcBase binds BuildKit's fedora-base context to the exact
// pinned manifest retained in the repository reference-media area. Quay tags
// and retention are not part of the reproducible Soda build contract.
func PrepareLocalBootcBase(ctx context.Context, root string, runner process.Runner, platform config.PlatformSpec) (string, error) {
	if platform.BaseReference == "" || platform.BaseArchive == "" || len(platform.BaseArchiveSHA256) != 64 {
		return "", errors.New("local Fedora bootc base contract is incomplete")
	}
	digest := strings.TrimPrefix(platform.BaseReference, "quay.io/fedora/fedora-bootc@")
	if digest == platform.BaseReference || !strings.HasPrefix(digest, "sha256:") {
		return "", errors.New("local Fedora bootc base differs from the approved digest contract")
	}
	localTag := "soda-fedora-bootc:" + strings.ReplaceAll(digest, ":", "-")
	tag := process.Command{Dir: root, Name: "docker", Args: []string{"image", "tag", digest, localTag}}
	if err := runner.Run(ctx, tag); err == nil {
		return localTag, nil
	}
	archive := filepath.Join(root, platform.BaseArchive)
	info, err := os.Stat(archive)
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("load pinned Fedora bootc base: %s is unavailable", archive)
	}
	file, err := os.Open(archive)
	if err != nil {
		return "", fmt.Errorf("read pinned Fedora bootc base: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("checksum pinned Fedora bootc base: %w", err)
	}
	if hex.EncodeToString(hash.Sum(nil)) != platform.BaseArchiveSHA256 {
		return "", errors.New("pinned Fedora bootc base archive checksum differs from the selected platform contract")
	}
	if err := runner.Run(ctx, process.Command{Dir: root, Name: "docker", Args: []string{"load", "--input", archive}}); err != nil {
		return "", fmt.Errorf("load pinned Fedora bootc base: %w", err)
	}
	if err := runner.Run(ctx, tag); err != nil {
		return "", fmt.Errorf("bind pinned Fedora bootc base digest: %w", err)
	}
	return localTag, nil
}
