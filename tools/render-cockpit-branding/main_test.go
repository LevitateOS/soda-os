package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIconDerivatives(t *testing.T) {
	t.Chdir("../..")
	out := t.TempDir()
	require.NoError(t, render(out))
	for _, size := range []int{16, 32, 48, 180} {
		name := "favicon-" + strconv.Itoa(size) + ".png"
		if size == 180 {
			name = "apple-touch-icon.png"
		}
		actual := readPNG(t, filepath.Join("assets/branding/cockpit", name))
		expected := readPNG(t, filepath.Join(out, name))
		require.Equal(t, image.Rect(0, 0, size, size), actual.Bounds())
		require.Equal(t, expected.Pix, actual.Pix, "%s is stale", name)
		require.Equal(t, size == 180, actual.Opaque())
		if size == 180 {
			require.Equal(t, color.NRGBA{6, 36, 91, 255}, actual.NRGBAAt(0, 0))
		}
	}
}

func readPNG(t *testing.T, path string) *image.NRGBA {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	decoded, err := png.Decode(bytes.NewReader(data))
	require.NoError(t, err)
	out := image.NewNRGBA(decoded.Bounds())
	draw.Draw(out, out.Bounds(), decoded, decoded.Bounds().Min, draw.Src)
	return out
}

func TestICOContainsExactPNGFrames(t *testing.T) {
	root := "../../assets/branding/cockpit"
	data, err := os.ReadFile(filepath.Join(root, "favicon.ico"))
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(data), 54)
	require.Equal(t, []byte{0, 0, 1, 0, 3, 0}, data[:6])
	offset := uint32(54)
	for i, size := range []int{16, 32, 48} {
		entry := data[6+i*16 : 22+i*16]
		require.Equal(t, []byte{byte(size), byte(size), 0, 0, 1, 0, 32, 0}, entry[:8])
		length := binary.LittleEndian.Uint32(entry[8:])
		require.Equal(t, offset, binary.LittleEndian.Uint32(entry[12:]))
		require.LessOrEqual(t, uint64(offset)+uint64(length), uint64(len(data)))
		frame, err := os.ReadFile(filepath.Join(root, "favicon-"+strconv.Itoa(size)+".png"))
		require.NoError(t, err)
		require.Equal(t, frame, data[offset:offset+length])
		offset += length
	}
	require.Equal(t, uint32(len(data)), offset)
}

func TestPaletteContrast(t *testing.T) {
	data, err := os.ReadFile("../../assets/branding/cockpit/palette.css")
	require.NoError(t, err)
	blocks := regexp.MustCompile(`(?s)(?:\.soda-theme-light|\.pf-v6-theme-dark) \{([^}]+)\}`).FindAllSubmatch(data, -1)
	require.GreaterOrEqual(t, len(blocks), 2)
	for _, block := range blocks[:2] {
		tokens := map[string]string{}
		for _, token := range regexp.MustCompile(`--soda-([a-z-]+): (#[0-9a-f]{6});`).FindAllSubmatch(block[1], -1) {
			tokens[string(token[1])] = string(token[2])
		}
		checkThemeContrast(t, tokens)
	}
}

func checkThemeContrast(t *testing.T, tokens map[string]string) {
	t.Helper()
	for _, background := range []string{"canvas", "surface"} {
		for _, foreground := range []string{"text", "muted", "brand", "brand-hover", "brand-pressed"} {
			require.GreaterOrEqual(t, contrast(t, tokens[foreground], tokens[background]), 4.5, "%s on %s", foreground, background)
		}
		for _, foreground := range []string{"border", "focus"} {
			require.GreaterOrEqual(t, contrast(t, tokens[foreground], tokens[background]), 3.0, "%s on %s", foreground, background)
		}
	}
	for _, background := range []string{"brand", "brand-hover", "brand-pressed"} {
		require.GreaterOrEqual(t, contrast(t, tokens["on-brand"], tokens[background]), 4.5, "button text on %s", background)
	}
}

func contrast(t *testing.T, a, b string) float64 {
	t.Helper()
	x, y := luminance(t, a), luminance(t, b)
	return (math.Max(x, y) + 0.05) / (math.Min(x, y) + 0.05)
}

func luminance(t *testing.T, hex string) float64 {
	t.Helper()
	require.Len(t, hex, 7)
	var result float64
	for i, weight := range []float64{0.2126, 0.7152, 0.0722} {
		value, err := strconv.ParseUint(hex[1+i*2:3+i*2], 16, 8)
		require.NoError(t, err)
		c := float64(value) / 255
		if c <= 0.04045 {
			c /= 12.92
		} else {
			c = math.Pow((c+0.055)/1.055, 2.4)
		}
		result += c * weight
	}
	return result
}
