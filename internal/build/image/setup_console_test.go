package image

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetupProfileUsesOnlyAuthenticatedAdministratorConsoles(t *testing.T) {
	profile, err := os.ReadFile(filepath.Join("..", "..", "..", "packaging", "rpm", "runtime", "sources", "profile.d", "soda-console-welcome.sh"))
	require.NoError(t, err)
	for _, tc := range []struct {
		name, tty, uid, groups, ssh, pending, result string
		launch                                       bool
	}{
		{"local administrator", "/dev/tty1", "1000", "owner wheel", "", "0", "0", true},
		{"serial administrator", "/dev/ttyS0", "1000", "owner wheel", "", "0", "0", true},
		{"ARM serial console", "/dev/ttyAMA0", "1000", "owner wheel", "", "0", "0", true},
		{"virtual console", "/dev/hvc0", "1000", "owner wheel", "", "0", "0", true},
		{"ready still opens", "/dev/tty1", "1000", "owner wheel", "", "0", "0", true},
		{"dismissed", "/dev/tty1", "1000", "owner wheel", "", "1", "0", false},
		{"ordinary user", "/dev/tty1", "1001", "alice", "", "0", "0", false},
		{"workspace", "/dev/tty1", "1001", "wheel soda-workspaces", "", "0", "0", false},
		{"root", "/dev/tty1", "0", "root wheel", "", "0", "0", false},
		{"SSH", "/dev/tty1", "1000", "owner wheel", "client", "0", "0", false},
		{"PTY", "/dev/pts/0", "1000", "owner wheel", "", "0", "0", false},
		{"failure returns shell", "/dev/tty1", "1000", "owner wheel", "", "0", "1", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeWelcomeTestCommand(t, root, "welcome", ":\n")
			writeWelcomeTestCommand(t, root, "tty", "printf '%s\\n' \"$TEST_TTY\"\n")
			writeWelcomeTestCommand(t, root, "id", "case $1 in -u) echo \"$TEST_UID\";; -Gn) echo \"$TEST_GROUPS\";; *) exit 1;; esac\n")
			writeWelcomeTestCommand(t, root, "setup", "test \"$1\" = pending || exit 2\nexit \"$TEST_PENDING\"\n")
			writeWelcomeTestCommand(t, root, "sudo", "test \"$2\" = console || exit 2\necho elevated-console\nexit \"$TEST_RESULT\"\n")
			body := strings.NewReplacer("/usr/libexec/soda/soda-console-welcome", filepath.Join(root, "welcome"), "/usr/libexec/soda/soda-setup", filepath.Join(root, "setup")).Replace(string(profile))
			path := filepath.Join(root, "profile")
			require.NoError(t, os.WriteFile(path, []byte(body), 0600))
			command := exec.Command("bash", "--noprofile", "--norc", "-ic", `. "$1"; echo logged-in-shell`, "--", path)
			command.Env = append(os.Environ(), "PATH="+root+":"+os.Getenv("PATH"), "SSH_CONNECTION="+tc.ssh, "SSH_TTY=", "TEST_TTY="+tc.tty, "TEST_UID="+tc.uid, "TEST_GROUPS="+tc.groups, "TEST_PENDING="+tc.pending, "TEST_RESULT="+tc.result)
			output, err := command.Output()
			require.NoError(t, err)
			require.Equal(t, tc.launch, strings.Contains(string(output), "elevated-console"))
			require.True(t, strings.HasSuffix(string(output), "logged-in-shell\n"))
		})
	}
}
