package scripts

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestForgejoBrandingPNGPlacements(t *testing.T) {
	for _, asset := range []struct {
		name   string
		size   int
		opaque bool
		corner color.RGBA
	}{
		{"favicon-16.png", 16, false, color.RGBA{}},
		{"favicon.png", 32, false, color.RGBA{}},
		{"apple-touch-icon.png", 180, true, color.RGBA{R: 6, G: 36, B: 91, A: 255}},
		{"logo.png", 512, false, color.RGBA{}},
	} {
		t.Run(asset.name, func(t *testing.T) {
			file, err := os.Open(filepath.Join("..", "assets", "branding", "forgejo", asset.name))
			require.NoError(t, err)
			decoded, err := png.Decode(file)
			require.NoError(t, file.Close())
			require.NoError(t, err)
			require.Equal(t, image.Rect(0, 0, asset.size, asset.size), decoded.Bounds())
			rgba := image.NewRGBA(decoded.Bounds())
			draw.Draw(rgba, rgba.Bounds(), decoded, decoded.Bounds().Min, draw.Src)
			require.Equal(t, asset.opaque, rgba.Opaque())
			require.Equal(t, asset.corner, rgba.RGBAAt(0, 0))
		})
	}
}

func TestForgejoBrandingMatchesSVGMaster(t *testing.T) {
	check := exec.Command("scripts/render-forgejo-branding.sh", "--check")
	check.Dir = ".."
	output, err := check.CombinedOutput()
	require.NoErrorf(t, err, "Forgejo branding is stale:\n%s", output)
}

func TestForgejoBrandingRendererRejectsUnknownArguments(t *testing.T) {
	for _, args := range [][]string{{"--unknown"}, {"--check", "extra"}} {
		command := exec.Command("./render-forgejo-branding.sh", args...)
		output, err := command.CombinedOutput()
		require.Error(t, err)
		require.Equal(t, 2, command.ProcessState.ExitCode())
		require.Contains(t, string(output), "usage:")
	}
}
