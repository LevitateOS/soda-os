package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEqualPNGComparesDecodedPixels(t *testing.T) {
	root := t.TempDir()
	left := filepath.Join(root, "left.png")
	right := filepath.Join(root, "right.png")
	fixture := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	fixture.SetNRGBA(0, 0, color.NRGBA{R: 6, G: 36, B: 91, A: 255})
	fixture.SetNRGBA(1, 0, color.NRGBA{R: 255, G: 255, B: 255, A: 0})
	writePNG(t, left, fixture, png.BestSpeed)
	writePNG(t, right, fixture, png.BestCompression)

	equal, err := equalPNG(left, right)
	require.NoError(t, err)
	require.True(t, equal)

	fixture.SetNRGBA(1, 0, color.NRGBA{R: 1, A: 255})
	writePNG(t, right, fixture, png.BestCompression)
	equal, err = equalPNG(left, right)
	require.NoError(t, err)
	require.False(t, equal)
}

func writePNG(t *testing.T, path string, value image.Image, compression png.CompressionLevel) {
	t.Helper()
	file, err := os.Create(path)
	require.NoError(t, err)
	require.NoError(t, (&png.Encoder{CompressionLevel: compression}).Encode(file, value))
	require.NoError(t, file.Close())
}
