package image

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/LevitateOS/soda-os/internal/process"
)

// Build on the matching native build host; only static output enters the RPMs.
func (b *Builder) buildCockpit(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	for _, args := range [][]string{{"install", "--frozen-lockfile"}, {"build"}} {
		if err := b.runner.Run(ctx, process.Command{Dir: b.path("cockpit"), Name: "vp", Args: args}); err != nil {
			return fmt.Errorf("build Cockpit assets (requires the pinned Vite+ build toolchain): %w", err)
		}
	}
	return nil
}

func (b *Builder) stageCockpitSources(sources string) error {
	for _, name := range []string{"soda-projects", "soda-runners", "soda-tailscale", "soda-updates"} {
		root := b.path(filepath.Join("cockpit", "dist", name))
		for _, file := range []string{"manifest.json", "index.html", "LICENSES.txt", "assets"} {
			if _, err := os.Stat(filepath.Join(root, file)); err != nil {
				return fmt.Errorf("missing built Cockpit package; run vp -C cockpit build: %w", err)
			}
		}
		if err := os.CopyFS(filepath.Join(sources, name+"-cockpit"), os.DirFS(root)); err != nil {
			return fmt.Errorf("stage %s: %w", name, err)
		}
		if err := normalizeCockpitAssetModes(filepath.Join(sources, name+"-cockpit")); err != nil {
			return fmt.Errorf("normalize %s modes: %w", name, err)
		}
	}
	return nil
}

func normalizeCockpitAssetModes(root string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if entry.IsDir() {
			mode = 0o755
		}
		return os.Chmod(path, mode)
	})
}
