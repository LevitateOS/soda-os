package image

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LevitateOS/soda-os/internal/process"
)

// PrepareLocalBootcBase binds BuildKit's fedora-base context to the exact
// pinned manifest retained in the repository reference-media area. Quay tags
// and retention are not part of the reproducible Soda build contract.
func PrepareLocalBootcBase(ctx context.Context, root string, runner process.Runner, reference string) (string, error) {
	if reference != bootcBaseReference {
		return "", errors.New("local Fedora bootc base differs from the approved digest")
	}
	digest := strings.TrimPrefix(reference, "quay.io/fedora/fedora-bootc@")
	localTag := "soda-fedora-bootc:" + strings.ReplaceAll(digest, ":", "-")
	tag := process.Command{Dir: root, Name: "docker", Args: []string{"image", "tag", digest, localTag}}
	if err := runner.Run(ctx, tag); err == nil {
		return localTag, nil
	}
	archive := filepath.Join(root, bootcBaseArchive)
	info, err := os.Stat(archive)
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("load pinned Fedora bootc base: %s is unavailable", archive)
	}
	if err := runner.Run(ctx, process.Command{Dir: root, Name: "docker", Args: []string{"load", "--input", archive}}); err != nil {
		return "", fmt.Errorf("load pinned Fedora bootc base: %w", err)
	}
	if err := runner.Run(ctx, tag); err != nil {
		return "", fmt.Errorf("bind pinned Fedora bootc base digest: %w", err)
	}
	return localTag, nil
}
