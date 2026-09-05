package image

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestForgejoInitializationSeedsBrandingOnce(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "app.ini")
	calls := filepath.Join(root, "calls")
	writeWelcomeTestCommand(t, root, "hostnamectl", "echo soda\n")
	writeWelcomeTestCommand(t, root, "forgejo-tailnet", "exit 1\n")
	writeWelcomeTestCommand(t, root, "install", ":\n")
	writeWelcomeTestCommand(t, root, "chown", ":\n")
	writeWelcomeTestCommand(t, root, "runuser", "printf '%s\\n' \"$*\" >>\"$TEST_CALLS\"\n")
	writeWelcomeTestCommand(t, root, "forgejo", "printf '%s\\n' \"$*\" >>\"$TEST_CALLS\"\nprintf 'fixture-%s\\n' \"$3\"\n")
	source, err := filepath.Abs(filepath.Join("..", "..", "..", "packaging", "rpm", "forgejo", "sources"))
	require.NoError(t, err)
	script, err := os.ReadFile(filepath.Join(source, "forgejo-init"))
	require.NoError(t, err)
	body := strings.NewReplacer(
		"/etc/forgejo/app.ini", config,
		"/usr/share/soda/forgejo/app.ini.tmpl", filepath.Join(source, "app.ini.tmpl"),
		"/usr/libexec/soda/forgejo-tailnet", filepath.Join(root, "forgejo-tailnet"),
	).Replace(string(script))
	path := filepath.Join(root, "forgejo-init")
	require.NoError(t, os.WriteFile(path, []byte(body), 0700))
	command := exec.Command("sh", path)
	command.Env = append(os.Environ(), "PATH="+root+":"+os.Getenv("PATH"), "TEST_CALLS="+calls)
	output, err := command.CombinedOutput()
	require.NoErrorf(t, err, "%s", output)
	initial, err := os.ReadFile(config)
	require.NoError(t, err)
	for _, expected := range []string{"APP_NAME = Soda OS", "DEFAULT_THEME = soda-auto", "STATIC_CACHE_TIME = 0", "SECRET_KEY = fixture-SECRET_KEY", "HTTP_ADDR = 0.0.0.0"} {
		require.Contains(t, string(initial), expected)
	}
	require.NotContains(t, string(initial), "@SECRET_KEY@")
	info, err := os.Stat(config)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0640), info.Mode().Perm())
	operatorConfig := strings.NewReplacer("APP_NAME = Soda OS", "APP_NAME = Team Git", "DEFAULT_THEME = soda-auto", "DEFAULT_THEME = forgejo-dark", "STATIC_CACHE_TIME = 0", "STATIC_CACHE_TIME = 1h").Replace(string(initial))
	require.NoError(t, os.WriteFile(config, []byte(operatorConfig), 0640))
	command = exec.Command("sh", path)
	command.Env = append(os.Environ(), "PATH="+root+":"+os.Getenv("PATH"), "TEST_CALLS="+calls)
	output, err = command.CombinedOutput()
	require.NoErrorf(t, err, "%s", output)
	retained, err := os.ReadFile(config)
	require.NoError(t, err)
	require.Equal(t, operatorConfig, string(retained))
	log, err := os.ReadFile(calls)
	require.NoError(t, err)
	require.Equal(t, 4, strings.Count(string(log), "generate secret "), "secrets must only be generated on first initialization")
	require.Contains(t, string(log), "--service-name soda-forgejo")
}
