package installer

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

type svgCircle struct {
	CX          string `xml:"cx,attr"`
	CY          string `xml:"cy,attr"`
	R           string `xml:"r,attr"`
	Fill        string `xml:"fill,attr"`
	Stroke      string `xml:"stroke,attr"`
	StrokeWidth string `xml:"stroke-width,attr"`
}

type svgGroup struct {
	Circles []svgCircle `xml:"circle"`
}

type svgDocument struct {
	Circles []svgCircle `xml:"circle"`
	Groups  []svgGroup  `xml:"g"`
}

func TestBrandingVariantsKeepSymbolBoundaryVisible(t *testing.T) {
	root := filepath.Join("..", "..", "..", "assets", "branding", "source")
	variants := []struct {
		name        string
		fill        string
		stroke      string
		strokeWidth string
	}{
		{"soda-symbol.svg", "#06245B", "#FFFFFF", "8"},
		{"soda-logo-horizontal.svg", "#06245B", "#FFFFFF", "8"},
		{"soda-logo-horizontal-dark.svg", "#06245B", "#FFFFFF", "8"},
		{"soda-symbol-white.svg", "none", "#FFFFFF", "12"},
		{"soda-logo-white.svg", "none", "#FFFFFF", "12"},
		{"soda-symbol-navy.svg", "none", "#06245B", "12"},
		{"soda-logo-navy.svg", "none", "#06245B", "12"},
		{"soda-symbol-black.svg", "none", "#000000", "12"},
		{"soda-logo-black.svg", "none", "#000000", "12"},
	}

	for _, variant := range variants {
		t.Run(variant.name, func(t *testing.T) {
			circle := readSVGOuterCircle(t, filepath.Join(root, variant.name))
			require.Equal(t, variant.fill, circle.Fill)
			require.Equal(t, variant.stroke, circle.Stroke)
			require.Equal(t, variant.strokeWidth, circle.StrokeWidth)
		})
	}
}

func readSVGOuterCircle(t *testing.T, path string) svgCircle {
	t.Helper()
	contents, err := os.ReadFile(path)
	require.NoError(t, err)

	var document svgDocument
	require.NoError(t, xml.Unmarshal(contents, &document))
	circles := document.Circles
	for _, group := range document.Groups {
		circles = append(circles, group.Circles...)
	}
	for _, circle := range circles {
		if circle.CX == "128" && circle.CY == "128" && circle.R == "116" {
			return circle
		}
	}
	t.Fatalf("%s has no outer symbol circle", path)
	return svgCircle{}
}
