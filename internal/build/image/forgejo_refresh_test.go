package image

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Execute the packaged entry point with only external process/filesystem
// boundaries redirected into the test directory.
func TestForgejoTailnetRefresh(t *testing.T) {
	for _, tc := range []struct {
		name, config, endpoint string
		restart, fail          bool
	}{
		{"matching", "[server]\nDOMAIN = soda.test.ts.net\nSSH_DOMAIN = soda.test.ts.net\nROOT_URL = http://soda.test.ts.net:30000/\n", "soda.test.ts.net 100.64.0.1", false, false},
		{"stale", "[server]\nDOMAIN = soda\nSSH_DOMAIN = soda\nROOT_URL = http://soda:30000/\n", "soda.test.ts.net 100.64.0.1", true, false},
		{"wrong SSH address", "[server]\nDOMAIN = soda.test.ts.net\nSSH_DOMAIN = soda\nROOT_URL = http://soda.test.ts.net:30000/\n", "soda.test.ts.net 100.64.0.1", true, false},
		{"different section", "[other]\nDOMAIN = soda.test.ts.net\nSSH_DOMAIN = soda.test.ts.net\nROOT_URL = http://soda.test.ts.net:30000/\n", "soda.test.ts.net 100.64.0.1", true, false},
		{"not enrolled", "unchanged\n", "", false, true},
		{"restart failure", "[server]\nDOMAIN = soda\n", "soda.test.ts.net 100.64.0.1", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			config := filepath.Join(root, "app.ini")
			require.NoError(t, os.WriteFile(config, []byte(tc.config), 0600))
			before, err := os.Stat(config)
			require.NoError(t, err)
			writeWelcomeTestCommand(t, root, "hostnamectl", "printf 'soda\\n'\n")
			writeWelcomeTestCommand(t, root, "forgejo-tailnet", "[ -n \"$TEST_ENDPOINT\" ] || exit 1\nprintf '%s\\n' \"$TEST_ENDPOINT\"\n")
			writeWelcomeTestCommand(t, root, "systemctl", "printf '%s\\n' \"$*\" >\"$TEST_RESTART\"\nexit \"$TEST_EXIT\"\n")
			script, err := os.ReadFile(filepath.Join("..", "..", "..", "packaging", "rpm", "forgejo", "sources", "forgejo-init"))
			require.NoError(t, err)
			body := strings.NewReplacer("/etc/forgejo/app.ini", config, "/usr/libexec/soda/forgejo-tailnet", filepath.Join(root, "forgejo-tailnet")).Replace(string(script))
			path := filepath.Join(root, "forgejo-init")
			require.NoError(t, os.WriteFile(path, []byte(body), 0700))
			exit := map[bool]string{false: "0", true: "1"}[tc.fail]
			command := exec.Command("sh", path, "refresh-tailnet")
			command.Env = append(os.Environ(), "PATH="+root+":"+os.Getenv("PATH"), "TEST_ENDPOINT="+tc.endpoint, "TEST_RESTART="+filepath.Join(root, "restart"), "TEST_EXIT="+exit)
			output, err := command.CombinedOutput()
			require.Equal(t, tc.fail, err != nil, "%s", output)
			got, err := os.ReadFile(config)
			require.NoError(t, err)
			require.Equal(t, tc.config, string(got), "refresh must not rewrite a running service's configuration")
			after, err := os.Stat(config)
			require.NoError(t, err)
			require.Equal(t, before.ModTime(), after.ModTime())
			call, err := os.ReadFile(filepath.Join(root, "restart"))
			if tc.restart {
				require.NoError(t, err)
				require.Equal(t, "restart forgejo.service\n", string(call))
			} else {
				require.ErrorIs(t, err, os.ErrNotExist)
			}
		})
	}
}

func TestForgejoInitializationKeepsNativeOrdering(t *testing.T) {
	root := filepath.Join("..", "..", "..", "packaging", "rpm", "forgejo", "sources", "systemd")
	init, err := os.ReadFile(filepath.Join(root, "forgejo-init.service"))
	require.NoError(t, err)
	require.Contains(t, string(init), "Type=oneshot")
	require.NotContains(t, string(init), "RemainAfterExit")
	service, err := os.ReadFile(filepath.Join(root, "forgejo.service"))
	require.NoError(t, err)
	require.Contains(t, string(service), "Requires=forgejo-init.service")
	require.Contains(t, string(service), "After=forgejo-init.service ")
	require.NotContains(t, string(init)+string(service), "cloud-final")
	require.NotContains(t, string(init)+string(service), "cloud-init.target")
}

func TestStaleRefreshRunsTheExistingInitializerOnRestart(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "app.ini")
	initial := "[server]\nHTTP_ADDR = 0.0.0.0\nHTTP_PORT = 30000\nDOMAIN = soda\nSSH_DOMAIN = soda\nROOT_URL = http://soda:30000/\n[service]\nDISABLE_REGISTRATION = false\n"
	require.NoError(t, os.WriteFile(config, []byte(initial), 0600))
	writeWelcomeTestCommand(t, root, "hostnamectl", "echo soda\n")
	writeWelcomeTestCommand(t, root, "forgejo-tailnet", "echo 'soda.test.ts.net 100.64.0.1'\n")
	writeWelcomeTestCommand(t, root, "install", ":\n")
	writeWelcomeTestCommand(t, root, "chown", ":\n")
	writeWelcomeTestCommand(t, root, "runuser", "printf '%s\\n' \"$*\" >>\"$TEST_CALLS\"\necho 'Soda OS'\n")
	// Model the native restart boundary by executing the real initializer again.
	// Actual systemd transaction behavior is covered by installed acceptance.
	writeWelcomeTestCommand(t, root, "systemctl", "test \"$*\" = 'restart forgejo.service'\nprintf '%s\\n' restart >>\"$TEST_CALLS\"\nexec sh \"$TEST_INIT\"\n")
	script, err := os.ReadFile(filepath.Join("..", "..", "..", "packaging", "rpm", "forgejo", "sources", "forgejo-init"))
	require.NoError(t, err)
	body := strings.NewReplacer("/etc/forgejo/app.ini", config, "/usr/libexec/soda/forgejo-tailnet", filepath.Join(root, "forgejo-tailnet")).Replace(string(script))
	path := filepath.Join(root, "forgejo-init")
	require.NoError(t, os.WriteFile(path, []byte(body), 0700))
	command := exec.Command("sh", path, "refresh-tailnet")
	command.Env = append(os.Environ(), "PATH="+root+":"+os.Getenv("PATH"), "TEST_INIT="+path, "TEST_CALLS="+filepath.Join(root, "calls"))
	output, err := command.CombinedOutput()
	require.NoError(t, err, "%s", output)
	got, err := os.ReadFile(config)
	require.NoError(t, err)
	require.Equal(t, strings.ReplaceAll(initial, "soda", "soda.test.ts.net"), string(got))
	calls, err := os.ReadFile(filepath.Join(root, "calls"))
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(string(calls), "restart\n--user git -- forgejo migrate"))
	require.Contains(t, string(calls), "forgejo admin auth list")
}
