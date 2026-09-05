package image

import (
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestForgejoSodaThemesExtendNativeThemes(t *testing.T) {
	root := filepath.Join("..", "..", "..", "packaging", "rpm", "forgejo", "sources", "custom", "public", "assets", "css")
	for _, mode := range []string{"light", "dark"} {
		contents, err := os.ReadFile(filepath.Join(root, "theme-soda-"+mode+".css"))
		require.NoError(t, err)
		css := string(contents)
		require.Contains(t, css, `@import "theme-forgejo-`+mode+`.css";`)
		require.Contains(t, css, `@import "soda-controls.css";`)
		require.Contains(t, css, `@import "palette.css";`)
		require.NotRegexp(t, `#[0-9a-fA-F]{6}`, css)
		require.Contains(t, css, "color-scheme: "+mode+";")
		for _, untouched := range []string{"--color-diff-", "--color-ansi-", "--color-error-", "--color-red:", "--color-green:"} {
			require.NotContains(t, css, untouched)
		}
		require.NotContains(t, css, "https:")
	}
	auto, err := os.ReadFile(filepath.Join(root, "theme-soda-auto.css"))
	require.NoError(t, err)
	require.Contains(t, string(auto), `@import "theme-soda-light.css";`)
	require.Contains(t, string(auto), `@import "theme-soda-dark.css" (prefers-color-scheme: dark);`)
	controls, err := os.ReadFile(filepath.Join(root, "soda-controls.css"))
	require.NoError(t, err)
	for _, selector := range []string{".ui.primary.button", ".ui.primary.buttons .button", ".button.primary", ":focus-visible"} {
		require.Contains(t, string(controls), selector)
	}
	require.Contains(t, string(controls), "--color-primary-contrast: var(--soda-button-text);")
}

func TestForgejoSodaPaletteContrast(t *testing.T) {
	for _, mode := range []string{"light", "dark"} {
		colors := forgejoThemeColors(t, mode)
		for _, pair := range [][2]string{
			{"--color-text", "--color-body"},
			{"--color-text", "--color-box-body"},
			{"--color-text-light", "--color-box-header"},
			{"--color-primary", "--color-body"},
			{"--color-primary", "--color-box-body"},
			{"--color-primary", "--color-box-header"},
			{"--color-input-text", "--color-input-background"},
			{"#ffffff", "--soda-button-bg"},
			{"#ffffff", "--soda-button-hover"},
			{"#ffffff", "--soda-button-active"},
			{"--color-success-text", "--color-success-bg"},
			{"--color-warning-text", "--color-warning-bg"},
			{"--color-info-text", "--color-info-bg"},
			{"--color-selection-fg", "--color-selection-bg"},
		} {
			require.GreaterOrEqual(t, forgejoContrast(t, colors[pair[0]], colors[pair[1]]), 4.5, "%s %v", mode, pair)
		}
		for _, pair := range [][2]string{
			{"--color-input-border", "--color-input-background"},
			{"--soda-focus", "--color-body"},
			{"--soda-focus", "--color-box-body"},
		} {
			require.GreaterOrEqual(t, forgejoContrast(t, colors[pair[0]], colors[pair[1]]), 3.0, "%s %v", mode, pair)
		}
	}
}

func forgejoThemeColors(t *testing.T, mode string) map[string]string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "packaging", "rpm", "forgejo", "sources", "custom", "public", "assets", "css", "theme-soda-"+mode+".css")
	contents, err := os.ReadFile(path)
	require.NoError(t, err)
	colors := map[string]string{"#ffffff": "#ffffff"}
	palette, err := os.ReadFile(filepath.Join("..", "..", "..", "assets", "branding", "theme", "palette.css"))
	require.NoError(t, err)
	for _, match := range regexp.MustCompile(`(?m)^  (--[\w-]+): (#[0-9a-f]{6}(?:[0-9a-f]{2})?);$`).FindAllStringSubmatch(string(palette), -1) {
		colors[match[1]] = match[2]
	}
	for _, match := range regexp.MustCompile(`(?m)^  (--[\w-]+): var\((--[\w-]+)\);$`).FindAllStringSubmatch(string(contents), -1) {
		require.NotEmpty(t, colors[match[2]], "unresolved palette reference %s", match[2])
		colors[match[1]] = colors[match[2]]
	}
	return colors
}

func forgejoContrast(t *testing.T, foreground, background string) float64 {
	t.Helper()
	left := forgejoLuminance(t, foreground)
	right := forgejoLuminance(t, background)
	return (math.Max(left, right) + 0.05) / (math.Min(left, right) + 0.05)
}

func forgejoLuminance(t *testing.T, hex string) float64 {
	t.Helper()
	require.Len(t, hex, 7, "missing or invalid theme color")
	value, err := strconv.ParseUint(strings.TrimPrefix(hex, "#"), 16, 32)
	require.NoError(t, err)
	var result float64
	for index, weight := range []float64{0.2126, 0.7152, 0.0722} {
		channel := float64((value>>uint(16-index*8))&255) / 255
		if channel <= 0.04045 {
			channel /= 12.92
		} else {
			channel = math.Pow((channel+0.055)/1.055, 2.4)
		}
		result += channel * weight
	}
	return result
}
