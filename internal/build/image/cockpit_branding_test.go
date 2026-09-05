package image

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCockpitBrandingInstallsOnlyRuntimeAssets(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	data, err := os.ReadFile(filepath.Join(root, "packaging/rpm/projects/soda-projects.spec"))
	require.NoError(t, err)
	var installed []string
	for line := range strings.Lines(string(data)) {
		if strings.HasPrefix(line, "install -m 0644 ") && strings.Contains(line, "/cockpit/branding/sodaos/") {
			fields := strings.Fields(line)
			installed = append(installed, filepath.Base(fields[len(fields)-1]))
		}
	}
	require.ElementsMatch(t, []string{
		"branding.css", "palette.css", "soda-symbol.svg",
		"soda-logo-horizontal.svg", "soda-logo-horizontal-dark.svg",
		"login-background-light.svg", "login-background-dark.svg",
		"favicon.ico", "apple-touch-icon.png",
	}, installed)
	require.NotContains(t, string(data), "/cockpit/static/")
	require.NotContains(t, string(data), "/etc/cockpit/branding")
}

func TestCockpitBrandingRetainsNativeLoginAndTheme(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	data, err := os.ReadFile(filepath.Join(root, "packaging/rpm/projects/sources/branding/sodaos/branding.css"))
	require.NoError(t, err)
	css := string(data)
	for _, expected := range []string{
		`@import url("palette.css")`, "body.login-pf #brand::before", ".pf-v6-theme-dark",
		"--color-background: var(--soda-surface)", "--color-primary: var(--soda-brand)",
		"--button-color-text: var(--soda-on-brand)", "--button-color-hover-text: var(--soda-on-brand)",
		"login-background-light.svg", "login-background-dark.svg", "alt='Soda OS'",
	} {
		require.Contains(t, css, expected)
	}
	for _, forbidden := range []string{
		"<script", "CustomLoginPage", "--color-text-white:", "--color-danger:",
		"--color-warning:", "--color-disabled", "https://", "http://",
	} {
		require.NotContains(t, css, forbidden)
	}
}
