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

func TestCockpitStockPageColorCoverage(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	data, err := os.ReadFile(filepath.Join(root, "packaging/bootc/Containerfile"))
	require.NoError(t, err)
	for _, page := range []string{"systemd/logs", "systemd/services", "systemd/terminal", "systemd/hwinfo", "networkmanager/firewall"} {
		require.Contains(t, string(data), page)
	}
	require.Contains(t, string(data), `grep -Fq '</head>' "/usr/share/cockpit/${page}.html" || exit 1`)
	require.Contains(t, string(data), `<link href="../../static/branding.css" rel="stylesheet" />`)
	data, err = os.ReadFile(filepath.Join(root, "assets/branding/cockpit/palette.css"))
	require.NoError(t, err)
	for _, token := range []string{
		"--pf-t--global--background--color--primary--default: var(--soda-surface)",
		"--pf-t--global--background--color--secondary--default: var(--soda-canvas)",
		"--pf-t--global--text--color--regular: var(--soda-text)",
		"--pf-t--global--text--color--link--default: var(--soda-brand)",
		"--pf-t--global--background--color--action--plain--alt--clicked: var(--soda-surface-pressed)",
	} {
		require.Contains(t, string(data), token)
	}
	require.NotContains(t, string(data), "--status--")
	require.NotContains(t, string(data), "--disabled--")
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
