package image

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSharedPaletteHasNoApplicationPolicy(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	palette, err := os.ReadFile(filepath.Join(root, "assets/branding/theme/palette.css"))
	require.NoError(t, err)
	for _, forbidden := range []string{"@import", "url(", "@media", "--pf-", "--color-", ".pf-v6-theme-dark", "color-scheme:"} {
		require.NotContains(t, string(palette), forbidden)
	}
	values := map[string]bool{}
	for _, match := range regexp.MustCompile(`(--soda-[\w-]+):`).FindAllSubmatch(palette, -1) {
		name := string(match[1])
		require.False(t, values[name], "duplicate token %s", name)
		values[name] = true
	}
	for _, adapter := range []string{
		"cockpit/src/cockpit/theme.css",
		"packaging/rpm/forgejo/sources/custom/public/assets/css/theme-soda-light.css",
		"packaging/rpm/forgejo/sources/custom/public/assets/css/theme-soda-dark.css",
	} {
		css, readErr := os.ReadFile(filepath.Join(root, adapter))
		require.NoError(t, readErr)
		require.NotRegexp(t, `#[0-9a-fA-F]{6}|rgba?\(|hsla?\(`, string(css), adapter)
		refs := regexp.MustCompile(`var\((--soda-(?:light|dark)-[\w-]+)\)`).FindAllSubmatch(css, -1)
		require.NotEmpty(t, refs, adapter)
		for _, ref := range refs {
			require.True(t, values[string(ref[1])], "%s: unresolved %s", adapter, ref[1])
		}
	}
}
