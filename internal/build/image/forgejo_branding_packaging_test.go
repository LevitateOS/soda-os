package image

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestForgejoBrandingRPMInstall(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	sources, buildroot := t.TempDir(), t.TempDir()
	require.NoError(t, (&Builder{Root: root}).stageForgejoBranding(sources))
	for _, name := range []string{"forgejo", "forgejo-init", "forgejo-tailnet", "forgejo.service", "forgejo-init.service", "forgejo.sysusers", "forgejo.tmpfiles", "forgejo-app.ini.tmpl", "soda-forgejo.pam", "soda-forgejo-shadow.te"} {
		require.NoError(t, os.WriteFile(filepath.Join(sources, name), []byte(name), 0600))
	}
	spec, err := os.ReadFile(filepath.Join(root, "packaging", "rpm", "forgejo", "soda-forgejo.spec"))
	require.NoError(t, err)
	script := forgejoRPMInstallScript(t, string(spec), sources, buildroot)
	output, err := exec.Command("sh", "-eu", "-c", script).CombinedOutput()
	require.NoErrorf(t, err, "%s", output)
	custom := filepath.Join(buildroot, "usr", "share", "soda", "forgejo", "custom")
	for _, file := range [][2]string{
		{"assets/branding/theme/palette.css", "public/assets/css/palette.css"},
		{"assets/branding/source/soda-symbol.svg", "public/assets/img/logo.svg"},
		{"assets/branding/source/soda-symbol.svg", "public/assets/img/favicon.svg"},
		{"assets/branding/forgejo/logo.png", "public/assets/img/logo.png"},
		{"assets/branding/forgejo/favicon.png", "public/assets/img/favicon.png"},
		{"assets/branding/forgejo/apple-touch-icon.png", "public/assets/img/apple-touch-icon.png"},
	} {
		expected, readErr := os.ReadFile(filepath.Join(root, file[0]))
		require.NoError(t, readErr)
		installed, readErr := os.ReadFile(filepath.Join(custom, file[1]))
		require.NoError(t, readErr)
		require.Equal(t, expected, installed, file[1])
	}
	compareForgejoCustomTree(t, filepath.Join(root, "packaging", "rpm", "forgejo", "sources", "custom"), custom)
	require.Contains(t, string(spec), "%{_datadir}/soda/forgejo/custom/")
	require.NotContains(t, string(spec), "preview")
	_, err = os.Stat(filepath.Join(custom, "public", "assets", "soda-theme-preview.html"))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func forgejoRPMInstallScript(t *testing.T, spec, sources, buildroot string) string {
	t.Helper()
	parts := strings.Split(spec, "%install\n")
	require.Len(t, parts, 2)
	parts = strings.Split(parts[1], "%files\n")
	require.Len(t, parts, 2)
	script := strings.NewReplacer(
		"%{buildroot}", buildroot, "%{_sourcedir}", sources,
		"%{_bindir}", "/usr/bin", "%{_libexecdir}", "/usr/libexec",
		"%{_unitdir}", "/usr/lib/systemd/system", "%{_sysusersdir}", "/usr/lib/sysusers.d",
		"%{_tmpfilesdir}", "/usr/lib/tmpfiles.d", "%{_datadir}", "/usr/share",
		"%{_sysconfdir}", "/etc",
	).Replace(parts[0])
	require.NotContains(t, script, "%{")
	return script
}

func compareForgejoCustomTree(t *testing.T, source, installed string) {
	t.Helper()
	err := filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		relative, err := filepath.Rel(source, path)
		require.NoError(t, err)
		expected, err := os.ReadFile(path)
		require.NoError(t, err)
		actual, err := os.ReadFile(filepath.Join(installed, relative))
		require.NoError(t, err)
		require.Equal(t, expected, actual, relative)
		info, err := os.Stat(filepath.Join(installed, relative))
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0644), info.Mode().Perm(), relative)
		return nil
	})
	require.NoError(t, err)
}

func TestForgejoSodaDefaultsAndTemplates(t *testing.T) {
	root := filepath.Join("..", "..", "..", "packaging", "rpm", "forgejo", "sources")
	config, err := os.ReadFile(filepath.Join(root, "app.ini.tmpl"))
	require.NoError(t, err)
	for _, text := range []string{"APP_NAME = Soda OS", "DEFAULT_THEME = soda-auto", "STATIC_CACHE_TIME = 0", "AUTHOR = Soda OS", "DESCRIPTION = Your team's repositories and collaboration."} {
		require.Contains(t, string(config), text)
	}
	themes := regexp.MustCompile(`(?m)^THEMES = (.+)$`).FindStringSubmatch(string(config))
	require.Len(t, themes, 2)
	require.ElementsMatch(t, []string{
		"soda-auto", "soda-light", "soda-dark", "forgejo-auto", "forgejo-light", "forgejo-dark",
		"gitea-auto", "gitea-light", "gitea-dark",
		"forgejo-auto-deuteranopia-protanopia", "forgejo-light-deuteranopia-protanopia", "forgejo-dark-deuteranopia-protanopia",
		"forgejo-auto-tritanopia", "forgejo-light-tritanopia", "forgejo-dark-tritanopia",
	}, strings.Split(themes[1], ","))
	home, err := os.ReadFile(filepath.Join(root, "custom", "templates", "home.tmpl"))
	require.NoError(t, err)
	for _, text := range []string{`{{template "base/head" .}}`, `{{template "base/footer" .}}`, "{{AppDisplayName}}", "{{MetaDescription}}", `width="220" height="220"`, `{{AssetUrlPrefix}}/img/logo.svg`} {
		require.Contains(t, string(home), text)
	}
	require.NotContains(t, string(home), "home_forgejo")
	header, err := os.ReadFile(filepath.Join(root, "custom", "templates", "custom", "header.tmpl"))
	require.NoError(t, err)
	require.Contains(t, string(header), `sizes="180x180" href="{{AssetUrlPrefix}}/img/apple-touch-icon.png"`)
	unit, err := os.ReadFile(filepath.Join(root, "systemd", "forgejo.service"))
	require.NoError(t, err)
	require.Contains(t, string(unit), "FORGEJO_CUSTOM=/usr/share/soda/forgejo/custom")
	require.Contains(t, string(unit), "--config /etc/forgejo/app.ini")
	require.Contains(t, string(unit), "ReadWritePaths=/var/lib/forgejo")
}
